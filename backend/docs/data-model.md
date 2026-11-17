# Modelo de dados

> Status: migrations `00001` a `00010`
> Última atualização: 16 de novembro de 2026

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
    urls ||--o{ url_daily_clicks : "conta por dia em"
    urls ||--o{ url_daily_countries : "conta por país em"
    urls ||--o{ url_daily_audience : "conta visitantes e robôs em"
    urls ||--o{ url_daily_visitor_hashes : "deduplica visitantes com"

    users {
        uuid id PK
        text email UK
        text full_name
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
        timestamptz created_at
        timestamptz updated_at
    }

    link_access_sessions {
        bytea token_hash PK
        bigint link_id FK
        timestamptz created_at
        timestamptz expires_at
    }

    processed_click_events {
        uuid event_id PK
        timestamptz processed_at
    }

    url_daily_clicks {
        bigint url_id PK,FK
        date day PK
        text source PK
        bigint clicks
    }

    url_daily_countries {
        bigint url_id PK,FK
        date day PK
        text country PK
        bigint clicks
    }

    url_daily_audience {
        bigint url_id PK,FK
        date day PK
        bigint unique_visitors
        bigint robot_visits
    }

    url_daily_visitor_hashes {
        bigint url_id PK,FK
        date day PK
        bytea visitor_hash PK
    }

    visitor_salts {
        date day PK
        bytea salt
    }
```

`password_registration_attempts` não aponta para `users`, pois representa um cadastro que ainda não virou conta. `processed_click_events` não aponta para `urls`: guarda só o `event_id` para deduplicação.

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

Identidade local principal. Usa UUIDv7, email canônico e email obrigatoriamente confirmado. `full_name` (migration `00008`) é opcional no banco porque contas antigas não têm nome; quando presente, não pode ser vazio e tem até 100 caracteres. O cadastro com senha sempre envia o nome, e o login com Google o preenche quando falta. `password_registration_attempts.full_name` guarda o nome até a confirmação.

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

### `password_reset_attempts`

Há no máximo uma linha por usuário com senha. O token opaco fica no cookie e seu hash identifica a tentativa; `verification_proof_hash` guarda HMAC(token, código), nunca o código puro. O upsert respeita cooldown e bloqueio, troca o código sem estender a vida da tentativa ativa e reinicia os contadores apenas após a expiração. `used_at` impede reutilização após a confirmação.

### `urls` ✅

O schema e os fluxos HTTP de criação, listagem, consulta individual e redirects público/privado estão implementados. Links privados exigem `password_hash`; links públicos exigem que esse campo seja `NULL`. URLs criadas não expiram automaticamente; a expiração de `link_access_sessions` limita apenas o acesso temporário a um link privado (30 minutos por padrão, configurável).

`id` é `BIGINT GENERATED ALWAYS AS IDENTITY`; `short_code` é derivado do ID com Sqids e `MinLength: 6`, persistido e protegido por `UNIQUE`. A migration exige **no mínimo seis caracteres, sem máximo fixo**. O redirect consulta `short_code` diretamente, sem decodificá-lo. `click_count` é incrementado pelo worker de cliques, ou pelo fallback síncrono, com deduplicação em `processed_click_events`. Consulte [Links e shortcodes](short-links.md) para configuração, limites reais e fluxo de criação.

### `link_access_sessions` 📋

Sessão temporária e específica de um link privado. Não substitui `user_sessions` e não autentica uma conta.

### `processed_click_events` ✅

Tabela de deduplicação do worker de cliques ([ADR 0002](decisions/0002-click-events-rabbitmq-redis-cache.md)). Na mesma transação, o worker insere o `event_id` com `ON CONFLICT DO NOTHING` e, só se a linha foi inserida, incrementa `urls.click_count`. Uma reentrega do mesmo evento não insere nada e não conta de novo. Um evento para uma URL inexistente desfaz a transação e não é registrado.

O worker apaga eventos com `processed_at` mais antigo que `PROCESSED_CLICK_EVENT_RETENTION` (7 dias por padrão), a cada hora; o índice em `processed_at` atende essa limpeza.

### `url_daily_clicks` ✅

Cliques de cada link por dia e por origem: `link` (alguém abriu o link curto) ou `qr` (alguém leu o QR code, que leva `?qr=1`). O dia é a data do clique em `America/Sao_Paulo`. Só existem linhas para dias com clique; a API completa os outros com zero. O worker e o fallback somam aqui na mesma transação de `processed_click_events` e `urls.click_count`, então reentregas não contam de novo ([ADR 0005](decisions/0005-daily-clicks-and-qr-source.md)). Apagar a URL apaga as linhas (`ON DELETE CASCADE`).

### `url_daily_countries` ✅

Cliques de pessoas por link, dia e país (código ISO de duas letras), vindo da base IP→país local ([ADR 0007](decisions/0007-country-unique-visitors-robots.md)). Cliques sem país conhecido não têm linha: são a diferença para `url_daily_clicks`, calculada pela API. Robôs não entram.

### `url_daily_audience` ✅

Por link e dia: `unique_visitors` (pessoas diferentes, pelo hash do dia) e `robot_visits` (robôs de prévia e buscadores, que ficam fora de `click_count`, `url_daily_clicks` e dos países). Os únicos são por dia: a soma de um período conta de novo quem voltou em outro dia.

### `url_daily_visitor_hashes` ✅

Visitantes já contados num link num dia: `HMAC-SHA256(sal do dia, url_id ‖ IP ‖ User-Agent)` truncado em 16 bytes, calculado pela API. O worker insere com `ON CONFLICT DO NOTHING` e só soma em `unique_visitors` se a linha entrou, na mesma transação de `processed_click_events`. O job horário do worker apaga os dias anteriores a ontem.

### `visitor_salts` ✅

Um sal aleatório de 32 bytes por dia de `America/Sao_Paulo`, criado pela primeira instância da API que precisa dele. O job horário do worker apaga os sais de dias passados; sem o sal, um hash guardado não pode ser refeito a partir de um IP.

## Regras de exclusão

| Relação | Comportamento |
|---|---|
| Usuário → credenciais/identidades/sessões/reset | `ON DELETE CASCADE` |
| Usuário → URLs | `ON DELETE RESTRICT` |
| URL → sessões de acesso | `ON DELETE CASCADE` |
| URL → cliques por dia | `ON DELETE CASCADE` |
| URL → países, visitantes e robôs por dia | `ON DELETE CASCADE` |

Não existe endpoint de exclusão de conta no MVP; essas regras protegem consistência futura.
