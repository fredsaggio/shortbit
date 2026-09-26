# Modelo de dados

> Status: migrations `00001` a `00006`
> Última atualização: 26 de setembro de 2026

PostgreSQL é a fonte da verdade. Tokens secretos são armazenados somente como hash.

## Diagrama

```mermaid
erDiagram
    users ||--o| password_credentials : "pode possuir"
    users ||--o{ auth_identities : "pode possuir"
    users ||--o{ user_sessions : "possui"
    users ||--o| password_reset_attempts : "pode possuir uma"
    users ||--o{ urls : "possui"
    urls ||--o{ link_access_sessions : "libera acesso com"

    users {
        uuid id PK
        text email UK
        timestamptz email_verified_at
        timestamptz created_at
        timestamptz updated_at
    }

    password_credentials {
        uuid user_id PK,FK
        text password_hash
        timestamptz created_at
        timestamptz updated_at
    }

    auth_identities {
        uuid user_id FK
        text provider PK
        text provider_user_id PK
        timestamptz created_at
    }

    user_sessions {
        bytea token_hash PK
        uuid user_id FK
        timestamptz created_at
        timestamptz expires_at
    }

    password_reset_attempts {
        bytea token_hash PK
        uuid user_id FK,UK
        bytea verification_proof_hash
        smallint failed_attempts
        timestamptz locked_until
        timestamptz last_code_sent_at
        timestamptz code_expires_at
        timestamptz attempt_expires_at
        timestamptz created_at
        timestamptz updated_at
        timestamptz used_at
    }

    urls {
        bigint id PK
        text short_code UK
        uuid user_id FK
        text original_url
        url_visibility visibility
        text password_hash
        bigint click_count
        timestamptz expires_at
        timestamptz created_at
        timestamptz updated_at
    }

    link_access_sessions {
        bytea token_hash PK
        bigint link_id FK
        timestamptz created_at
        timestamptz expires_at
    }
```

`password_registration_attempts` não aponta para `users`, pois representa um cadastro que ainda não virou conta.

## Ciclo de vida das contas

### Cadastro por senha

```mermaid
flowchart LR
    Attempt[password_registration_attempts] --> Confirm{Código confirmado?}
    Confirm -- Não --> Attempt
    Confirm -- Sim --> User[users]
    Confirm -- Sim --> Credential[password_credentials]
    User --> Credential
    Attempt -. limpeza assíncrona .-> Deleted[Tentativa removida]
```

### Primeiro login Google

```mermaid
flowchart TD
    Identity[Identidade validada] --> ExistingSub{auth_identity pelo sub?}
    ExistingSub -- Sim --> ExistingUser[users existente]
    ExistingSub -- Não --> PasswordEmail{Conta com senha e mesmo email?}
    PasswordEmail -- Sim --> Link[INSERT auth_identities no user existente]
    PasswordEmail -- Não --> New[INSERT users + auth_identities]
```

### Sessão

```mermaid
flowchart LR
    Login[Login aprovado] --> Token[Token puro no cookie]
    Token --> Hash[SHA-256]
    Hash --> Row[user_sessions]
    Row --> Expired{expires_at passou?}
    Expired -- Sim --> Reject[Rejeita autenticação]
```

## Tabelas

### `users`

Identidade local principal. Usa UUIDv7, email canônico e email obrigatoriamente confirmado.

### `password_credentials`

Existe somente para contas capazes de entrar por senha. O hash usa Argon2id. Separar credencial de `users` permite contas Google-only sem coluna de senha nula ou artificial.

### `auth_identities`

Guarda identidades externas. Para Google:

```text
provider = google
provider_user_id = claim sub
```

A chave `(provider, provider_user_id)` identifica a conta externa. A constraint `(user_id, provider)` impede vincular duas contas do mesmo provedor ao mesmo usuário.

### `password_registration_attempts`

Guarda temporariamente cadastro ainda não confirmado, incluindo password hash, prova HMAC do código, cooldowns e prazos. Não concede autenticação.

### `user_sessions`

Guarda o hash de tokens opacos. O token puro só fica no cookie do cliente.

### `password_reset_attempts` 🚧

Há no máximo uma linha por usuário com senha. O token opaco fica no cookie e seu hash identifica a tentativa; `verification_proof_hash` guarda HMAC(token, código), nunca o código puro. O upsert respeita cooldown e bloqueio, troca o código sem estender a vida da tentativa ativa e reinicia os contadores apenas após a expiração. `used_at` permitirá impedir reutilização após a confirmação.

### `urls` 📋

O schema está pronto, mas o fluxo HTTP ainda não foi implementado. Links privados exigem `password_hash`; links públicos exigem que esse campo seja `NULL`.

### `link_access_sessions` 📋

Sessão temporária e específica de um link privado. Não substitui `user_sessions` e não autentica uma conta.

## Regras de exclusão

| Relação | Comportamento |
|---|---|
| Usuário → credenciais/identidades/sessões/reset | `ON DELETE CASCADE` |
| Usuário → URLs | `ON DELETE RESTRICT` |
| URL → sessões de acesso | `ON DELETE CASCADE` |

Não existe endpoint de exclusão de conta no MVP; essas regras protegem consistência futura.
