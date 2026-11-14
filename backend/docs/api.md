# API HTTP

> Status: contratos implementados e planejados
> Última atualização: 14 de novembro de 2026

Base local:

```text
http://localhost:8080
```

Referência interativa de leitura: [`GET /docs`](http://localhost:8080/docs). A especificação
OpenAPI também está disponível em [`GET /docs/openapi.yaml`](http://localhost:8080/docs/openapi.yaml)
e em [`docs/openapi.yaml`](openapi.yaml). Esta página Markdown também registra rotas planejadas.

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
| `PATCH` | `/me` | Sim | ✅ |
| `POST` | `/me/password` | Sim | ✅ |
| `GET` | `/auth/google` | Não | ✅ |
| `GET` | `/auth/google/callback` | Cookies temporários | ✅ |
| `POST` | `/password-resets` | Não | ✅ |
| `POST` | `/password-resets/confirm` | Cookie da tentativa | ✅ |
| `POST` | `/urls` | Sim | ✅ |
| `GET` | `/urls` | Sim | ✅ |
| `GET` | `/urls/{code}` | Sim + ownership | ✅ |
| `GET` | `/{code}` | Não; cookie opcional para link privado | ✅ |
| `POST` | `/{code}/access` | Não | ✅ |

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

**Senha da conta** (cadastro, recuperação e troca): pelo menos 8 caracteres e no máximo 1024 bytes, com ao menos uma letra maiúscula (qualquer maiúscula Unicode, como `É`), um número e um caractere especial (qualquer coisa que não seja letra, número nem espaço). Cada regra que falha tem a sua mensagem em `400`: `senha muito curta`, `senha muito longa`, `a senha precisa ter pelo menos uma letra maiúscula`, `... um número` e `... um caractere especial`. Senhas criadas antes da regra continuam entrando. A senha de link privado segue só o mínimo de 8 caracteres ([ADR 0004](decisions/0004-account-password-composition.md)).

### `POST /registrations/password`

```json
{
  "email": "usuario@example.com",
  "password": "Senha12345!",
  "full_name": "Ana Souza"
}
```

`full_name` é obrigatório: de 1 a 100 caracteres depois de remover os espaços das pontas e juntar espaços repetidos. Caracteres de controle e invisíveis são rejeitados. O nome fica guardado na tentativa e vai para a conta na confirmação.

Sucesso:

```text
202 Accepted
Set-Cookie: password_registration=...
```

Respostas relevantes:

| Status | Motivo |
|---:|---|
| `400` | JSON, email, senha (veja as regras acima) ou nome completo inválidos |
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
  "password": "Senha12345!",
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

## Conta ✅

### `GET /me`

Requer `user_session` válida.

```json
{
  "id": "019...",
  "email": "usuario@example.com",
  "full_name": "Ana Souza",
  "has_password": true,
  "has_google": false,
  "created_at": "2026-11-14T12:00:00Z"
}
```

`full_name` é `null` em contas criadas antes do nome existir e ainda não preenchidas. `has_password` indica se a conta entra com senha (e pode trocá-la); `has_google`, se tem login com Google vinculado. Retorna `401` quando o cookie não existe, é desconhecido ou está expirado.

### `PATCH /me`

Altera o nome completo, com as mesmas regras do cadastro. Recebe `{"full_name": "Ana Souza"}` e devolve a conta atualizada, no formato do `GET /me`. Nome inválido retorna `400`.

### `POST /me/password`

```json
{
  "current_password": "Senha12345!",
  "new_password": "Nova-senha-1",
  "new_password_confirmation": "Nova-senha-1"
}
```

Segue o [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html): exige a senha atual. Em sucesso, numa única transação, troca a senha, **encerra todas as sessões da conta** e cria uma nova para este navegador (renovação do ID de sessão, conforme o [Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)). A nova sessão mantém o prazo da anterior, então um login "lembrado" continua lembrado.

```text
204 No Content
Set-Cookie: user_session=<novo token>
```

| Status | Motivo |
|---:|---|
| `400` | Senha atual incorreta, confirmação diferente, nova senha igual à atual ou fora das regras da senha da conta |
| `401` | Sem sessão válida |
| `409` | A conta só entra com Google e não tem senha |
| `429` | Mais de 5 tentativas seguidas; depois, 5 a cada 15 minutos, contadas por conta |

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

Não recebe `remember_me`. O login Google cria uma sessão lembrada por padrão. Pede os escopos `openid`, `email` e `profile`; o claim `name` preenche o nome completo da conta só se ela ainda não tiver um, para não sobrescrever um nome editado.

### `GET /auth/google/callback`

O Google envia `code` e `state`. O callback sempre termina com redirect para o frontend configurado em `FRONTEND_URL`:

```text
303 See Other
Location: http://localhost:5173/painel          sucesso
Set-Cookie: user_session=...

303 See Other
Location: http://localhost:5173/entrar?erro=google   qualquer falha
```

O cookie `user_session` é persistente e acompanha o TTL lembrado, de 30 dias por padrão. A falha não diz o motivo ao navegador (recusa, cookies ausentes, state diferente, erro do Google ou interno); o motivo fica no log. O destino vem só de `FRONTEND_URL`, nunca da requisição, para o callback não virar open redirect.

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
  "password": "Nova-senha123!",
  "password_confirmation": "Nova-senha123!"
}
```

O token opaco identifica a tentativa e fica em cookie HttpOnly. O email contém apenas o código de oito dígitos. A confirmação consome a tentativa, troca a senha e revoga as sessões anteriores; não cria uma nova sessão. A nova senha segue as regras da senha da conta, descritas em "Iniciar cadastro com senha".

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

### `GET /urls/{code}` ✅

Exige a sessão do dono do link. Retorna os metadados e o contador de cliques em
JSON, com os mesmos campos de um item de `GET /urls`. Não devolve a senha nem o
hash de um link privado. Código inexistente **e** link de outro usuário retornam
o mesmo `404`, sem revelar a existência do link. Sem sessão, retorna `401`.
Este endpoint não redireciona nem desbloqueia o link.

`click_count` tem consistência eventual: um clique publicado no RabbitMQ só
aparece depois que o worker o processa, e fica parado na fila enquanto o
worker estiver desligado.

```json
{
  "short_code": "Ab3dX9",
  "original_url": "https://example.com",
  "visibility": "private",
  "click_count": 7,
  "created_at": "2026-09-27T12:00:00Z",
  "updated_at": "2026-09-27T12:00:00Z"
}
```

### `GET /{code}` ✅

Não exige sessão de conta. Para um link público existente, registra o clique e retorna `302 Found`, com `Location` apontando para a URL original e `Cache-Control: no-store`. Para um link privado sem cookie de acesso válido, retorna `200 OK` com a página HTML de senha, sem contar clique. Com cookie válido para aquele link, registra o clique e retorna `302 Found`.

O clique é registrado como um evento no RabbitMQ, confirmado pelo broker antes do redirect, e o worker incrementa `click_count` depois. Se a publicação falhar, a API incrementa o contador no PostgreSQL antes de responder. Se nem o RabbitMQ nem o PostgreSQL registrarem o clique, retorna `503 Service Unavailable`, sem redirect. Código inexistente retorna `404`; outras falhas retornam `500`. O rate limit global pode retornar `429`.

### `POST /{code}/access` ✅

Recebe `application/x-www-form-urlencoded` com o campo `password`. Senha correta cria sessão específica para o link, define cookie `link_access_session` (`HttpOnly`, `SameSite=Lax`, `Path=/{code}`, `Secure` quando configurado) e retorna `303 See Other` para `/{code}`. Essa sessão dura **30 minutos por padrão** (`LINK_ACCESS_SESSION_TTL`), sem renovação automática.

Senha incorreta retorna `401` com o formulário HTML e não cria sessão; formulário inválido retorna `400`; código inexistente retorna `404`. Há rate limit por IP + shortcode antes da comparação da senha: burst de 5, reposição de 20 tentativas por minuto; excesso retorna `429` e `Retry-After`. O rate limit global também se aplica.
