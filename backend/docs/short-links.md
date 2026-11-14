# Links e shortcodes

> Status: geração Sqids, criação, listagem, metadados individuais, redirects público/privado com contagem pelo RabbitMQ e cache Redis de destinos implementados
> Última atualização: 12 de novembro de 2026

## Objetivo do domínio

Cada URL curta pertence a um usuário autenticado e possui:

- shortcode público;
- URL original;
- visibilidade pública ou privada;
- senha opcional para links privados;
- contador total de cliques;
- timestamps de criação e alteração.

## Estado atual

| Componente | Estado |
|---|---|
| Tabela `urls` | ✅ Implementada, com mínimo de seis caracteres e sem expiração |
| Model `URL` | ✅ Implementado |
| Gerador Sqids com `MinLength: 6` | ✅ Implementado |
| Repository de URLs | ✅ Reserva ID, cria, consulta e lista por owner/cursor; resolve links públicos/privados e conta clique |
| Service e handlers | ✅ Criação, listagem, consulta individual, redirects e desbloqueio privado |
| Rotas HTTP | ✅ `POST /urls`, `GET /urls` e `GET /urls/{code}` autenticadas; `GET /{code}` e `POST /{code}/access` públicas |
| Redis seletivo | ✅ Cache de destinos públicos quentes, admitido por frequência |
| Estratégia final de shortcode | ✅ Decidida: ID incremental + Sqids |

## Decisão para o shortcode

O PostgreSQL fornece `urls.id` incremental e único. O gerador usa a [implementação oficial de Sqids em Go](https://github.com/sqids/sqids-go) para codificar **um único número**, o ID positivo da URL, com `MinLength: 6`. Assim, todo código tem **pelo menos seis caracteres**, mas poderá ter sete ou mais; não há máximo configurável nem um ponto de crescimento definido por `62^6`. Usamos o alfabeto e a blocklist padrão da versão fixada da biblioteca, sem alfabeto secreto ou chave. O código resultante é persistido em `urls.short_code`.

```mermaid
flowchart LR
    Sequence[Sequence do PostgreSQL] --> ID[ID único]
    ID --> Sqids[Sqids Encode de um ID com MinLength 6]
    Sqids --> Code[Shortcode alfanumérico de 6 ou mais caracteres]
    Code --> Insert[Persistir em urls.short_code UNIQUE]
```

Sqids garante códigos diferentes para entradas numéricas diferentes **sob a mesma configuração**. A biblioteca pode retornar erro de geração, por exemplo se uma blocklist excessivamente restritiva esgotar as tentativas; o service propaga esse erro. `MinLength` acrescenta caracteres de preenchimento definidos pelo algoritmo, não zeros à esquerda como na conversão direta do número. O tamanho poderá crescer antes de se ocupar toda a combinação matemática de seis caracteres; não usar `62^6` como limite ou capacidade prometida por Sqids.

Sqids **não é criptografia** nem impede enumeração por alguém determinado. Um alfabeto customizado também não seria uma chave segura. Link privado depende de senha; ações administrativas dependem de autenticação e ownership. Rate limit no redirect limita abuso online, mas não transforma o shortcode em segredo. [FAQ oficial](https://sqids.org/faq).

Guardar `short_code` em coluna própria, com `UNIQUE` e imutável no MVP. O redirect consulta o valor exato (`WHERE short_code = ...`), sem decodificar o código para buscar pelo ID. Isso preserva links antigos se a biblioteca mudar e rejeita variantes não armazenadas; o `Decode` de Sqids não é canônico por si só. `UNIQUE` permanece como defesa contra bugs e mudanças incompatíveis de configuração; colisão não é parte esperada do fluxo normal, e não haverá retry aleatório.

A versão da dependência está fixada, e alfabeto, `MinLength` e blocklist devem permanecer iguais entre instâncias. Uma atualização da biblioteca ou da blocklist pode alterar o código gerado para o mesmo ID; códigos antigos continuam resolvíveis por lookup exato, mas a geração de novos códigos pode conflitar com eles. Antes de atualizar a dependência/configuração, comparar os vetores de regressão já testados e planejar qualquer incompatibilidade. Não há chave criptográfica.

O ID interno continua `BIGINT` positivo; não há corte em seis caracteres. Deletar links não recupera IDs da sequence, e rollbacks podem deixar lacunas. Validar a conversão do ID SQL para o `uint64` exigido pela API Go de Sqids e tratar erros de codificação explicitamente.

`migrations/00003_create_urls.sql` aceita códigos com **mínimo seis, sem máximo fixo**, mantendo validação alfanumérica e `UNIQUE`. `internal/shortcodes/generator.go` codifica IDs com Sqids. Repository, service, handler e rota autenticada de criação já estão implementados.

## Criação de URL ✅

```mermaid
flowchart TD
    Request[POST /urls] --> Auth{Sessão válida?}
    Auth -- Não --> Unauthorized[401]
    Auth -- Sim --> Validate[Valida URL, visibilidade e senha]
    Validate --> Allocate[Reserva ID único no PostgreSQL]
    Allocate --> Encode[Sqids gera código com no mínimo 6 caracteres]
    Encode --> Insert[INSERT com ID e shortcode]
    Insert --> Created[201 Created]
```

Como `short_code` é `NOT NULL` e depende do ID, o repository obtém o próximo ID da sequence **antes** do `INSERT` com `nextval(pg_get_serial_sequence('urls', 'id'))` e insere o ID explícito usando `OVERRIDING SYSTEM VALUE`; a sintaxe foi validada em teste de integração. Lacunas na sequence após falhas são normais. O gerador valida ID positivo; erros inesperados de `UNIQUE` são propagados, sem retry aleatório.

## Ownership

Autenticação responde “quem é o usuário”. Autorização responde “este usuário pode operar este link?”.

```mermaid
flowchart LR
    Cookie[user_session] --> Auth[Middleware obtém user_id]
    Auth --> Query[Query exige urls.user_id = user_id]
    Query --> Result{Link pertence ao usuário?}
    Result -- Sim --> Allowed[Operação permitida]
    Result -- Não --> Denied[Não revelar dados do link]
```

Endpoints administrativos nunca confiarão em um `user_id` enviado pelo cliente.

## Endpoints administrativos

| Endpoint | Função |
|---|---|
| `POST /urls` | ✅ Criar link público ou privado |
| `GET /urls?limit=...&cursor=...` | ✅ Listar somente links do usuário |
| `GET /urls/{code}` | ✅ Consultar metadados e contador de cliques com ownership |

A listagem usa paginação keyset por `(created_at DESC, id DESC)`, evitando `OFFSET`. `limit` assume 20 quando omitido e aceita de 1 a 20. A consulta pede `limit+1` para detectar a próxima página; o item extra não é enviado. `next_cursor` é uma string Base64URL sem padding que codifica `created_at` e `id` do último link entregue, ou `null` no fim. É apenas posição, não autenticação nem criptografia. A query sempre filtra `user_id` da sessão; a resposta não contém senha nem hash. O contrato JSON está em [API HTTP](api.md).

A consulta individual usa `short_code` e `user_id` na mesma query. Assim,
um código inexistente e um link de outro usuário resultam no mesmo `404`.
Ela retorna os metadados e o contador de cliques ao dono, sem hash da senha.

## Redirect público ✅

```mermaid
flowchart TD
    Request[GET /code] --> Find{Shortcode existe?}
    Find -- Não --> NotFound[404]
    Find -- Sim --> Visibility{É público?}
    Visibility -- Não --> PrivateFlow[Ver fluxo privado abaixo]
    Visibility -- Sim --> Publish[Publica evento de clique no RabbitMQ]
    Publish --> Confirmed{Broker confirmou?}
    Confirmed -- Sim --> Redirect[302 para original_url]
    Confirmed -- Não --> Fallback[Incrementa no PostgreSQL com o mesmo event_id]
    Fallback -- Ok --> Redirect
    Fallback -- Falhou --> Unavailable[503]
    Publish -.-> Worker[Worker incrementa click_count depois]
```

`GET /{code}` não exige login. Código inexistente retorna `404`; link privado sem sessão válida mostra a página de senha descrita abaixo. O clique só é contado quando o link é resolvido, e o redirect só sai depois que o clique está registrado: confirmado pelo RabbitMQ ou gravado no PostgreSQL.

1. O repository busca `id` e `original_url` com um `SELECT`, sem escrever.
2. O redirect service cria um evento com `event_id` (UUIDv7) e o publica no RabbitMQ, esperando a confirmação por até `RABBITMQ_PUBLISH_TIMEOUT`.
3. Se a publicação falhar, o service chama o mesmo `CountClickOnce` do worker, que grava o `event_id` em `processed_click_events` e incrementa `click_count` numa transação. Se a publicação na verdade chegou à fila e só demorou, o worker encontra o `event_id` e não conta de novo.
4. Se o fallback também falhar, a API responde `503`, sem redirect.

O worker (`cmd/worker`) consome a fila e incrementa `click_count` um evento por vez. O contador tem consistência eventual. A resposta usa `302 Found` com `Location` apontando para a URL original e `Cache-Control: no-store`, para que o navegador não guarde o redirect e os acessos seguintes voltem à API. O rate limit global continua aplicado.

## Link privado e desbloqueio ✅

```mermaid
sequenceDiagram
    participant Browser
    participant API
    participant DB as PostgreSQL

    Browser->>API: GET /{code}
    API-->>Browser: Página/formulário de senha
    Browser->>API: POST /{code}/access (senha)
    API->>DB: Busca e compara password_hash
    API->>DB: Cria link_access_session
    API-->>Browser: Cookie específico do link + 303 para /{code}
    Browser->>API: GET /{code} + cookie de acesso
    API->>DB: Valida sessão do link
    API->>API: Registra o clique (RabbitMQ ou fallback)
    API-->>Browser: 302 para URL original
```

`GET /{code}` mostra HTML com formulário quando o link privado não tem uma sessão válida. O `POST` recebe `application/x-www-form-urlencoded` com o campo `password`. Senha incorreta reapresenta o formulário com `401`, sem criar sessão nem contar clique. Senha correta cria `link_access_session` no PostgreSQL, define o cookie `link_access_session` com `HttpOnly`, `SameSite=Lax`, `Path=/{code}` e `Secure` em HTTPS, e responde `303` para o mesmo shortcode.

O TTL padrão da sessão é **30 minutos**, configurável por `LINK_ACCESS_SESSION_TTL`; não há renovação automática. O cookie não autentica a conta. No GET seguinte, o servidor verifica se o hash do token pertence ao `link_id` e se a sessão não expirou; somente então registra o clique, do mesmo jeito que no link público, e responde `302`. Mesmo se um cliente anexar manualmente o cookie a outro link, ele não é aceito. O POST possui rate limit em memória por IP + shortcode, com burst de 5 e reposição de 20 tentativas por minuto, antes da comparação Argon2id. Teste de integração cobre senha errada, cookie, isolamento entre dois links, expiração e contador.

## Duração dos links

URLs criadas não expiram automaticamente. A sessão temporária usada para desbloquear um link privado continua tendo expiração própria, mas isso não remove nem desativa a URL.

## Analytics

O MVP começa com `click_count` total. A consulta administrativa exige que `urls.user_id` seja o usuário autenticado.

Analytics por evento, localização, dispositivo ou série temporal ficam fora do MVP.

## Cache seletivo e contagem pela fila ✅

O redirect público consulta o Redis antes do PostgreSQL e conta o clique pelo RabbitMQ:

```mermaid
flowchart TD
    Request[GET do link público] --> Hit{Destino no Redis?}
    Hit -- Sim --> Publish[Publica evento no RabbitMQ]
    Hit -- Não --> Postgres[Busca destino no PostgreSQL]
    Postgres --> Publish
    Publish --> Confirm[Broker confirma publicação]
    Confirm --> Response[API responde 302]
    Confirm --> Heat[Incrementa frequência efêmera]
    Heat --> Hot{Atingiu limite?}
    Hot -- Sim --> Cache[Admite projeção no Redis]
    Publish --> Worker[Worker consome um evento]
    Worker --> Count[PostgreSQL incrementa contador em 1]
    Count --> Ack[Confirma processamento na fila]
```

O Redis é só uma otimização de leitura. Se estiver indisponível, ou se uma entrada estiver corrompida, a aplicação lê o PostgreSQL e o redirect continua funcionando.

- **Admissão:** cada redirect público efetivo incrementa `url:heat:{code}`, que expira após `HOT_URL_WINDOW` (60s). Quando o contador chega a `HOT_URL_THRESHOLD` (20) num miss, o destino é gravado em `url:dest:{code}` com TTL `URL_CACHE_TTL` (5 minutos).
- **Conteúdo:** só `url_id` e `original_url`. Senha, hash, tokens, sessões e `click_count` nunca vão para o cache.
- **TTL:** um hit não renova a entrada. Depois de expirar, o próximo acesso lê o PostgreSQL e, se o link continuar quente, é admitido de novo. Assim o TTL é o atraso máximo entre o banco e o cache.
- **Links privados** nunca são admitidos: só a resolução pública grava no cache.
- **Invalidação:** links são imutáveis e não há exclusão no MVP. Quando edição ou exclusão existirem, o fluxo deve apagar `url:dest:{code}`.

O worker não diminui o número de escritas: um clique continua gerando um incremento individual no banco, mas o redirect não espera essa escrita. As estatísticas têm consistência eventual. Reentregas são deduplicadas por `processed_click_events`; confirmar a mensagem depois do UPDATE não bastaria para impedir contagem dupla.

As decisões de durabilidade, falha do broker, execução do worker, deduplicação e contagem dos links privados estão no [ADR 0002](decisions/0002-click-events-rabbitmq-redis-cache.md). Resumo: quorum queue com publisher confirms; fallback para o `UPDATE` síncrono se o broker falhar e `503` se o PostgreSQL também falhar; worker em binário separado com tabela de eventos processados; links privados contam pela fila, mas nunca entram no cache. Publisher, worker, redirect publicando eventos e cache de destinos estão implementados. Não adicionar lotes, Kafka, Tinybird ou outbox por redirect.
