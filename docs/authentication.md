# Autenticação

> Status: cadastro e login concluídos; recuperação de senha em desenvolvimento
> Última atualização: 24 de setembro de 2026

## Visão geral

A aplicação aceita duas formas de login, mas ambas terminam na mesma sessão local:

```mermaid
flowchart LR
    Password[Email e senha] --> Session[Sessão opaca local]
    Google[Google OIDC] --> Session
    Session --> Cookie[Cookie user_session]
    Cookie --> Auth[Middleware de autenticação]
    Auth --> Protected[Endpoints protegidos]
```

A aplicação não emite JWT próprio. O ID token do Google é usado somente para comprovar a identidade durante o callback. Depois disso, Google e senha usam `user_sessions` no PostgreSQL.

## Modelo de conta

```mermaid
flowchart TD
    User[users] --> Password{Possui password_credentials?}
    User --> Identity{Possui auth_identities Google?}
    Password -- Sim --> PasswordLogin[Pode entrar com senha]
    Identity -- Sim --> GoogleLogin[Pode entrar com Google]
```

Um usuário pode ser:

| Tipo de conta | `password_credentials` | `auth_identities` |
|---|---:|---:|
| Somente senha | Sim | Não |
| Somente Google | Não | Sim |
| Senha vinculada ao Google | Sim | Sim |

Uma conta criada originalmente pelo Google não pode adicionar ou recuperar senha no MVP. Uma conta confirmada por senha pode ser vinculada automaticamente ao Google quando o Google comprova o mesmo email.

## Cadastro com senha ✅

O cadastro não cria imediatamente uma linha em `users`. Primeiro existe uma tentativa temporária.

```mermaid
sequenceDiagram
    participant Client as Navegador/cliente
    participant API
    participant DB as PostgreSQL
    participant Email as Resend

    Client->>API: POST /registrations/password<br/>email + senha
    API->>API: Normaliza email e valida senha
    API->>API: Gera token, hash, código e prova HMAC
    API->>DB: Insere password_registration_attempts
    API->>Email: Envia código de confirmação
    API-->>Client: 202 + cookie password_registration

    Client->>API: POST /registrations/password/confirm<br/>código + cookie
    API->>DB: Busca tentativa pelo hash do token
    API->>API: Valida prazo, bloqueio e HMAC
    API->>DB: Cria users + password_credentials
    API-->>Client: 201 Created
```

### Por que existe um token e um código?

- O token identifica a tentativa específica e fica em cookie `HttpOnly`; somente seu SHA-256 é armazenado.
- O código chega por email e prova que a pessoa controla aquele endereço.
- A prova armazenada é `HMAC(token, código)`. O código puro não fica no banco.

### Prazos e proteção contra abuso

- código: 10 minutos por padrão;
- tentativa inteira: 30 minutos por padrão;
- reenvio: cooldown de 30 segundos;
- erro de código: espera de 30 segundos;
- a cada cinco erros: bloqueio de cinco minutos;
- reenvio gera outro código sem prolongar o prazo total da tentativa;
- job periódico remove tentativas expiradas ou cujo email já virou usuário.

## Login com senha ✅

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB as PostgreSQL

    Client->>API: POST /sessions<br/>email + senha
    API->>DB: Busca user + password_credentials
    API->>API: Compara senha com Argon2id
    API->>API: Gera token opaco aleatório
    API->>DB: Salva SHA-256(token) em user_sessions
    API-->>Client: 204 + cookie user_session
```

Email inexistente e senha incorreta retornam a mesma mensagem pública. Isso reduz enumeração de contas.

## Login com Google ✅

O fluxo usa OAuth 2.0 Authorization Code Flow com OpenID Connect, PKCE, `state` e `nonce`.

```mermaid
sequenceDiagram
    participant Browser as Navegador
    participant API
    participant Google
    participant DB as PostgreSQL

    Browser->>API: GET /auth/google
    API->>API: Gera state, nonce e code_verifier
    API-->>Browser: Cookies temporários + 302
    Browser->>Google: Login e consentimento
    Google-->>Browser: authorization code + state
    Browser->>API: GET /auth/google/callback
    API->>API: Confere state
    API->>Google: Troca code + code_verifier
    Google-->>API: ID token
    API->>API: Valida assinatura, issuer, audience,<br/>expiração, nonce e email_verified
    API->>DB: Encontra, cria ou vincula usuário pelo sub
    API->>DB: Cria user_session local
    API-->>Browser: 204 + cookie user_session
```

### Regra de localização e vinculação

```mermaid
flowchart TD
    Start[Identidade Google validada] --> FindSub{provider=google + sub existe?}
    FindSub -- Sim --> Existing[Usa o user_id já vinculado]
    FindSub -- Não --> PasswordEmail{Existe conta confirmada<br/>com senha e mesmo email?}
    PasswordEmail -- Sim --> Link[Vincula auth_identity ao usuário]
    PasswordEmail -- Não --> Create[Cria users + auth_identity]
    Existing --> Session[Cria sessão local]
    Link --> Session
    Create --> Session
```

O `sub` é o identificador estável da conta dentro do Google. Depois que ele é vinculado, o login encontra o usuário pelo `sub`, não pelo email retornado em logins posteriores.

## Sessão opaca ✅

O cookie contém o token puro; o banco contém apenas o hash.

```mermaid
flowchart LR
    Raw[Token aleatório puro] --> Cookie[Cookie user_session]
    Raw --> SHA[SHA-256]
    SHA --> Database[user_sessions.token_hash]
```

Configuração atual do cookie:

| Atributo | Valor |
|---|---|
| Nome | `user_session` |
| `HttpOnly` | Sim |
| `Secure` | Configurável; `false` somente no HTTP local |
| `SameSite` | `Lax` |
| `Path` | `/` |

`HttpOnly` impede JavaScript da página de ler diretamente o token, mas não substitui proteção contra XSS. Um script malicioso ainda pode realizar requests em nome do usuário enquanto estiver executando na página.

## Autenticação de requests ✅

```mermaid
flowchart TD
    Request[Request para rota protegida] --> Read{Cookie user_session existe?}
    Read -- Não --> Unauthorized[401 Não autenticado]
    Read -- Sim --> Hash[Calcula SHA-256 do token]
    Hash --> Lookup{Sessão existe e não expirou?}
    Lookup -- Não --> Unauthorized
    Lookup -- Sim --> Context[Coloca user_id no context.Context]
    Context --> Handler[Executa handler protegido]
```

O handler protegido usa o `user_id` do contexto. Ele nunca aceita que o cliente escolha a identidade por query, header ou body.

## `GET /me` ✅

`GET /me` comprova que a sessão está válida e retorna a identidade local atualmente autenticada:

```json
{
  "id": "019...",
  "email": "usuario@example.com"
}
```

## Logout ✅

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB as PostgreSQL

    Client->>API: DELETE /sessions/current + cookie
    API->>DB: DELETE pelo hash do token
    API-->>Client: Remove cookie + 204
```

O logout é idempotente: sem cookie, token desconhecido ou chamada repetida continuam retornando `204`.

## Recuperação de senha 🚧

Já existem migration, model e repository para a tentativa com código de oito dígitos. Service, envio do email, confirmação e endpoints ainda não existem.

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB as PostgreSQL
    participant Email

    Client->>API: POST /password-resets<br/>email
    API->>DB: Salva hash do token e prova HMAC do código
    API->>Email: Envia código de 8 dígitos
    API-->>Client: 202 genérico + cookie HttpOnly da tentativa
    Client->>API: POST /password-resets/confirm<br/>cookie + código + nova senha
    API->>DB: Valida código e troca senha em transação
    API->>DB: Marca tentativa como usada e revoga sessões
    API-->>Client: 204
```

Email inexistente, conta Google-only e conta com senha terão a mesma resposta externa no pedido de recuperação. Erros reais de infraestrutura continuam sendo `500`.

## “Lembrar de mim” 📋

Será a mesma sessão opaca com duas políticas de duração:

```text
remember_me=false → TTL curto + cookie de sessão sem Expires/Max-Age
remember_me=true  → TTL longo + cookie persistente
```

Não haverá token em `localStorage`, JWT próprio ou expiração deslizante no MVP.

## Inventário de códigos e tokens

| Valor | Objetivo | Viaja puro onde? | Persistência |
|---|---|---|---|
| Código de confirmação | Provar controle do email | Email e body de confirmação | Somente prova HMAC |
| Token de cadastro | Identificar tentativa | Cookie `password_registration` | SHA-256 |
| OAuth `state` | Ligar início e callback | Cookie e query do callback | Não persistido |
| OIDC `nonce` | Ligar tentativa ao ID token | Cookie e claim assinado | Não persistido |
| PKCE `code_verifier` | Provar quem iniciou a troca | Cookie e request backend→Google | Não persistido |
| Authorization code | Autorizar uma troca por tokens | Query do callback | Não persistido |
| ID token | Comprovar identidade Google | Google→backend | Validado e descartado |
| Session token | Autenticar na ShortBit | Cookie `user_session` | SHA-256 |
| Password reset token | Autorizar troca de senha | Link e body de confirmação | SHA-256 |
