# Decisões arquiteturais

> Status: estrutura preparada; primeira decisão registrada
> Última atualização: 26 de setembro de 2026

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

- [0001 — Shortcodes com ID incremental e Sqids](0001-shortcode-sqids.md) — aceita; implementação pendente.
