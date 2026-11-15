# Desenvolvimento local

> Status: atual
> Última atualização: 24 de outubro de 2026

## Requisitos

- Go 1.27.4 (com `GOTOOLCHAIN=auto`, o padrão, um Go mais antigo baixa essa versão sozinho);
- Docker com Docker Compose;
- Goose;
- conta Resend configurada;
- cliente OAuth Web no Google Cloud.

## Variáveis de ambiente

O projeto carrega as variáveis externamente. Nunca faça commit de valores reais.

```env
POSTGRES_USER=...
POSTGRES_PASSWORD=...
POSTGRES_DB=...
DATABASE_URL=postgres://...@localhost:5433/...?sslmode=disable

HTTP_ADDR=:8080
USER_SESSION_TTL=12h
REMEMBERED_USER_SESSION_TTL=720h
COOKIE_SECURE=false

PASSWORD_REGISTRATION_CODE_TTL=10m
PASSWORD_REGISTRATION_ATTEMPT_TTL=30m
PASSWORD_RESET_CODE_TTL=10m
PASSWORD_RESET_ATTEMPT_TTL=30m

EMAIL_FROM=noreply@example.com
RESEND_API_KEY=...

GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

PUBLIC_BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:5173

RABBITMQ_USER=shortbit
RABBITMQ_PASSWORD=...
HOT_URL_THRESHOLD=20
HOT_URL_WINDOW=60s
URL_CACHE_TTL=5m

RABBITMQ_URL=amqp://shortbit:...@localhost:5672/
RABBITMQ_PUBLISH_TIMEOUT=500ms

# Worker (cmd/worker, ou a API com RUN_CLICK_WORKER=true)
RABBITMQ_PREFETCH=10
PROCESSED_CLICK_EVENT_RETENTION=168h

# Opcionais, usadas em produção (veja deployment.md)
RUN_MIGRATIONS=false
RUN_CLICK_WORKER=false
FRONTEND_DIST_DIR=
REDIS_URL=
CLIENT_IP_HEADER=

# Opcionais: métricas e logs por OTLP (veja observability.md). Sem o endpoint,
# nada sai do processo.
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_HEADERS=
```

As variáveis do último bloco são opcionais e ficam desligadas no desenvolvimento ([deploy](deployment.md), [ADR 0003](decisions/0003-free-single-service-deploy.md)):

- `RUN_MIGRATIONS=true` aplica as migrations pendentes ao subir a API, com trava consultiva do Postgres. Precisa de conexão direta com o banco, sem pooler.
- `RUN_CLICK_WORKER=true` roda o worker de cliques dentro da API, para hospedagens sem serviço de worker. Usa `RABBITMQ_PREFETCH` e `PROCESSED_CLICK_EVENT_RETENTION`. Não rode junto com o `make worker` sem necessidade: os dois consumiriam a mesma fila, o que funciona, mas não ajuda.
- `FRONTEND_DIST_DIR` aponta para o build do frontend (`frontend/dist`). A API passa a servir o app nas rotas dele, com CSP. Localmente, o Vite continua sendo o caminho.
- `REDIS_URL` (`redis://` ou `rediss://`, com senha) substitui `REDIS_ADDR` em Redis hospedado.
- `OTEL_EXPORTER_OTLP_ENDPOINT` liga o envio de métricas e logs por OpenTelemetry (OTLP/HTTP), para o Grafana local do Compose (`http://localhost:4318`) ou para o Grafana Cloud. `OTEL_EXPORTER_OTLP_HEADERS` leva o token (`Authorization=Basic ...`) e é segredo. Vazio, a API e o worker funcionam igual e não mandam nada. Os exportadores também entendem as outras variáveis `OTEL_*` padrão, como `OTEL_METRIC_EXPORT_INTERVAL`. Detalhes em [observability.md](observability.md).
- `CLIENT_IP_HEADER` faz o rate limit ler o IP do cliente de um cabeçalho que o proxy sobrescreve (`True-Client-IP` no Render). Sem proxy, deixe vazio: o cliente poderia mandar o cabeçalho e escolher o próprio limite. `X-Forwarded-For` é recusado.

`FRONTEND_URL` é opcional e assume `http://localhost:5173`, onde o Vite roda o frontend. É o destino do redirect do login com Google e precisa ser só a origem (esquema, host e porta), sem caminho nem query. Em produção, use a origem pública do frontend.

`PUBLIC_BASE_URL` é opcional e assume `http://localhost:8080` no desenvolvimento; é usado para compor o `short_url` devolvido por `POST /urls`. Em produção, configure o domínio público, `COOKIE_SECURE=true` e callback Google com HTTPS. Com `COOKIE_SECURE=true`, a API também envia `Strict-Transport-Security: max-age=31536000; includeSubDomains` em todas as respostas, porque cookies seguros indicam que ela está atrás de HTTPS; no desenvolvimento, com `COOKIE_SECURE=false`, o cabeçalho não é enviado.

`HOT_URL_THRESHOLD`, `HOT_URL_WINDOW` e `URL_CACHE_TTL` são opcionais: um link público com 20 redirects em 60 segundos tem o destino guardado no Redis por 5 minutos. Sem Redis, a API lê o PostgreSQL normalmente.

`RABBITMQ_USER` e `RABBITMQ_PASSWORD` criam o usuário do container RabbitMQ no Compose; `RABBITMQ_URL` é a URL que a API usa e deve conter as mesmas credenciais. `RABBITMQ_URL` é obrigatória e aceita `amqp://` ou `amqps://`; em produção, use `amqps://`. Ela contém a senha e nunca é logada. `RABBITMQ_PUBLISH_TIMEOUT` é opcional (padrão `500ms`) e limita a espera pela confirmação do broker. Se o broker estiver indisponível, a API registra um aviso, continua contando cliques de forma síncrona no PostgreSQL e tenta reconectar em segundo plano ([ADR 0002](decisions/0002-click-events-rabbitmq-redis-cache.md)). O painel de administração do RabbitMQ fica em `http://localhost:15672`.

## Subir o PostgreSQL

```bash
make infra/up
```

O Compose publica o PostgreSQL localmente na porta `5433`.

Para encerrar:

```bash
make infra/down
```

## Executar o worker de cliques

```bash
make worker
```

O worker consome a fila `click_events.count` e incrementa `urls.click_count` uma vez por evento. Ele lê só `DATABASE_URL`, `RABBITMQ_URL`, `RABBITMQ_PREFETCH` (opcional, padrão `10`) e `PROCESSED_CLICK_EVENT_RETENTION` (opcional, padrão `168h`), sem exigir as variáveis de Google, email e sessão da API. A migration `00007` precisa estar aplicada. Sem o worker rodando, os cliques ficam na fila e `click_count` não muda até ele voltar.

## Migrations

```bash
make migration/status
make migration/up
make migration/down
```

Criar migration:

```bash
make migration/new name=descricao_da_migration
```

As migrations também são embarcadas no binário de testes pelo arquivo `migrations/embed.go`.

## Executar a API

```bash
make run
```

Servidor padrão:

```text
http://localhost:8080
```

A documentação HTTP renderizada fica em `http://localhost:8080/docs`, e o
OpenAPI bruto em `http://localhost:8080/docs/openapi.yaml`. O `docs/redoc.html`
embarcado carrega `/docs/openapi.yaml` no navegador, então editar o YAML basta;
não há HTML para gerar de novo. O script do ReDoc vem de CDN, e o navegador
precisa de internet para renderizar a página.

## Tudo em contêineres

```bash
make up
```

Sobe PostgreSQL, Redis, RabbitMQ, a API e o worker de cliques, com a mesma imagem de produção (o `Dockerfile` da raiz). A API aplica as migrations ao subir e serve o front em `http://localhost:8080`; o worker roda num contêiner separado (ADR 0002). `make infra/down` para tudo.

- A API e o worker usam a rede do host, então os endereços `localhost` do `.env` (`DATABASE_URL`, `RABBITMQ_URL`) valem sem mudança. Só as variáveis listadas no `docker-compose.yml` entram nos contêineres.
- O Redis é sempre o do Compose, mesmo que o `.env` tenha um `REDIS_URL` apontando para outro.
- `FRONTEND_URL` vira `http://localhost:8080`, porque o front vem da própria API. Para o login com Google, `GOOGLE_REDIRECT_URL` continua `http://localhost:8080/auth/google/callback`.
- Ocupa a porta 8080: não rode `make run` ao mesmo tempo.
- Depois de mudar o código, `make up` de novo reconstrói a imagem.

## Testes

Unitários:

```bash
make test
```

Integração:

```bash
make test/integration
```

Os testes de integração usam Testcontainers e criam PostgreSQL descartável. Docker precisa estar disponível.

## Teste manual do Google

No Google Cloud, o cliente deve ser do tipo Web application e possuir exatamente este redirect local:

```text
http://localhost:8080/auth/google/callback
```

Com a API ativa, abra:

```text
http://localhost:8080/auth/google
```

Depois do consentimento, o callback redireciona para `FRONTEND_URL/painel` (padrão `http://localhost:5173/painel`). Com o frontend rodando (`make front/dev`), o painel abre já logado. Sem ele, o navegador mostra erro de conexão, mas a sessão foi criada; confira em:

```text
http://localhost:8080/me
```

## Logs

A API e o worker escrevem uma linha JSON por evento (`log/slog`), criadas por `logging.NewJSONLogger`. As regras:

- **Nunca logar dado pessoal nem segredo:** email, nome, senha, hash, token, cookie, código de verificação, URL de destino de link ou IP. Para referenciar algo, use IDs internos (`url_id`, `event_id`) ou o código curto.
- O log de acesso grava método, rota (`/urls/{code}`), caminho (sem query string), status, duração e tamanho da resposta.
- Eventos de segurança usam `telemetry.SecurityEvent`: uma linha `security event` com um campo `event` fixo (`login_failed`, `rate_limited`, `password_changed`, `account_deleted`...) e um contador de mesmo nome. Os atributos são palavras fixas e IDs internos, como `user_id` ou o nome do limitador; nunca o email ou o IP que foi limitado.
- Texto que vem de fora (o parâmetro `error` do Google, por exemplo) é cortado antes de ir para o log.
- **Rede de proteção:** o logger apaga emails (`[email]`) e credenciais dentro de URLs (`scheme://[credentials]@host`) de qualquer texto ou erro antes de gravar. Isso cobre mensagens que o código não controla, como erros de bibliotecas e valores de `panic`, mas não é desculpa para logar dado pessoal.

## Serviços externos

### Resend

Envia o código de confirmação de cadastro. `EMAIL_FROM` precisa usar um remetente aceito pela conta Resend.

### Google

Fornece somente a identidade durante o login OIDC. Access token e refresh token não são persistidos.

## Diagnóstico rápido

| Sintoma | Verificação |
|---|---|
| `relation does not exist` | Execute `make migration/up` |
| Cookie não chega em HTTP local | Confirme `COOKIE_SECURE=false` |
| `redirect_uri_mismatch` | Compare callback do Google e `GOOGLE_REDIRECT_URL` literalmente |
| `/me` retorna `401` | Confirme cookie `user_session` e sessão não expirada |
| Email não chega | Verifique remetente, domínio e logs do Resend |
| Integração não inicia | Verifique Docker e acesso ao socket |
