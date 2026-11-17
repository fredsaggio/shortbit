# Decisões arquiteturais

> Status: atual
> Última atualização: 16 de novembro de 2026

Esta pasta recebe Architecture Decision Records (ADRs). Um ADR registra por que uma decisão importante foi tomada, quais alternativas foram consideradas e quais consequências foram aceitas.

## Quando criar um ADR

Criar somente para decisões difíceis de reverter ou que provavelmente serão questionadas novamente, como:

- sessão opaca em vez de JWT próprio;
- vínculo automático entre Google e conta confirmada por senha;
- estratégia definitiva de shortcode;
- adoção e política de cache Redis;
- garantia de contagem exata antes do redirect.

Não criar ADR para cada função, endpoint ou detalhe mecânico.

## Formato

```md
# NNNN — Título

Status: Proposed | Accepted | Deferred | Superseded
Data: AAAA-MM-DD

## Contexto

Qual problema exige uma decisão?

## Opções consideradas

Quais alternativas reais foram avaliadas?

## Decisão

Qual opção foi escolhida e por quê?

## Consequências

Quais benefícios, limitações e trabalhos futuros surgem dessa escolha?
```

## ADRs

- [0001 — Shortcodes com ID incremental e Sqids](0001-shortcode-sqids.md) — aceita; implementada.
- [0002 — Contagem de cliques via RabbitMQ e cache Redis de destinos quentes](0002-click-events-rabbitmq-redis-cache.md) — aceita; implementada.
- [0003 — Deploy gratuito em um único serviço web](0003-free-single-service-deploy.md) — aceita; implementada.
- [0004 — Regras de composição na senha da conta](0004-account-password-composition.md) — aceita; implementada.
- [0005 — Cliques por dia e por origem (link ou QR code)](0005-daily-clicks-and-qr-source.md) — aceita; implementada.
- [0006 — Métricas e logs por OpenTelemetry, no Grafana Cloud](0006-opentelemetry-grafana-cloud.md) — aceita; implementada.
- [0007 — País, visitantes únicos e robôs nos cliques](0007-country-unique-visitors-robots.md) — aceita; implementada.
