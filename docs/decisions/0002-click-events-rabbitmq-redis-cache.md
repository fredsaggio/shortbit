# 0002 — Contagem de cliques via RabbitMQ e cache Redis de destinos quentes

Status: Accepted (implementação em andamento: conexão, publisher e worker prontos)
Data: 2026-10-24
Revisão: 2026-11-12 — fallback reaproveita o `event_id`; falhas do worker usam `basic.reject`; topologia e timeout do publish definidos.

## Contexto

Hoje cada redirect executa `UPDATE urls SET click_count = click_count + 1 ... RETURNING original_url` no PostgreSQL antes de responder. A resolução do destino e a contagem acontecem na mesma escrita síncrona. Com um link muito acessado, todas as requisições disputam o lock da mesma linha, e a resposta ao usuário espera a escrita.

A Fase 13 do plano prevê cache Redis para destinos quentes e RabbitMQ para eventos de clique, processados um a um, sem lotes. O plano exigia confirmar com o usuário a política de indisponibilidade do broker, a durabilidade, o tratamento de falhas recorrentes, a deduplicação, a execução do worker e a migração dos links privados. Este ADR registra essas escolhas.

## Opções consideradas

- Manter o `UPDATE` síncrono: simples e exato, mas mantém escrita e disputa de lock no caminho do usuário.
- Cache Redis antes da fila: um cache hit ainda precisaria do `UPDATE` no PostgreSQL para contar, então o banco não sai do caminho crítico.
- Agrupar cliques em lotes, Kafka, Redis Pub/Sub ou outbox por redirect: descartados pelo plano para esta etapa.
- RabbitMQ com worker individual e, depois, cache Redis: escolhido.

## Decisão

### Ordem de implementação

1. Conexão e configuração do RabbitMQ.
2. Publisher com confirmação do broker.
3. Worker com deduplicação persistente.
4. Troca do fluxo do redirect: resolução somente leitura e contagem pela fila.
5. Cache Redis de destinos quentes.

O cache só entra quando a contagem já não depende do PostgreSQL no redirect.

### Durabilidade

- Fila do tipo **quorum queue**. Ela é sempre durável e é o tipo replicado recomendado no RabbitMQ 4, que removeu o espelhamento de filas clássicas.
- Mensagens **persistentes**, publicadas com **publisher confirms** e flag `mandatory`. Sem `mandatory`, uma mensagem sem fila de destino é confirmada mesmo assim e se perde em silêncio. A API só responde `302` depois do confirm, que numa quorum queue significa que a maioria das réplicas aceitou a mensagem.
- Consumo com **ack manual**, enviado somente depois do commit no PostgreSQL.
- Falhas recorrentes: a fila declara `delivery-limit` de 20. Ao ultrapassá-lo, a mensagem vai para uma **dead-letter queue**. Dead-lettering at-least-once exige `dead-letter-strategy=at-least-once` e `overflow=reject-publish`.
- No RabbitMQ 4.3+, só `basic.reject` e a queda do consumidor contam para o `delivery-limit`. Um `basic.nack` com requeue **não conta**, e a mensagem ficaria em loop sem nunca ir para a dead-letter queue. Por isso o worker sinaliza falha com `Reject(requeue=true)`, nunca com `Nack`.

### Topologia e mensagem

| Item | Valor |
| --- | --- |
| Exchange | `click_events` (direct, durável), routing key `click` |
| Fila | `click_events.count` (quorum) |
| Dead-letter | exchange `click_events.dlx` (fanout) e fila `click_events.dead` (quorum) |
| Corpo | JSON com `event_id` (UUIDv7), `url_id` e `occurred_at` |
| Propriedades | `message_id` = `event_id`, persistente, `application/json` |

A API e o worker declaram a mesma topologia com os mesmos argumentos (`messaging.DeclareClickTopology`), então qualquer um pode subir primeiro. Mudar um argumento de uma fila existente exige apagá-la ou migrar, porque o RabbitMQ rejeita uma redeclaração diferente.

O publisher usa um único canal com confirmações assíncronas e reconecta em segundo plano com backoff exponencial (1s a 30s). Sem conexão, ou com o broker sinalizando `connection.blocked`, o publish falha na hora para não segurar o redirect.

### Indisponibilidade do broker

Se a publicação ou o confirm falharem dentro de um timeout curto (`RABBITMQ_PUBLISH_TIMEOUT`, padrão 500ms), a API faz **fallback para o `UPDATE` síncrono atual** no PostgreSQL. Se o PostgreSQL também falhar, responde `503`. Mantém-se a regra atual: o clique só é contado se for persistido antes da resposta. Redirecionar sem contar foi descartado porque perderia cliques em silêncio.

Por isso o RabbitMQ **não** entra no readiness da API, e a API sobe mesmo com o broker fora (com um log de aviso), como já acontece com o Redis.

### Execução do worker

O worker é um **binário separado** (`cmd/worker`), que escala e reinicia sem afetar a API. Ele reutiliza `config`, `db` e `repositories` e processa **um evento por transação**, sem lotes, com `prefetch` configurável (`RABBITMQ_PREFETCH`, padrão 10). O job de limpeza de cadastros continua dentro da API.

Quando o processamento falha (por exemplo, PostgreSQL fora), o worker espera antes de devolver o evento com `Reject(requeue=true)`. A espera começa em 1s e dobra a cada falha seguida, até 30s, voltando a 1s no primeiro sucesso. Sem ela, uma queda do banco gastaria as 20 tentativas em milissegundos. Com ela, o primeiro evento só vai para a dead-letter queue depois de uns 9 minutos de falhas seguidas. Eventos na dead-letter queue não se perdem e podem ser reenviados pelo painel.

- Evento malformado (JSON inválido, `event_id` nulo ou `url_id` não positivo) vai direto para a dead-letter queue com `Reject(requeue=false)`.
- Evento para URL inexistente é confirmado e logado, sem retry.
- No desligamento, o worker termina o evento em andamento e devolve os já recebidos com `Nack(requeue=true)`, que não conta como falha.

### Deduplicação

Usar o padrão idempotent consumer:

- A API gera um `event_id` (UUIDv7) por clique e o envia como `message_id`.
- Numa única transação, o worker executa `INSERT INTO processed_click_events ... ON CONFLICT DO NOTHING`. Se a linha foi inserida, faz `UPDATE urls SET click_count = click_count + 1`.
- Depois do commit, envia o ack. Numa reentrega o `INSERT` não insere nada e o worker só envia o ack.
- Um job do worker remove, a cada hora, eventos processados há mais de `PROCESSED_CLICK_EVENT_RETENTION` (padrão 7 dias). A retenção precisa ser maior que o tempo que um evento pode ficar parado na fila: se o worker ficar fora por mais que isso, um evento já contado pelo fallback síncrono pode ser contado de novo.
- Um `UPDATE` que afete zero linhas (link removido no futuro) é logado e confirmado.

Confirmar a mensagem depois do `UPDATE` não basta para impedir contagem dupla. A tabela de eventos processados é o que garante a idempotência.

Um timeout do confirm não prova que a mensagem se perdeu: o broker pode tê-la guardado e só demorado a responder. Por isso o **fallback síncrono também grava o `event_id`** em `processed_click_events`, na mesma transação do `UPDATE`. Se a mensagem atrasada chegar ao worker, ele a reconhece como já processada e não conta de novo.

### Links privados

A validação da `link_access_session` continua síncrona no PostgreSQL, por ser controle de acesso. Apenas a **contagem** passa para a fila, no mesmo caminho dos links públicos. O destino de link privado **nunca** vai para o Redis.

### Cache Redis

- Admissão pelo contador de frequência existente: 20 redirects efetivos em 60 segundos admitem o destino por até 5 minutos, conforme o plano.
- Somente links públicos, guardando apenas o dado de resolução. Senha, hash, token, dados de sessão e `click_count` nunca vão para o cache.
- Falha do Redis volta ao PostgreSQL.
- Hoje os links são imutáveis e não há exclusão, então não há invalidação a tratar. Quando edição ou exclusão existir, o fluxo correspondente deve apagar a chave.

## Consequências

- O redirect deixa de esperar a escrita no PostgreSQL. O contador passa a ter **consistência eventual** e pode ficar alguns instantes atrasado em relação aos acessos.
- O número de `UPDATE`s por clique não diminui: a escrita só muda de lugar.
- Surgem novos componentes para operar: broker, worker, tabela `processed_click_events` e a limpeza dela.
- O publish com confirm acrescenta alguns milissegundos ao redirect em troca de não perder cliques depois da resposta.
- Com o broker fora, o sistema volta ao comportamento atual. Com broker e PostgreSQL fora, o redirect retorna `503`.
- A URL do RabbitMQ contém credenciais: nunca deve ser logada nem incluída em mensagens de erro. Em produção, usar `amqps://`.

Referências: [RabbitMQ — Quorum queues](https://www.rabbitmq.com/docs/quorum-queues), [RabbitMQ — Consumer acknowledgements and publisher confirms](https://www.rabbitmq.com/docs/confirms), [RabbitMQ — Work queues em Go](https://www.rabbitmq.com/tutorials/tutorial-two-go).
