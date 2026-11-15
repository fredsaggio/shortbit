# Observabilidade

> Status: atual
> Última atualização: 15 de novembro de 2026

A API e o worker mandam **métricas** e **logs** por OpenTelemetry (OTLP/HTTP). Em produção, eles vão para o Grafana Cloud. Localmente, vão para um Grafana que sobe com o `docker compose`. A decisão está no [ADR 0006](decisions/0006-opentelemetry-grafana-cloud.md).

Sem `OTEL_EXPORTER_OTLP_ENDPOINT`, nada sai do processo: os instrumentos gravam num provedor vazio, e os logs continuam só no stdout. Ou seja, a variável é opcional.

## O que é medido

O Prometheus troca pontos por `_` e acrescenta a unidade: `shortbit.redirects` vira `shortbit_redirects_total`, e `http.server.request.duration` vira `http_server_request_duration_seconds_*`.

| Métrica | Tipo | Atributos | O que responde |
|---|---|---|---|
| `http.server.request.duration` | histograma (s) | `http.route`, `http.request.method`, `http.response.status_code` | Volume, erros e tempo de resposta por rota |
| `shortbit.redirects` | contador | `result` (`redirected`, `password_required`, `not_found`, `failed`), `source` (`link`, `qr`) | Quantos links foram abertos, e por onde |
| `shortbit.destination_cache.lookups` | contador | `result` (`hit`, `miss`, `error`) | Quanto o Redis poupa o PostgreSQL |
| `shortbit.click_events.published` | contador | `outcome` (`queued`, `synchronous`, `not_recorded`) | Se a fila está de pé (`synchronous` é o plano B direto no banco) |
| `shortbit.click_events.processed` | contador | `outcome` (`counted`, `duplicate`, `discarded`, `failed`, `dead_lettered`) | O que o worker fez com cada evento |
| `shortbit.click_events.lag` | histograma (s) | — | Quanto o número da página do link atrasa em relação ao clique |
| `shortbit.security_events` | contador | `event` | Falhas de login, bloqueios por limite e mudanças de conta |
| `db.client.connections.{in_use,idle,max}` | gauge | — | Pool do PostgreSQL |
| `go.*` | vários | — | Memória, goroutines e GC (`contrib/instrumentation/runtime`) |

**Regra de cardinalidade:** atributo só recebe palavras fixas. Código de link, email, IP ou ID de usuário nunca vira atributo, porque cada valor diferente cria uma série nova. Isso estoura o limite do plano e expõe quem fez o quê. A rota é o padrão do `ServeMux` (`/urls/{code}`), nunca o caminho. `none` marca a requisição respondida antes de escolher a rota, como no bloqueio do limite global.

As métricas saem a cada 60 segundos. `OTEL_METRIC_EXPORT_INTERVAL` muda esse intervalo, em milissegundos.

## Logs

Toda linha que vai para o stdout também vai para o coletor, com os mesmos campos e a mesma redação de emails e credenciais (veja "Logs" em [development.md](development.md)). No Loki, os campos ficam pesquisáveis: `{service_name="shortbit-api"} | route="/{code}" | status="404"`.

### Eventos de segurança

Seguem o [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html). Cada um é uma linha `security event` com o campo `event` e soma 1 em `shortbit.security_events{event=...}`.

| `event` | Nível | Quando | Campos |
|---|---|---|---|
| `login_failed` | WARN | Senha errada no login, na troca de senha ou na exclusão da conta | `reason`, `user_id` quando a sessão existe |
| `google_login_failed` | WARN | Callback do Google recusado | `reason` (`provider_rejected`, `missing_code_or_state`, `missing_cookies`, `state_mismatch`, `authentication_failed`, `internal_error`) |
| `rate_limited` | WARN | Qualquer limite de tentativas recusou a requisição | `limiter` (`global`, `login`, `login_email`, `link_access`, `account_password`...) |
| `link_password_failed` | WARN | Senha errada de link privado | — |
| `verification_code_failed` | WARN | Código errado ou tentativa travada, no cadastro ou na recuperação | `flow`, `reason` |
| `account_created` | INFO | Cadastro com senha confirmado | `user_id`, `method` |
| `password_changed` | INFO | Troca de senha | `user_id` |
| `password_reset_completed` | INFO | Recuperação de senha concluída | — |
| `account_deleted` | INFO | Conta excluída | `user_id` |

O evento nunca leva email, IP, senha, código ou token: o limitador aparece pelo nome, não pela chave.

## Desenvolvimento

```bash
make up
```

Sobe também o `grafana/otel-lgtm` (Grafana, Prometheus, Loki e um coletor OpenTelemetry num contêiner só). A API e o worker do Compose já mandam para ele. Abra `http://localhost:3000`: o painel **shortbit** já vem carregado, e dá para ver sem login. Para editar, entre com `admin`/`admin`. O Grafana só escuta em `localhost`.

Com a API fora do Compose (`make run`), suba só o Grafana e aponte para ele:

```bash
docker compose up -d --wait grafana-shortener
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 make run
```

O painel é o arquivo [`observability/shortbit-dashboard.json`](../../observability/shortbit-dashboard.json). Uma mudança feita na tela do Grafana só fica no repositório se o JSON for exportado e salvo de volta nesse arquivo.

## Produção (Grafana Cloud)

1. Crie a conta grátis em grafana.com e um stack na região `us-east` (perto do Render).
2. No stack, abra **Connections → Add new connection → OpenTelemetry (OTLP)**. Gere um token com permissão de escrita de métricas e logs. A tela mostra o endpoint (`https://otlp-gateway-prod-us-east-<n>.grafana.net/otlp`) e o cabeçalho pronto (`Authorization=Basic%20<base64 de instanceID:token>`).
3. No Render, em **Environment**, crie:

   | Variável | Valor |
   |---|---|
   | `OTEL_EXPORTER_OTLP_ENDPOINT` | o endpoint da tela |
   | `OTEL_EXPORTER_OTLP_HEADERS` | `Authorization=Basic%20...` (segredo) |
   | `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=production` (já está no `render.yaml`) |

   O `%20` é o espaço codificado. Use essa forma, que não precisa de aspas no `.env`.
4. Depois do deploy, o log do Render deve mostrar `telemetry on: sending metrics and logs over OTLP`. Se aparecer `telemetry export failed`, o endpoint ou o token estão errados. A API segue funcionando e tenta de novo no próximo envio.
5. No Grafana Cloud, vá em **Dashboards → New → Import** e envie o `observability/shortbit-dashboard.json`. No topo do painel, escolha as fontes de métricas (`grafanacloud-<stack>-prom`) e de logs (`grafanacloud-<stack>-logs`).

Em produção, o worker roda dentro da API (`RUN_CLICK_WORKER=true`), então tudo aparece como `shortbit-api`.

### Alertas

Crie em **Alerting → Alert rules**, com notificação por email. Os limites são pontos de partida para o tráfego atual e devem ser ajustados com o tempo:

| Alerta | Consulta (Prometheus) | Dispara quando | Por quê |
|---|---|---|---|
| Erros 5xx | `100 * sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m])) / sum(rate(http_server_request_duration_seconds_count[5m]))` | > 5 por 5 min | Algo quebrou para quem usa |
| Redirect lento | `histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{http_route="/{code}"}[5m])))` | > 1 s por 10 min | O produto é o redirect |
| Fila de cliques fora | `sum(rate(shortbit_click_events_published_total{outcome!="queued"}[5m]))` | > 0 por 10 min | O RabbitMQ caiu e o plano B está segurando |
| Cliques atrasados | `histogram_quantile(0.95, sum by (le) (rate(shortbit_click_events_lag_seconds_bucket[10m])))` | > 300 s por 10 min | O worker parou ou está lento |
| Ataque de senha | `sum(increase(shortbit_security_events_total{event=~"login_failed\|link_password_failed"}[15m]))` | > 50 | Tentativas de adivinhar senhas |

O "site caiu" continua com o UptimeRobot, que funciona mesmo com a API parada.

### Limites do plano grátis

O plano grátis tem cotas mensais de séries de métricas e de volume de logs, além de uma retenção limitada. Os números mudam; confira na página de preços do Grafana Cloud. O intervalo de 60 segundos e a regra de cardinalidade mantêm o shortbit bem abaixo das cotas. Se o volume de logs apertar, o log de acesso (`http request`) é o primeiro candidato a sair do envio.

### Privacidade

Os logs e as métricas não têm email, IP, senha, token nem query string (veja "Logs" em [development.md](development.md)). O caminho do log de acesso traz o código do link (`/Ab3dX9`), que não identifica ninguém. A política de privacidade do site diz que registros técnicos de funcionamento ficam num serviço de monitoramento.
