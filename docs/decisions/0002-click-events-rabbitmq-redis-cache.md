# 0002 — Contagem de cliques via RabbitMQ e cache Redis de destinos quentes

Status: Accepted (implementação em andamento: conexão com o RabbitMQ)
Data: 2026-10-24

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
- Falhas recorrentes: a quorum queue tem `delivery-limit` padrão de 20 no RabbitMQ 4. Ao ultrapassá-lo, a mensagem vai para uma **dead-letter queue**. Dead-lettering at-least-once exige `dead-letter-strategy=at-least-once` e `overflow=reject-publish`; os detalhes ficam para a implementação do publisher e do worker.

### Indisponibilidade do broker

Se a publicação ou o confirm falharem dentro de um timeout curto, a API faz **fallback para o `UPDATE` síncrono atual** no PostgreSQL. Se o PostgreSQL também falhar, responde `503`. Mantém-se a regra atual: o clique só é contado se for persistido antes da resposta. Redirecionar sem contar foi descartado porque perderia cliques em silêncio.

Por isso o RabbitMQ **não** entra no readiness da API, e a API sobe mesmo com o broker fora (com um log de aviso), como já acontece com o Redis.

### Execução do worker

O worker é um **binário separado** (`cmd/worker`), que escala e reinicia sem afetar a API. Ele reutiliza `config`, `db` e `repositories` e processa **um evento por transação**, sem lotes, com `prefetch` baixo e configurável. O job de limpeza de cadastros continua dentro da API.

### Deduplicação

Usar o padrão idempotent consumer:

- A API gera um `event_id` (UUIDv7) por clique e o envia como `message_id`.
- Numa única transação, o worker executa `INSERT INTO processed_click_events ... ON CONFLICT DO NOTHING`. Se a linha foi inserida, faz `UPDATE urls SET click_count = click_count + 1`.
- Depois do commit, envia o ack. Numa reentrega o `INSERT` não insere nada e o worker só envia o ack.
- Um job remove eventos processados antigos. A janela de retenção deve ser bem maior que o tempo máximo de reentrega.
- Um `UPDATE` que afete zero linhas (link removido no futuro) é logado e confirmado.

Confirmar a mensagem depois do `UPDATE` não basta para impedir contagem dupla. A tabela de eventos processados é o que garante a idempotência.

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
