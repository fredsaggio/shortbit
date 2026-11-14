# ShortBit — Frontend

Interface web do ShortBit em React 19, TypeScript e Vite, sem biblioteca de componentes: o visual vem de `src/styles/global.css`.

## Telas

| Rota | Tela | Acesso |
| --- | --- | --- |
| `/` | Landing page com campo para encurtar | Livre |
| `/entrar` | Login com senha ou Google | Só sem sessão |
| `/cadastro` | Cadastro com código de confirmação por email | Só sem sessão |
| `/recuperar-senha` | Recuperação de senha por código | Livre |
| `/painel` | Criar links (público ou com senha) e listar os seus | Com sessão |
| `/painel/links/:code` | Detalhe de um link e total de cliques | Com sessão |

A página de senha do link privado e o redirect (`/{code}`) continuam na API, porque são abertos por quem recebe o link curto.

## Como rodar

Na raiz do repositório, com a API rodando (`make run`):

```bash
make front/install   # instala as dependências (npm ci)
make front/dev       # http://localhost:5173
```

O Vite repassa para a API (padrão `http://localhost:8080`) só os caminhos dela: `/registrations`, `/sessions`, `/me`, `/auth`, `/password-resets`, `/urls`, `/health` e `/docs`. Assim, navegador, front e API ficam na mesma origem e os cookies funcionam sem CORS.

Os caminhos vão **sem prefixo** (`/api/...`) de propósito: os cookies do cadastro, da recuperação de senha e do Google têm `Path` restrito a esses caminhos, e um prefixo faria o navegador deixar de enviá-los. Por isso as rotas do front usam nomes que não colidem com os da API. Em produção, um proxy reverso deve fazer o mesmo papel.

O login com Google volta para `FRONTEND_URL/painel` (configurado na API, padrão `http://localhost:5173`).

## Variáveis

Opcionais; veja `.env.example`.

| Variável | Padrão | Uso |
| --- | --- | --- |
| `VITE_API_PROXY_TARGET` | `http://localhost:8080` | Destino do proxy do Vite em desenvolvimento |
| `VITE_SHORT_LINK_BASE_URL` | `http://localhost:8080` | Base dos links curtos mostrados no painel; deve ser igual ao `PUBLIC_BASE_URL` da API |

## Qualidade

| Comando (na raiz) | O que faz |
| --- | --- |
| `make front/test` | Testes com Vitest e Testing Library. A API é simulada trocando o `fetch` (`src/test/fakeApi.ts`) |
| `make front/lint` | oxlint |
| `make front/build` | Checagem de tipos (`tsc -b`) e build de produção em `frontend/dist` |

## Estrutura

```text
src/
├── api/          # Cliente HTTP e chamadas da API (auth, links)
├── auth/         # Estado da sessão (AuthProvider + useAuth)
├── components/   # Layout, botões, campos, alertas, guardas de rota
├── pages/        # Uma tela por arquivo, com os testes ao lado
├── styles/       # CSS global e paleta de cores
└── test/         # Setup do Vitest, API falsa e helper de renderização
```

## Paleta

Inspirada no Bitly: azul-marinho, laranja e azul sobre branco. As cores são variáveis em `:root`, em `src/styles/global.css`.

| Variável | Cor | Uso |
| --- | --- | --- |
| `--navy` | `#0B1736` | Texto, hero, rodapé e card de cliques |
| `--orange` | `#EE6123` | Laranja da marca: logo, destaques, detalhes |
| `--orange-strong` | `#C74A16` | Fundo dos botões principais |
| `--blue` | `#2A5BD7` | Links, ícones, seletor ativo, faixa de chamada |
| `--page` | `#F4F6FA` | Fundo das páginas |
| `--surface` | `#FFFFFF` | Cabeçalho e cards |

Os botões usam `--orange-strong` porque texto branco sobre `#EE6123` tem contraste de 3,3:1, abaixo do mínimo de 4,5:1 (WCAG AA). Com `#C74A16` fica em 4,75:1.
