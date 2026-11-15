# Deploy

> Status: atual
> Última atualização: 14 de novembro de 2026

O shortbit roda de graça em `https://shortbit.com.br` com um único serviço no Render, que serve a API, o frontend e o worker de cliques. O porquê está no [ADR 0003](decisions/0003-free-single-service-deploy.md).

| Peça | Onde | Plano |
|---|---|---|
| API + frontend + worker | Render (Docker, `render.yaml`) | Free |
| PostgreSQL | Neon | Free |
| Redis | Upstash | Free |
| RabbitMQ | CloudAMQP (Little Lemur) | Free |
| Email | Resend | Free, com o domínio verificado |
| DNS | Registro.br | — |
| Monitor | UptimeRobot (ou parecido) | Free |
| Métricas e logs | Grafana Cloud | Free |

Crie todos os serviços na região **AWS us-east-1 (N. Virginia)**, a mesma do Render em `virginia`. Assim a latência entre eles fica perto de 1 a 2 ms.

## A imagem

O `Dockerfile` na raiz faz o build do frontend e da API e entrega uma imagem distroless, sem shell e com usuário não-root. Para testar localmente contra a infraestrutura do Compose:

```bash
docker build -t shortbit:local .
docker run --rm --network host --env-file .env \
  -e HTTP_ADDR=:8092 -e RUN_MIGRATIONS=true -e RUN_CLICK_WORKER=true \
  shortbit:local
```

Depois abra `http://localhost:8092`.

## Variáveis de produção

| Variável | Valor | Por quê |
|---|---|---|
| `RUN_MIGRATIONS` | `true` | Aplica as migrations ao subir |
| `RUN_CLICK_WORKER` | `true` | O plano grátis do Render não tem worker separado |
| `FRONTEND_DIST_DIR` | `/app/web` (já na imagem) | A API serve o frontend |
| `CLIENT_IP_HEADER` | `True-Client-IP` | IP real do cliente para o rate limit, atrás do proxy |
| `COOKIE_SECURE` | `true` | Cookies só em HTTPS; também liga o HSTS (o navegador passa a usar só HTTPS no domínio e nos subdomínios por 1 ano) |
| `PUBLIC_BASE_URL` | `https://shortbit.com.br` | Base dos links curtos |
| `FRONTEND_URL` | `https://shortbit.com.br` | Destino do login com Google |
| `GOOGLE_REDIRECT_URL` | `https://shortbit.com.br/auth/google/callback` | Callback do Google |
| `EMAIL_FROM` | `shortbit <nao-responda@shortbit.com.br>` | Remetente verificado no Resend |
| `DATABASE_URL` | URL **direta** do Neon | Segredo |
| `REDIS_URL` | `rediss://...` do Upstash | Segredo |
| `RABBITMQ_URL` | `amqps://...` do CloudAMQP | Segredo |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google Cloud | Segredos |
| `RESEND_API_KEY` | Resend | Segredo |
| `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_HEADERS` | Grafana Cloud (opcionais) | O cabeçalho é segredo; veja o passo 9 |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=production` | Separa produção de desenvolvimento no Grafana |

O `render.yaml` já traz os valores fixos. Os segredos são pedidos pelo Render na criação e nunca vão para o Git.

## Passo a passo

### 1. Neon (PostgreSQL)

1. Crie o projeto na região AWS US East (N. Virginia), com Postgres 18 se disponível.
2. Em **Connect**, desligue **Connection pooling** e copie a URL. A trava das migrations vive na sessão do Postgres e não funciona através do pooler (PgBouncer em modo transação).
3. A URL termina em `?sslmode=require`. Ela é o `DATABASE_URL`.

### 2. Upstash (Redis)

1. Crie um banco Redis na região `us-east-1`, com TLS ligado.
2. Copie a URL `rediss://default:...@...upstash.io:6379`. Ela é o `REDIS_URL`.

Se o Redis cair, a API segue sem cache.

### 3. CloudAMQP (RabbitMQ)

1. Crie uma instância **Little Lemur** em AWS `us-east-1`.
2. Copie a **AMQP URL** e troque `amqp://` por `amqps://` se vier sem TLS. Ela é o `RABBITMQ_URL`.

A API usa 2 conexões (publicação e consumo). Depois do deploy, confira no log do Render as linhas `RabbitMQ click publisher ready` e `RabbitMQ click consumer ready`. Se aparecer erro ao declarar a fila `click_events.count`, o plano pode não aceitar fila quorum. Enquanto isso, a API conta os cliques direto no PostgreSQL.

### 4. Resend (email)

1. Em **Domains**, adicione `shortbit.com.br`.
2. Copie os registros DNS que o Resend mostrar (MX e TXT de SPF, TXT de DKIM) para o Registro.br, como no passo 7.
3. Espere o domínio aparecer como verificado e crie uma API key só com permissão de envio. Ela é o `RESEND_API_KEY`.

Sem o domínio verificado, o Resend só entrega emails para o dono da conta.

### 5. Google (login)

No Google Cloud Console, no cliente OAuth já usado localmente ou num novo:

1. Em **Authorized redirect URIs**, adicione `https://shortbit.com.br/auth/google/callback`.
2. Na tela de consentimento, adicione `shortbit.com.br` aos domínios autorizados e publique o app (em modo de teste, só os usuários de teste conseguem entrar).

### 6. Render

1. Em **New > Blueprint**, conecte o repositório do GitHub. O Render lê o `render.yaml`.
2. Preencha os segredos pedidos: `DATABASE_URL`, `REDIS_URL`, `RABBITMQ_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` e `RESEND_API_KEY`.
3. Espere o primeiro deploy. No log, procure `database migrations up to date`, `serving the web app` e `click worker started`.
4. Teste em `https://shortbit.onrender.com` antes de apontar o domínio.

Cada push na `main` roda o CI do GitHub Actions (`.github/workflows/ci.yml`): `gofmt`, `go vet`, testes unitários com `-race`, testes de integração, lint, testes e build do frontend, e o build da imagem Docker. O Render só faz o deploy do commit quando todas essas checagens passam (`autoDeployTrigger: checksPass` no `render.yaml`); um commit que quebra os testes nunca chega ao ar.

Duas configurações fora do código precisam estar certas, senão nenhum push gera deploy, sem aviso nenhum:

- **No GitHub**, em Settings → Applications → Installed GitHub Apps → Render → Configure, o repositório precisa estar em **Repository access**. Sendo público, o Render consegue baixar o código mesmo fora da lista e o primeiro deploy funciona, mas não recebe os avisos de push.
- **No Render**, em Settings → Build & Deploy → **Auto-Deploy**, a opção precisa ser **After CI Checks Pass**. O Blueprint não aplicou o `autoDeployTrigger` sozinho no primeiro deploy.

### 7. Domínio no Registro.br

1. No Render, em **Settings > Custom Domains**, adicione `shortbit.com.br` e `www.shortbit.com.br`. O Render mostra os registros a criar.
2. No Registro.br, abra o domínio e edite a zona DNS (se ela estiver no modo básico, mude para a edição avançada de zona). Crie:
   - para `shortbit.com.br`: o registro **A** com o IP que o Render mostrar;
   - para `www`: o **CNAME** para `shortbit.onrender.com`;
   - os registros do Resend do passo 4.
3. Espere a propagação (minutos a algumas horas). O Render emite o certificado HTTPS sozinho.

### 8. Monitor

No UptimeRobot (ou parecido), crie um monitor HTTP para `https://shortbit.com.br/health/live` a cada 5 minutos. Ele mantém o serviço acordado e te avisa por email se o site cair.

Use `/health/live`, nunca `/health/ready`: o `ready` consulta o banco e impediria o Neon grátis de suspender, gastando as horas de processamento do plano. Pelo mesmo motivo, o health check do Render também é o `live`.

### 9. Métricas e logs (Grafana Cloud)

Opcional, mas é onde se vê o site funcionando: requisições por rota, erros, tempo do redirect, fila de cliques e eventos de segurança. Passo a passo em [observability.md](observability.md#produção-grafana-cloud): criar o stack, gerar o token OTLP, preencher `OTEL_EXPORTER_OTLP_ENDPOINT` e `OTEL_EXPORTER_OTLP_HEADERS` no Render, importar o painel e criar os alertas.

## Conferências depois do deploy

- `https://shortbit.com.br` abre a home e o cadastro envia o código por email.
- Um link criado no painel redireciona e o clique aparece em alguns segundos.
- O cabeçalho `Content-Security-Policy` vem nas páginas do app (`curl -I https://shortbit.com.br/painel`).
- O `True-Client-IP` não pode ser forjado. As requisições precisam sair em rajada paralela: em sequência, a latência deixa o ritmo abaixo do limite (10 por segundo) e nada prova. Primeiro o controle, com o mesmo IP falso em todas; depois o teste, com um IP falso diferente em cada:

  ```bash
  seq 1 60 | xargs -P 60 -I{} curl -s -o /dev/null -w "%{http_code}\n" -H "True-Client-IP: 203.0.113.77" https://shortbit.com.br/health/live | sort | uniq -c
  sleep 5
  seq 1 60 | xargs -P 60 -I{} curl -s -o /dev/null -w "%{http_code}\n" -H "True-Client-IP: 198.51.100.{}" https://shortbit.com.br/health/live | sort | uniq -c
  ```

  O controle precisa ter `429`, provando que o limite funciona. No teste, o certo é aparecer `429` também: os IPs inventados foram trocados pelo seu IP real e todas contaram no mesmo limite. Se o teste vier todo `200`, o cabeçalho está sendo aceito do cliente; remova `CLIENT_IP_HEADER` e veja o [ADR 0003](decisions/0003-free-single-service-deploy.md).

  Isso não distingue "sobrescrito pelo IP real" de "removido no caminho" (aí todos os visitantes cairiam no IP do proxy e dividiriam um limite). Para isso, mantenha o seu limite estourado com uma rajada contínua e, ao mesmo tempo, abra `/health/live` de outra rede (por exemplo, o celular nos dados móveis): ele tem que responder `ok`.

## Limites dos planos grátis

São números informados pelos provedores, que podem mudar:

- **Render:** 750 horas por mês por conta. Um serviço ligado o mês inteiro usa até 744, então não crie um segundo serviço grátis. Sem tráfego por 15 minutos, o serviço dorme e acorda em cerca de um minuto (o monitor evita isso).
- **Neon:** suspende o banco ocioso e tem limites mensais de armazenamento e de processamento.
- **Upstash:** limite mensal de comandos. Cada redirect usa poucos comandos.
- **CloudAMQP Little Lemur:** cerca de 1 milhão de mensagens por mês e 20 conexões.

Para sair do grátis, a mesma imagem roda numa VPS com o `docker compose`: basta configurar as mesmas variáveis.
