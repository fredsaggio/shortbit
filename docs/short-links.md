# Links e shortcodes

> Status: schema e gerador implementados; endpoints ainda planejados
> Última atualização: 24 de setembro de 2026

## Objetivo do domínio

Cada URL curta pertence a um usuário autenticado e possui:

- shortcode público;
- URL original;
- visibilidade pública ou privada;
- senha opcional para links privados;
- expiração opcional;
- contador total de cliques;
- timestamps de criação e alteração.

## Estado atual

| Componente | Estado |
|---|---|
| Tabela `urls` | ✅ Implementada |
| Model `URL` | ✅ Implementado |
| Gerador Base62 aleatório | ✅ Implementado e testado |
| Repository de URLs | 📋 Planejado |
| Service e handlers | 📋 Planejados |
| Rotas HTTP | 📋 Planejadas |
| Redis seletivo | 📋 Planejado para depois da versão PostgreSQL |
| Estratégia final de shortcode | ⏸️ Decisão adiada |

## Modelo atual do shortcode

O código existente gera dez caracteres usando:

```text
0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz
```

O gerador usa `crypto/rand` e amostragem por rejeição para evitar viés na distribuição. A tabela mantém `UNIQUE(short_code)`, que será a garantia definitiva contra persistir duplicatas.

```mermaid
flowchart LR
    Random[crypto/rand] --> Reject[Descarta bytes que causariam viés]
    Reject --> Base62[Mapeia uniformemente para Base62]
    Base62 --> Code[Shortcode de 10 caracteres]
    Code --> Insert[INSERT com UNIQUE short_code]
```

Foi discutida a alternativa Sqids derivada do `id` incremental. A decisão está pausada. Nenhuma mudança deve ser feita até avaliar novamente enumeração, estabilidade da configuração e necessidade de persistir o shortcode.

## Criação de URL 📋

```mermaid
flowchart TD
    Request[POST /urls] --> Auth{Sessão válida?}
    Auth -- Não --> Unauthorized[401]
    Auth -- Sim --> Validate[Valida URL, visibilidade,<br/>senha e expiração]
    Validate --> Generate[Gera shortcode]
    Generate --> Insert{INSERT no PostgreSQL}
    Insert -- Código único --> Created[201 Created]
    Insert -- Colisão de short_code --> Retry[Gera outro código e tenta novamente]
    Retry --> Insert
```

O retry deverá ser limitado e acontecer somente para a constraint específica de `short_code`. Outros erros do banco não são colisões e devem ser propagados.

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

## Endpoints administrativos planejados

| Endpoint | Função |
|---|---|
| `POST /urls` | Criar link público ou privado |
| `GET /urls?limit=...&cursor=...` | Listar somente links do usuário |
| `GET /urls/{code}` | Consultar metadados e analytics com ownership |

A listagem usará paginação por cursor baseada em `(created_at, id)`, evitando paginação instável por offset.

## Redirect público 📋

```mermaid
flowchart TD
    Request[GET /code] --> Find{Shortcode existe?}
    Find -- Não --> NotFound[404]
    Find -- Sim --> Expired{Está expirado?}
    Expired -- Sim --> Gone[410]
    Expired -- Não --> Visibility{É público?}
    Visibility -- Não --> PasswordPage[Exibe página de senha]
    Visibility -- Sim --> Increment[Incrementa click_count atomicamente]
    Increment --> Redirect[302 para original_url]
```

O clique só será contado quando o sistema realmente realizar o redirect.

O incremento deve ser atômico no PostgreSQL:

```sql
UPDATE urls
SET click_count = click_count + 1
WHERE id = $1;
```

Não será usado o fluxo vulnerável `SELECT → incrementar em Go → UPDATE`.

## Link privado e desbloqueio 📋

```mermaid
sequenceDiagram
    participant Browser
    participant API
    participant DB as PostgreSQL

    Browser->>API: GET /{code}
    API-->>Browser: Página/formulário de senha
    Browser->>API: POST /{code}/unlock
    API->>DB: Busca e compara password_hash
    API->>DB: Cria link_access_session
    API-->>Browser: Cookie específico do link + redirect
    Browser->>API: GET /{code} + cookie de acesso
    API->>DB: Valida sessão do link e incrementa clique
    API-->>Browser: 302 para URL original
```

O cookie de acesso a link privado não autentica a conta e só libera o link ao qual foi associado.

## Expiração

`expires_at = NULL` representa link sem expiração. Quando preenchido, precisa ser posterior à criação. Um link expirado não redireciona nem incrementa `click_count`.

## Analytics

O MVP começa com `click_count` total. A consulta administrativa exige que `urls.user_id` seja o usuário autenticado.

Analytics por evento, localização, dispositivo ou série temporal ficam fora do MVP.

## Cache seletivo futuro

O PostgreSQL será implementado e medido antes do Redis.

```mermaid
flowchart TD
    Redirect[Redirect efetivo] --> Heat[Incrementa frequência efêmera]
    Heat --> Hot{Atingiu limite?}
    Hot -- Não --> Postgres[Continua no PostgreSQL]
    Hot -- Sim --> Cache[Admite projeção no Redis]
    Cache --> Resolve[Próximas resoluções podem evitar SELECT]
    Resolve --> Count[Cliques continuam no PostgreSQL]
```

Redis será uma otimização de leitura. Se estiver indisponível, a aplicação volta ao PostgreSQL.
