# ShortBit — Frontend

Interface web do ShortBit em React 19, TypeScript e Vite, com Tailwind CSS v4 e componentes do shadcn/ui (Radix). A fonte é a Plus Jakarta Sans, e o logo usa a Nunito; as duas são servidas pelo próprio app (`@fontsource-variable`), sem chamar o Google Fonts.

Os formulários usam `noValidate` e mensagens próprias (`src/validation.ts` e `src/useFieldErrors.ts`) em vez dos balões do navegador. A API valida tudo de novo.

## Telas

| Rota | Tela | Acesso |
| --- | --- | --- |
| `/` | Landing page com campo para encurtar | Livre |
| `/entrar` | Login com senha ou Google | Só sem sessão |
| `/cadastro` | Cadastro com nome completo e código de confirmação por email | Só sem sessão |
| `/recuperar-senha` | Recuperação de senha por código | Livre |
| `/painel` | Criar links (públicos ou privados, com senha) e listar os seus | Com sessão |
| `/painel/links/:code` | Detalhe de um link e total de cliques | Com sessão |
| `/conta` | Minha conta: dados, nome completo e troca de senha (contas com senha) | Com sessão |

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
| `VITE_SHORT_LINK_BASE_URL` | `http://localhost:8080` em dev; a origem da página no build de produção | Base dos links curtos mostrados no painel; deve ser igual ao `PUBLIC_BASE_URL` da API |

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
│   └── ui/       # Componentes gerados pelo shadcn (npx shadcn@latest add <nome>)
├── lib/          # cn(), para juntar classes do Tailwind
├── pages/        # Uma tela por arquivo, com os testes ao lado
└── test/         # Setup do Vitest, API falsa e helper de renderização
```

`src/index.css` guarda os tokens (cores, raios, sombras) e o tema do shadcn. Os arquivos de `components/ui/` são gerados: o lint ignora neles a regra de hot reload, e a aparência do projeto fica nos wrappers de `components/` (`Button`, `TextField`, `Alert`...).

## Identidade visual

Copiada da do Bitly: cabeçalho, hero e rodapé azul-marinho, páginas em creme, cards brancos, laranja de destaque e azul nas ações. Só tema claro. Os tokens ficam em `src/index.css` e viram classes do Tailwind (`bg-navy`, `text-brand`, `bg-action`...).

| Token | Cor | Uso |
| --- | --- | --- |
| `navy` | `#031F39` | Texto, cabeçalho, hero, rodapé, card de cliques |
| `brand` | `#F36600` | Logo, ícones, avatar, anel de foco |
| `brand-700` | `#B34900` | Texto laranja sobre fundo claro (rótulos, 404) |
| `action` / `action-hover` | `#0058DD` / `#0041A3` | Botões principais e links |
| `surface` | `#F6F4EF` | Fundo das páginas |
| `card` | `#FFFFFD` | Cards e menus |
| `input` | `#8C8A86` | Borda dos campos |

Contraste (WCAG 2.2 AA): branco sobre `#0058DD` dá 6,1:1; o texto secundário `#62615F` sobre o creme dá 5,6:1; laranja sobre o azul-marinho dá 5,3:1. O laranja `#F36600` não leva texto branco nem vira texto sobre fundo claro (3,0:1); nesses casos entra o `brand-700`.

Interações: transições de 150 a 300 ms, desligadas com `prefers-reduced-motion`; foco visível em laranja; alvos de toque de 44 px; carregamento com skeleton; avisos de sucesso em toast (sonner).

## Senhas

- **Senha da conta** (cadastro, recuperação e troca): checklist ao vivo com o que a API exige: pelo menos 8 caracteres, uma letra maiúscula, um número e um caractere especial e, na troca, diferente da atual. A composição é uma decisão de produto contra o NIST SP 800-63B ([ADR 0004](../backend/docs/decisions/0004-account-password-composition.md)). A confirmação da senha é conferida no envio.
- **Senha do link** (home e painel): barra de força de 5 níveis calculada pelo [zxcvbn-ts](https://zxcvbn-ts.github.io/zxcvbn/), com dicionários comum e pt-BR. Abaixo de 8 caracteres a barra avisa o mínimo, que a API exige. Os dicionários (cerca de 500 KB comprimidos) ficam em arquivos à parte e só são baixados quando alguém escolhe "Privado".

## Produção

Em produção não há servidor só para o frontend: a API serve o build (`FRONTEND_DIST_DIR`), então app, API e links curtos ficam na mesma origem. O `Dockerfile` da raiz faz o build. Veja `backend/docs/deployment.md`.
