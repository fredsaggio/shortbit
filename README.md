# ShortBit

ShortBit is a URL shortener for public or password-protected links. It tracks clicks, provides analytics, and supports password or Google sign-in.

> Projeto em desenvolvimento, construído em Go e PostgreSQL, com frontend em React, para estudo de backend, segurança, concorrência e evolução de uma API até produção.

| Parte | O que faz |
| --- | --- |
| **Contas** | Cadastro com nome completo, senha e confirmação por email; login com senha ou Google OIDC (o nome vem do Google); recuperação de senha por código; `/me`, edição do nome, troca de senha com renovação da sessão; logout |
| **Sessões** | Sessões opacas em cookies, com `remember_me` opcional no login por senha e sessão lembrada por padrão no Google |
| **Links** | Criação de links públicos e privados com shortcodes Sqids, listagem com paginação por cursor (até 20 por página) e consulta individual com metadados e contador de cliques |
| **Redirect** | Redirect público e privado; o clique vai para o RabbitMQ e um worker incrementa o contador, com fallback síncrono no PostgreSQL; links privados exigem senha e criam uma sessão temporária por link |
| **Cache** | Links públicos com 20 acessos em 60 segundos têm o destino guardado no Redis por 5 minutos; sem Redis, a leitura volta ao PostgreSQL |
| **Proteções** | Rate limiting, limite de tamanho do corpo e limpeza de cadastros pendentes |
| **Frontend** | Landing page, cadastro com nome, login (senha ou Google), recuperação de senha, painel para criar e listar links, detalhe com cliques e página Minha conta ([`frontend/`](frontend/README.md)) |

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

Tudo o que o projeto usa, por camada. Versões exatas em `backend/go.mod`, `frontend/package.json` e `docker-compose.yml`.

### Backend

| Tecnologia | Para quê |
| --- | --- |
| Go 1.27.4 (`net/http` da biblioteca padrão) | API e worker de cliques |
| pgx | Driver do PostgreSQL |
| Goose | Migrations, embarcadas no binário e aplicadas ao subir em produção |
| go-redis | Cache dos destinos mais acessados e contador de frequência |
| amqp091-go | Publicação e consumo da fila de cliques no RabbitMQ |
| Sqids | Gera os códigos curtos a partir do ID |
| Argon2id (`golang.org/x/crypto`) | Hash das senhas de conta e de link |
| go-oidc + `golang.org/x/oauth2` | Login com Google (OpenID Connect) |
| resend-go | Envio dos emails de código |
| Testcontainers | Testes de integração com PostgreSQL, Redis e RabbitMQ em containers |

### Frontend

| Tecnologia | Para quê |
| --- | --- |
| React 19, TypeScript 7, Vite 8 | Interface e build |
| React Router 8 | Rotas do app |
| Tailwind CSS v4 + shadcn/ui (Radix) | Estilo e componentes acessíveis |
| lucide-react | Ícones |
| sonner | Avisos (toasts) |
| zxcvbn-ts (dicionários comum e pt-BR) | Barra de força da senha dos links, num Web Worker |
| Plus Jakarta Sans e Sora (Fontsource) | Fonte do app e fonte do logo, servidas pelo próprio site |
| Vitest + Testing Library + jsdom | Testes |
| oxlint | Lint |

### Ambiente local

| Tecnologia | Para quê |
| --- | --- |
| Docker Compose | Sobe PostgreSQL 18, Redis 8, RabbitMQ 4 (com painel de administração), a API, o worker e um Grafana local (`grafana/otel-lgtm`) |
| Make | Todos os comandos do projeto, a partir da raiz |

### Produção

O passo a passo de cada serviço está em [`backend/docs/deployment.md`](backend/docs/deployment.md) e a decisão em [ADR 0003](backend/docs/decisions/0003-free-single-service-deploy.md). Todos no plano grátis.

| Serviço | Para quê |
| --- | --- |
| **Render** | Hospeda a aplicação: uma imagem Docker (`Dockerfile`) que serve a API, o frontend e o worker, configurada pelo `render.yaml`. Região Virginia |
| **Neon** | PostgreSQL gerenciado (us-east-1), conexão direta, sem pooler |
| **Upstash** | Redis gerenciado (us-east-1), com TLS (`rediss://`) |
| **CloudAMQP** | RabbitMQ gerenciado, plano Little Lemur (us-east-1), com TLS (`amqps://`) |
| **Resend** | Envia os emails de código de cadastro e de recuperação de senha, de `@shortbit.com.br` |
| **Google Cloud** (Google Auth Platform) | Cliente OAuth do login com Google em produção |
| **Registro.br** | Domínio `shortbit.com.br` e o DNS: registro A e CNAME do site, registros do Resend e MX/SPF do email |
| **ImprovMX** | Redireciona qualquer `@shortbit.com.br` (como `privacidade@`) para o Gmail do dono |
| **UptimeRobot** | Chama `/health/live` a cada 5 minutos para o Render não dormir, e avisa por email se o site cair |
| **Grafana Cloud** | Recebe métricas e logs por OpenTelemetry: painel com requisições, erros, redirects, fila de cliques e eventos de segurança, e alertas por email ([observability.md](backend/docs/observability.md)) |
| **Let's Encrypt / Google Trust Services** | Certificado HTTPS, emitido e renovado pelo Render |

### CI/CD

| Tecnologia | Para quê |
| --- | --- |
| GitHub Actions (`.github/workflows/ci.yml`) | A cada push e PR na `main`: gofmt, go vet, testes com `-race`, testes de integração, lint/testes/build do frontend e build da imagem Docker |
| Render Auto-Deploy (After CI Checks Pass) | Publica sozinho só depois que o CI passa |
| actionlint | Valida o workflow (rodado manualmente) |

--- | --- |
| Backend | Go 1.27.4, `net/http` da biblioteca padrão, pgx |
| Frontend | React 19, TypeScript, Vite, React Router, Vitest + Testing Library, oxlint |
| Banco | PostgreSQL 18, migrations com Goose |
| Cache | Redis 8 (go-redis): destinos quentes e contador de frequência |
| Mensageria | RabbitMQ 4 (amqp091-go) |
| Autenticação | Argon2id para senhas, Google OIDC (go-oidc) |
| Email | Resend |
| Shortcodes | Sqids |
| Documentação da API | OpenAPI renderizado com ReDoc |
| Observabilidade | OpenTelemetry (métricas e logs por OTLP), Grafana |
| Testes | `go test`, Testcontainers |
| Infra | Docker Compose |

---

## Estrutura

```text
.
├── Makefile            # Todos os comandos, rodados a partir da raiz
├── AGENTS.md           # Regras do projeto (segurança, privacidade, comunicação)
├── docker-compose.yml  # PostgreSQL, Redis e RabbitMQ
├── frontend/           # Interface web em React + Vite (ver frontend/README.md)
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

**Pré-requisitos:** Go 1.27.4 (o Go baixa essa versão sozinho com `GOTOOLCHAIN=auto`), Node.js 24 com npm, Docker com o Docker Compose, Goose, uma conta Resend e um cliente OAuth Web no Google Cloud.

Configure as variáveis de ambiente conforme [Desenvolvimento local](backend/docs/development.md) e depois:

```bash
make infra/up                                # sobe o PostgreSQL
docker compose up -d --wait redis-shortener rabbitmq-shortener
make migration/up                            # aplica as migrations
make run                                     # sobe a API
make worker                                  # sobe o worker de cliques (outro terminal)
make front/install                           # dependências do frontend (uma vez)
make front/dev                               # sobe o frontend (outro terminal)
```

O `make infra/up` sobe só o PostgreSQL. A API funciona sem Redis e sem RabbitMQ: registra um aviso e continua sem cache e contando cliques de forma síncrona.

| Serviço | Endereço |
| --- | --- |
| Frontend | http://localhost:5173 |
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
| `make test` | Testes unitários do backend |
| `make test/integration` | Testes de integração. Sobem PostgreSQL, Redis e RabbitMQ descartáveis com Testcontainers, então o Docker precisa estar ligado |
| `go -C backend vet ./...` | Análise estática |
| `go -C backend test -race ./...` | Detector de race conditions, para mudanças em concorrência, sessões, cache e rate limiting |
| `gofmt -w <arquivos>` | Formatação |
| `make front/test` | Testes do frontend (Vitest + Testing Library) |
| `make front/lint` | Lint do frontend (oxlint) |
| `make front/build` | Checagem de tipos e build de produção do frontend |

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
| [`backend/docs/deployment.md`](backend/docs/deployment.md) | Deploy grátis: Render, Neon, Upstash, CloudAMQP, Resend, Google, DNS e monitor |
| [`backend/docs/decisions/`](backend/docs/decisions/README.md) | Decisões de arquitetura (ADRs) |
