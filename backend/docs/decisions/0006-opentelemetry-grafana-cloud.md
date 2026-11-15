# 0006 — Métricas e logs por OpenTelemetry, no Grafana Cloud

Status: Accepted
Data: 2026-11-15

## Contexto

Em produção, o shortbit só tinha o log do Render e o UptimeRobot. Não dava para responder "quantos redirects por minuto?", "o p95 piorou?", "a fila de cliques está de pé?" ou "alguém está tentando adivinhar senhas?" sem ler log linha por linha. O deploy é um único serviço no plano grátis do Render (ADR 0003): sem agente coletor, sem segundo serviço e sem disco.

## Opções consideradas

1. **Endpoint `/metrics` do Prometheus, raspado de fora.** É o padrão clássico. Mas exige um coletor que alcance o Render: o Grafana Cloud tem um raspador de URL pública, só que aí o `/metrics` fica exposto na internet e precisa de autenticação própria. Os logs ficariam de fora.
2. **OpenTelemetry com envio (push) por OTLP/HTTP para o Grafana Cloud.** A API envia métricas e logs direto, com um token no cabeçalho. Não precisa de agente nem de porta aberta. O mesmo código manda para um Grafana local em desenvolvimento.
3. **Serviços de log do Render (log streams).** Resolveria só os logs, e depende do plano.

## Decisão

Opção 2:

- SDK do OpenTelemetry para Go, com exportadores `otlpmetrichttp` e `otlploghttp`, configurados pelas variáveis `OTEL_*` padrão. O token fica só no ambiente.
- Desligado sem `OTEL_EXPORTER_OTLP_ENDPOINT`: a variável é opcional e a aplicação sobe igual.
- Métricas a cada 60 segundos, com atributos limitados a palavras fixas.
- Os logs saem pela ponte `otelslog`, através da mesma redação de emails e credenciais do stdout.
- Eventos de segurança com nome fixo (OWASP Logging Cheat Sheet), em log e em contador.
- Grafana Cloud (plano grátis) em produção; `grafana/otel-lgtm` no `docker compose` em desenvolvimento.
- Painel versionado em `observability/shortbit-dashboard.json`.

Traces ficaram de fora por enquanto: com uma API e um banco, a duração por rota já aponta onde está a lentidão. O mesmo SDK aceita traces depois, sem trocar de fornecedor.

## Consequências

- Registros técnicos (rota, status, duração, eventos sem dado pessoal) passam a ficar num serviço de terceiros. A política de privacidade diz isso.
- Se o Grafana Cloud cair ou o token expirar, a API só registra `telemetry export failed` e segue. Nada do usuário depende da telemetria.
- Trocar de fornecedor é mudar o endpoint e o cabeçalho: qualquer backend com OTLP serve.
- O painel e os alertas dependem dos nomes das métricas. Renomear uma métrica exige atualizar o JSON e as regras de alerta.
