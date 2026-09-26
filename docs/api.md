# API HTTP

> Status: contratos implementados e planejados
> Última atualização: 24 de setembro de 2026

Base local:

```text
http://localhost:8080
```

## Convenções

- JSON desconhecido é rejeitado nos endpoints que recebem body.
- O body deve conter exatamente um objeto JSON.
- Erros atuais são enviados como `text/plain; charset=utf-8` em português.
- Autenticação usa o cookie `user_session`.
- `204 No Content` não possui body.
- Todos os requests passam por limite global de body, rate limit, recovery, access log e request ID.

## Resumo de rotas

| Método | Rota | Autenticação | Estado |
|---|---|---|---|
| `GET` | `/health/live` | Não | ✅ |
| `GET` | `/health/ready` | Não | ✅ |
| `POST` | `/registrations/password` | Não | ✅ |
| `POST` | `/registrations/password/confirm` | Cookie temporário | ✅ |
| `POST` | `/registrations/password/resend` | Cookie temporário | ✅ |
| `POST` | `/sessions` | Não | ✅ |
| `DELETE` | `/sessions/current` | Cookie opcional | ✅ |
| `GET` | `/me` | Sim | ✅ |
| `GET` | `/auth/google` | Não | ✅ |
| `GET` | `/auth/google/callback` | Cookies temporários | ✅ |
| `POST` | `/password-resets` | Não | ✅ |
| `POST` | `/password-resets/confirm` | Cookie da tentativa | 📋 |
| `POST` | `/urls` | Sim | 📋 |
| `GET` | `/urls` | Sim | 📋 |
| `GET` | `/urls/{code}` | Sim + ownership | 📋 |
| `GET` | `/{code}` | Não | 📋 |
| `POST` | `/{code}/unlock` | Não | 📋 |

## Health checks ✅

### `GET /health/live`

Informa que o processo HTTP está vivo.

```text
200 OK
```

### `GET /health/ready`

Executa `Ping` no PostgreSQL com timeout.

```text
200 OK                  banco disponível
503 Service Unavailable banco indisponível
```

## Iniciar cadastro com senha ✅

### `POST /registrations/password`

```json
{
  "email": "usuario@example.com",
  "password": "senha12345"
}
```

Sucesso:

```text
202 Accepted
Set-Cookie: password_registration=...
```

Respostas relevantes:

| Status | Motivo |
|---:|---|
| `400` | JSON, email ou senha inválidos |
| `409` | Email já cadastrado |
| `429` | Rate limit por IP ou email |
| `500` | Falha interna ou no envio do email |

## Confirmar cadastro ✅

### `POST /registrations/password/confirm`

Requer o cookie `password_registration`.

```json
{
  "code": "123456"
}
```

Sucesso:

```http
HTTP/1.1 201 Created
Content-Type: application/json
```

```json
{
  "id": "019...",
  "email": "usuario@example.com"
}
```

| Status | Motivo |
|---:|---|
| `400` | JSON inválido, código inválido ou expirado |
| `409` | Outro fluxo confirmou o mesmo email primeiro |
| `410` | Tentativa ausente ou expirada |
| `429` | Cooldown ou bloqueio |
| `500` | Erro interno |

## Reenviar código ✅

### `POST /registrations/password/resend`

Não recebe body. Requer o cookie `password_registration`.

```text
202 Accepted
```

Pode retornar `410`, `429` ou `500`.

## Login com senha ✅

### `POST /sessions`

```json
{
  "email": "usuario@example.com",
  "password": "senha12345"
}
```

Sucesso:

```text
204 No Content
Set-Cookie: user_session=...
```

| Status | Motivo |
|---:|---|
| `400` | JSON inválido |
| `401` | Email ou senha incorretos |
| `429` | Rate limit por IP ou email |
| `500` | Erro interno |

`remember_me` ainda não faz parte do body implementado.

## Usuário atual ✅

### `GET /me`

Requer `user_session` válida.

```json
{
  "id": "019...",
  "email": "usuario@example.com"
}
```

Retorna `401` quando o cookie não existe, é desconhecido ou está expirado.

## Logout ✅

### `DELETE /sessions/current`

Remove a sessão do banco quando o cookie existe e expira o cookie no navegador.

```text
204 No Content
```

A operação é idempotente e não exige middleware de autenticação.

## Google OIDC ✅

### `GET /auth/google`

Cria cookies temporários e responde com redirect para o Google:

```text
302 Found
```

### `GET /auth/google/callback`

O Google envia `code` e `state`. Em sucesso:

```text
204 No Content
Set-Cookie: user_session=...
```

Atualmente não há frontend para receber um redirect final, por isso o callback termina em uma página vazia.

## Recuperação de senha 🚧

### `POST /password-resets`

Request:

```json
{
  "email": "usuario@example.com"
}
```

Email inexistente, Google-only e conta com senha respondem igualmente; a resposta também define um cookie HttpOnly `password_reset`:

```text
202 Accepted
```

### `POST /password-resets/confirm`

Contrato planejado:

```json
{
  "code": "12345678",
  "password": "nova-senha123",
  "password_confirmation": "nova-senha123"
}
```

O token opaco identifica a tentativa e fica em cookie HttpOnly. O email contém apenas o código de oito dígitos. O início já está implementado; a confirmação ainda não.

Sucesso planejado:

```text
204 No Content
```

## Links 📋

Os contratos exatos serão fechados durante a implementação vertical do domínio. As rotas reservadas são:

```text
POST /urls
GET  /urls?limit=...&cursor=...
GET  /urls/{code}
GET  /{code}
POST /{code}/unlock
```
