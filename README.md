# ShortBit

ShortBit is a URL shortener for public or password-protected links. It tracks clicks, provides analytics, and supports password or Google sign-in.

> Projeto em desenvolvimento, construído em Go e PostgreSQL para estudo de backend, segurança, concorrência e evolução de uma API até produção.

| Parte | O que faz |
| --- | --- |
| **Contas** | Cadastro com senha e confirmação por email, login com senha ou Google OIDC, recuperação de senha por código, `/me` e logout |
| **Sessões** | Sessões opacas em cookies, com `remember_me` opcional no login por senha e sessão lembrada por padrão no Google |
| **Links** | Criação de links públicos e privados com shortcodes Sqids, listagem com paginação por cursor (até 20 por página) e consulta individual com metadados e contador de cliques |
| **Redirect** | Redirect público e privado; o clique vai para o RabbitMQ e um worker incrementa o contador, com fallback síncrono no PostgreSQL; links privados exigem senha e criam uma sessão temporária por link |
| **Cache** | Links públicos com 20 acessos em 60 segundos têm o destino guardado no Redis por 5 minutos; sem Redis, a leitura volta ao PostgreSQL |
| **Proteções** | Rate limiting, limite de tamanho do corpo e limpeza de cadastros pendentes |

---

## Estado atual

Tudo da tabela acima está implementado e testado. A Fase 13 tirou a contagem de cliques e a leitura dos links quentes do caminho do redirect ([ADR 0002](backend/docs/decisions/0002-click-events-rabbitmq-redis-cache.md)):

| Passo | Estado |
| --- | --- |
| Conexão e configuração do RabbitMQ | ✅ |
| Publisher com confirmação do broker | ✅ |
| Worker (`backend/cmd/worker`) com deduplicação | ✅ |
| Redirect contando pela fila, com fallback síncrono | ✅ |
| Cache Redis dos destinos mais acessados | ✅ |

Analytics além do contador total ficam fora do MVP.

---

## Stack

| Camada | Tecnologias |
| --- | --- |
| Backend | Go 1.27.4, `net/http` da biblioteca padrão, pgx |
| Banco | PostgreSQL 18, migrations com Goose |
| Cache | Redis 8 (go-redis): destinos quentes e contador de frequência |
| Mensageria | RabbitMQ 4 (amqp091-go) |
| Autenticação | Argon2id para senhas, Google OIDC (go-oidc) |
| Email | Resend |
| Shortcodes | Sqids |
| Documentação da API | OpenAPI renderizado com ReDoc |
| Testes | `go test`, Testcontainers |
| Infra | Docker Compose |

---

## Estrutura

```text
.
├── Makefile            # Todos os comandos, rodados a partir da raiz
├── AGENTS.md           # Regras do projeto (segurança, privacidade, comunicação)
├── docker-compose.yml  # PostgreSQL, Redis e RabbitMQ
└── backend/            # API e worker em Go (módulo Go próprio)
    ├── go.mod
    ├── cmd/
    │   ├── api/            # Ponto de entrada da API
    │   └── worker/         # Worker que consome a fila de cliques
    ├── internal/
    │   ├── app/            # Composition root: monta services, repositories e handlers
    │   ├── server/         # Rotas, health checks e /docs
    │   ├── handlers/       # Handlers HTTP e templates HTML
    │   ├── middleware/     # Autenticação, rate limit, access log, limite de corpo
    │   ├── services/       # Regras de negócio
    │   ├── repositories/   # Acesso ao PostgreSQL
    │   ├── models/         # Tipos de domínio
    │   ├── config/         # Leitura e validação das variáveis de ambiente
    │   ├── db/             # Pool de conexões e utilitários de teste
    │   ├── cache/          # Redis: frequência dos links e cache de destinos
    │   ├── messaging/      # RabbitMQ
    │   ├── jobs/           # Tarefas em segundo plano
    │   ├── shortcodes/     # Gerador Sqids
    │   ├── argon2/         # Hash de senhas
    │   ├── googleoidc/     # Cliente do login Google
    │   ├── email/          # Envio pelo Resend
    │   ├── sessiontoken/   # Tokens opacos de sessão
    │   ├── verificationcode/ # Códigos de confirmação e de recuperação
    │   ├── ctxval/         # Valores no context da requisição
    │   └── integration/    # Testes de integração HTTP + PostgreSQL
    ├── migrations/         # Migrations Goose, embarcadas no binário de testes
    └── docs/               # Documentação técnica, OpenAPI e ADRs (embarcados em /docs)
```

---

## Como rodar

**Pré-requisitos:** Go 1.27.4 (o Go baixa essa versão sozinho com `GOTOOLCHAIN=auto`), Docker com o Docker Compose, Goose, uma conta Resend e um cliente OAuth Web no Google Cloud.

Configure as variáveis de ambiente conforme [Desenvolvimento local](backend/docs/development.md) e depois:

```bash
make infra/up                                # sobe o PostgreSQL
docker compose up -d --wait redis-shortener rabbitmq-shortener
make migration/up                            # aplica as migrations
make run                                     # sobe a API
make worker                                  # sobe o worker de cliques (outro terminal)
```

O `make infra/up` sobe só o PostgreSQL. A API funciona sem Redis e sem RabbitMQ: registra um aviso e continua sem cache e contando cliques de forma síncrona.

| Serviço | Endereço |
| --- | --- |
| API | http://localhost:8080 |
| Documentação da API (ReDoc) | http://localhost:8080/docs |
| OpenAPI bruto | http://localhost:8080/docs/openapi.yaml |
| Health checks | http://localhost:8080/health/live e `/health/ready` |
| PostgreSQL | `localhost:5433` |
| Redis | `127.0.0.1:6379` |
| RabbitMQ (painel) | http://localhost:15672 |

`make help` lista todos os comandos.

---

## Testes e qualidade

| Comando | O que faz |
| --- | --- |
| `make test` | Testes unitários |
| `make test/integration` | Testes de integração. Sobem PostgreSQL, Redis e RabbitMQ descartáveis com Testcontainers, então o Docker precisa estar ligado |
| `go vet ./...` | Análise estática |
| `go test -race ./...` | Detector de race conditions, para mudanças em concorrência, sessões, cache e rate limiting |
| `gofmt -w <arquivos>` | Formatação |

---

## Banco de dados

- As migrations ficam em `backend/migrations/`: uma para o trigger de `updated_at` e uma por tabela.
- `make migration/new name=descricao` cria uma migration nova; `make migration/up` aplica, `make migration/down` desfaz a última e `make migration/status` mostra o estado.
- `make migration/validate` confere as migrations sem aplicar.
- O modelo das tabelas está em [`backend/docs/data-model.md`](backend/docs/data-model.md).

---

## Documentação

O índice completo está em [`backend/docs/README.md`](backend/docs/README.md).

| Documento | Conteúdo |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | Regras do projeto |
| [`backend/docs/authentication.md`](backend/docs/authentication.md) | Cadastro, login, Google, cookies e sessões |
| [`backend/docs/short-links.md`](backend/docs/short-links.md) | Criação, resolução, links privados e o plano de cache e fila |
| [`backend/docs/api.md`](backend/docs/api.md) | Endpoints, o que recebem e o que devolvem |
| [`backend/docs/openapi.yaml`](backend/docs/openapi.yaml) | Contrato OpenAPI, servido em `/docs` |
| [`backend/docs/data-model.md`](backend/docs/data-model.md) | As tabelas e os relacionamentos |
| [`backend/docs/glossary.md`](backend/docs/glossary.md) | Códigos, tokens e termos do projeto |
| [`backend/docs/development.md`](backend/docs/development.md) | Variáveis de ambiente, execução e testes locais |
| [`backend/docs/decisions/`](backend/docs/decisions/README.md) | Decisões de arquitetura (ADRs) |
