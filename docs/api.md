# API HTTP

> Status: contratos implementados e planejados
> Última atualização: 26 de setembro de 2026

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
| `POST` | `/password-resets/confirm` | Cookie da tentativa | ✅ |
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
  "password": "senha12345",
  "remember_me": false
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

`remember_me` é opcional e assume `false` quando omitido. Sem ele, a sessão expira no servidor após 12h por padrão e o cookie não tem `Expires`/`Max-Age`. Com `true`, a sessão e o cookie persistente duram 30 dias por padrão. Ambos os prazos são configuráveis e absolutos; atividade não os renova.

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

Não recebe `remember_me`. O login Google cria uma sessão lembrada por padrão.

### `GET /auth/google/callback`

O Google envia `code` e `state`. Em sucesso:

```text
204 No Content
Set-Cookie: user_session=...
```

O cookie `user_session` é persistente e acompanha o TTL lembrado, de 30 dias por padrão.

Atualmente não há frontend para receber um redirect final, por isso o callback termina em uma página vazia.

## Recuperação de senha ✅

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

Request:

```json
{
  "code": "12345678",
  "password": "nova-senha123",
  "password_confirmation": "nova-senha123"
}
```

O token opaco identifica a tentativa e fica em cookie HttpOnly. O email contém apenas o código de oito dígitos. A confirmação consome a tentativa, troca a senha e revoga as sessões anteriores; não cria uma nova sessão.

Sucesso:

```text
204 No Content
```

## Links

### `POST /urls` ✅

Exige sessão autenticada. Recebe `url`, `visibility` (`public` ou `private`) e, para link privado, `password`. Retorna `201 Created` com `short_code` e `short_url`. Senha e hash nunca entram na resposta.

```json
{
  "short_code": "Ab3dX9",
  "short_url": "https://sho.rt/Ab3dX9"
}
```

### `GET /urls` ✅

Exige sessão autenticada. Aceita `limit` opcional (padrão 20; intervalo permitido de 1 a 20) e `cursor` opcional. Sem cursor, retorna a primeira página. Para a próxima página, o cliente copia `next_cursor` da resposta anterior para `?cursor=...`; não precisa interpretá-lo. O cursor é Base64URL sem padding de `created_at` e `id`, não um segredo ou autorização. Cursor ou limite inválido retorna `400`; sem sessão, `401`.

```json
{
  "urls": [
    {
      "short_code": "Ab3dX9",
      "original_url": "https://example.com",
      "visibility": "public",
      "click_count": 0,
      "created_at": "2026-09-27T12:00:00Z",
      "updated_at": "2026-09-27T12:00:00Z"
    }
  ],
  "next_cursor": "<cursor-da-proxima-pagina>"
}
```

`next_cursor` é `null` quando não há outra página. `urls` é `[]` quando não há links. A listagem nunca devolve senha nem hash de link privado.

### Próximas rotas 📋

```text
GET  /urls/{code}
GET  /{code}
POST /{code}/unlock
```
