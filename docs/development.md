# Desenvolvimento local

> Status: atual
> Última atualização: 24 de outubro de 2026

## Requisitos

- Go 1.27;
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

RABBITMQ_USER=shortbit
RABBITMQ_PASSWORD=...
RABBITMQ_URL=amqp://shortbit:...@localhost:5672/
```

`PUBLIC_BASE_URL` é opcional e assume `http://localhost:8080` no desenvolvimento; é usado para compor o `short_url` devolvido por `POST /urls`. Em produção, configure o domínio público, `COOKIE_SECURE=true` e callback Google com HTTPS.

`RABBITMQ_USER` e `RABBITMQ_PASSWORD` criam o usuário do container RabbitMQ no Compose; `RABBITMQ_URL` é a URL que a API usa e deve conter as mesmas credenciais. `RABBITMQ_URL` é obrigatória e aceita `amqp://` ou `amqps://`; em produção, use `amqps://`. Ela contém a senha e nunca é logada. Se o broker estiver indisponível na inicialização, a API registra um aviso e continua contando cliques de forma síncrona no PostgreSQL ([ADR 0002](decisions/0002-click-events-rabbitmq-redis-cache.md)). O painel de administração do RabbitMQ fica em `http://localhost:15672`.

## Subir o PostgreSQL

```bash
make infra/up
```

O Compose publica o PostgreSQL localmente na porta `5433`.

Para encerrar:

```bash
make infra/down
```

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
OpenAPI bruto em `http://localhost:8080/docs/openapi.yaml`. Depois de editar
`docs/openapi.yaml`, gere novamente o HTML embarcado no binário:

```bash
npx --yes @redocly/cli@2.36.0 build-docs docs/openapi.yaml --output docs/redoc.html --title 'ShortBit API' --disableGoogleFont
```

O HTML já contém o conteúdo da especificação, mas usa o script do ReDoc via CDN
para a navegação dinâmica; o navegador precisa de internet para essa parte.

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

Depois do consentimento, o callback atualmente responde `204 No Content`. A página vazia é esperada enquanto não existe frontend. No mesmo navegador, acesse:

```text
http://localhost:8080/me
```

Uma resposta `200` confirma que o cookie e a sessão local foram criados.

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
