# Regras do Projeto

## Segurança e Privacidade
- **Arquivos sensíveis / `.env`:** NUNCA leia, visualize, exiba ou inspecione arquivos `.env` ou qualquer arquivo contendo credenciais e segredos sem a permissão explícita do usuário.
- **Alternativa para checagem:** Para verificar variáveis necessárias ou configurações de ambiente, consulte exclusivamente arquivos de exemplo como `.env.example` ou os esquemas de configuração do código (ex: `internal/config/config.go` e a seção de variáveis em `docs/development.md`).

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
- `cmd/api/`: ponto de entrada da API. O worker de cliques vai para `cmd/worker/` (ADR 0002).
- `internal/`: `app` (composition root), `server` (rotas e health), `handlers`, `middleware`, `services` (regras de negócio), `repositories` (PostgreSQL), `models`, `config`, `db`, `cache` (Redis), `messaging` (RabbitMQ), `jobs`, `shortcodes` (Sqids), `argon2`, `googleoidc`, `email` (Resend), `sessiontoken`, `verificationcode`, `ctxval`, `integration` (testes HTTP + PostgreSQL).
- `migrations/`: migrations Goose, embarcadas no binário de testes.
- `docs/`: `authentication.md`, `short-links.md`, `api.md`, `openapi.yaml`, `data-model.md`, `glossary.md`, `development.md` e `decisions/` (ADRs). Leia só quando mexer no assunto.
- `IMPLEMENTATION_PLAN.md` (ordem das fases e progresso) e `PROJECT_CONTEXT.md` (produto e regras gerais) ficam só na máquina do usuário, ignorados pelo Git.

## Comandos (rodar na raiz)
| Comando | O que faz |
| --- | --- |
| `make infra/up` / `make infra/down` | Sobe só o PostgreSQL / para tudo. Redis e RabbitMQ sobem com `docker compose up -d --wait redis-shortener rabbitmq-shortener` |
| `make run` | Sobe a API em `:8080` |
| `make test` | Testes unitários |
| `make test/integration` | Testes de integração com Testcontainers (build tag `integration`), então o Docker precisa estar ligado. Um pacote novo com testes de integração precisa entrar na lista do alvo, no `Makefile` |
| `make migration/new name=descricao` | Cria migration Goose |
| `make migration/up` / `make migration/down` / `make migration/status` | Aplica / desfaz a última / mostra o estado |
| `make migration/validate` | Confere as migrations sem aplicar |

## Antes de terminar uma mudança
1. `gofmt -w <arquivos-alterados>`, `go vet ./...` e `make test` passam. Mexeu em concorrência, sessões, cache ou rate limiting: rode também `go test -race ./...`. Mexeu em repository, integração HTTP ou mensageria: rode `make test/integration`.
2. **Variável de ambiente nova e obrigatória** (`internal/config/config.go`): documente em `docs/development.md` e **avise o usuário para criá-la no `.env`**, porque sem ela a API não sobe.
3. Uma decisão difícil de reverter entra como ADR em `docs/decisions/`, e o índice em `docs/decisions/README.md` é atualizado. Uma mudança de comportamento atualiza o documento responsável em `docs/`.
4. Mudou `docs/openapi.yaml`: gere de novo o `docs/redoc.html` (comando em `docs/development.md`).
5. Jobs e workers devem poder rodar de novo depois de um reinício ou de uma reentrega (idempotentes).
6. **Nunca faça push, merge ou abra PR sem o usuário pedir.**
