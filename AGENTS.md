# Regras do Projeto

## Segurança e Privacidade
- **Arquivos sensíveis / `.env`:** NUNCA leia, visualize, exiba ou inspecione arquivos `.env` ou qualquer arquivo contendo credenciais e segredos sem a permissão explícita do usuário.
- **Alternativa para checagem:** Para verificar variáveis necessárias ou configurações de ambiente, consulte exclusivamente arquivos de exemplo como `.env.example` ou os esquemas de configuração do código (ex: `backend/internal/config/config.go` e a seção de variáveis em `backend/docs/development.md`).

## Pesquisa e Padrões de Segurança
- **Consulta a Padrões da Indústria:** Sempre que projetar, refatorar, implementar ou revisar tópicos técnicos de segurança (como autenticação, autorização, hashing de senhas, controle de acesso RBAC, proteção contra injeções, CORS, tokens JWT/OAuth2, gerenciamento de sessões, cabeçalhos HTTP e validação de entradas), consulte e fundamente as decisões em fontes e guias de autoridade reconhecidos, em especial:
  - **OWASP** (OWASP Top 10, OWASP API Security Top 10, Cheat Sheet Series, ASVS).
  - **NIST** (ex: NIST SP 800-63 para gerenciamento de identidades e autenticação).
  - **RFCs e especificações oficiais** (ex: RFC 7519 para JWT, RFC 6749 para OAuth 2.0).
- **Abordagem Proativa:** Se houver dúvida ou ambiguidade sobre a abordagem mais segura para um problema, utilize ferramentas de pesquisa e documentação para validar a melhor prática atual antes de gerar código ou sugerir soluções.

## Sem co-author
Não coloque co-author em nenhum commit criado por você, em nenhuma hipótese.

## Comunicação e Prevenção de Alucinações
- **Esclarecimento Prévio de Dúvidas:** Se houver qualquer dúvida, ambiguidade ou especificação incompleta no prompt/pedido do usuário, a IA DEVE fazer perguntas e esclarecer os pontos ANTES de traçar planos ou gerar/modificar qualquer código.
- **Tolerância Zero a Suposições:** Nunca assuma regras de negócio, escopos ou comportamentos incertos. Confirme os detalhes com o usuário para evitar alucinações e retrabalho.

## Estrutura do projeto
- **Um só `Makefile`, na raiz.** Rode tudo com `make` a partir da raiz; `make help` lista todos os alvos.
- `frontend/`: interface em React + TypeScript + Vite. Detalhes, rotas e o porquê do proxy sem prefixo em `frontend/README.md`. As rotas do front não podem colidir com os caminhos da API repassados pelo proxy (`/registrations`, `/sessions`, `/me`, `/auth`, `/password-resets`, `/urls`, `/health`, `/docs`).
- `backend/`: módulo Go (`go.mod` próprio). Comandos Go rodam com `go -C backend ...` ou pelos alvos do `Makefile`.
- `backend/cmd/api/`: ponto de entrada da API. `backend/cmd/worker/`: worker que consome a fila de cliques (ADR 0002).
- `backend/internal/`: `app` (composition root e worker de cliques), `server` (rotas e health), `frontend` (serve o build do app em produção), `handlers`, `middleware`, `services` (regras de negócio), `repositories` (PostgreSQL), `models`, `config`, `db`, `cache` (Redis: frequência e destinos), `messaging` (RabbitMQ), `jobs`, `shortcodes` (Sqids), `argon2`, `googleoidc`, `email` (Resend), `sessiontoken`, `verificationcode`, `ctxval`, `logging` (logger com redação), `problem` (erros RFC 9457 e página de erro), `telemetry` (métricas, eventos de segurança e envio OTLP), `integration` (testes HTTP + PostgreSQL).
- `backend/migrations/`: migrations Goose, embarcadas no binário de testes.
- `backend/docs/`: `authentication.md`, `short-links.md`, `api.md`, `openapi.yaml`, `data-model.md`, `glossary.md`, `development.md`, `deployment.md`, `observability.md` e `decisions/` (ADRs).
- `observability/`: painel do Grafana (`shortbit-dashboard.json`) e a configuração que o carrega no Grafana local do Compose. Leia só quando mexer no assunto.
- Deploy: `Dockerfile` e `render.yaml` na raiz. Uma imagem só: a API serve o frontend e roda o worker (ADR 0003). Passo a passo em `backend/docs/deployment.md`.
- CI: `.github/workflows/ci.yml` roda as mesmas checagens da seção "Antes de terminar uma mudança" a cada push e PR na `main`, e o Render só faz deploy quando elas passam. Actions ficam fixadas pelo SHA do commit, com a versão em comentário.
- `IMPLEMENTATION_PLAN.md` (ordem das fases e progresso) e `PROJECT_CONTEXT.md` (produto e regras gerais) ficam só na máquina do usuário, ignorados pelo Git.

## Comandos (rodar na raiz)
| Comando | O que faz |
| --- | --- |
| `make infra/up` / `make infra/down` | Sobe só o PostgreSQL / para tudo. Redis e RabbitMQ sobem com `docker compose up -d --wait redis-shortener rabbitmq-shortener` |
| `make up` | Sobe tudo em contêineres (infra, API com o front em `:8080`, worker e Grafana em `:3000`), com a imagem de produção |
| `make run` | Sobe a API em `:8080` |
| `make worker` | Sobe o worker de cliques |
| `make front/install` / `make front/dev` | Instala as dependências do frontend / sobe o Vite em `:5173`, repassando as rotas da API para `:8080` |
| `make front/test` / `make front/lint` / `make front/build` | Testes (Vitest), lint (oxlint) e checagem de tipos + build do frontend |
| `make test` | Testes unitários |
| `make test/integration` | Testes de integração com Testcontainers (build tag `integration`): PostgreSQL, Redis e RabbitMQ, então o Docker precisa estar ligado. Um pacote novo com testes de integração precisa entrar na lista do alvo, no `Makefile` |
| `make migration/new name=descricao` | Cria migration Goose |
| `make migration/up` / `make migration/down` / `make migration/status` | Aplica / desfaz a última / mostra o estado |
| `make migration/validate` | Confere as migrations sem aplicar |

## Antes de terminar uma mudança
1. `gofmt -w <arquivos-alterados>`, `go -C backend vet ./...` e `make test` passam. Mexeu em concorrência, sessões, cache ou rate limiting: rode também `go -C backend test -race ./...`. Mexeu em repository, integração HTTP ou mensageria: rode `make test/integration`.
2. **Variável de ambiente nova e obrigatória** (`backend/internal/config/config.go`): documente em `backend/docs/development.md` e **avise o usuário para criá-la no `.env`**, porque sem ela a API não sobe.
3. Uma decisão difícil de reverter entra como ADR em `backend/docs/decisions/`, e o índice em `backend/docs/decisions/README.md` é atualizado. Uma mudança de comportamento atualiza o documento responsável em `backend/docs/`.
4. Mudou um endpoint: atualize `backend/docs/api.md` e `backend/docs/openapi.yaml`. O `backend/docs/redoc.html` lê o YAML no navegador e não precisa ser gerado.
5. Jobs e workers devem poder rodar de novo depois de um reinício ou de uma reentrega (idempotentes).
6. Mexeu no frontend: `make front/lint`, `make front/test` e `make front/build` passam.
7. **Nunca faça push, merge ou abra PR sem o usuário pedir.**
