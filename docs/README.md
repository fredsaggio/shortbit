# Documentação da ShortBit

> Status: atual
> Última atualização: 29 de setembro de 2026

Esta pasta explica como o sistema funciona. Ela complementa os documentos da raiz:

- [`PROJECT_CONTEXT.md`](../PROJECT_CONTEXT.md): visão, regras e decisões gerais do produto;
- [`IMPLEMENTATION_PLAN.md`](../IMPLEMENTATION_PLAN.md): ordem de implementação e progresso;
- `docs/`: explicação técnica dos fluxos e contratos.

## Mapa da documentação

| Documento | Pergunta respondida |
|---|---|
| [Autenticação](authentication.md) | Como cadastro, login, Google, cookies e sessões funcionam? |
| [Links e shortcodes](short-links.md) | Como criação, resolução, privacidade e analytics devem funcionar? |
| [API HTTP](api.md) | Quais endpoints existem, o que recebem e o que devolvem? |
| [OpenAPI](openapi.yaml) | Contrato de todas as rotas implementadas, renderizado em `/docs` pela API. |
| [Modelo de dados](data-model.md) | Quais tabelas existem e como se relacionam? |
| [Glossário](glossary.md) | O que significam os códigos, tokens e termos usados no projeto? |
| [Desenvolvimento](development.md) | Como executar, migrar e testar o projeto localmente? |
| [Decisões](decisions/README.md) | Onde registrar decisões arquiteturais e suas consequências? |

## Legenda

- ✅ Implementado e testado.
- 🚧 Em desenvolvimento.
- 📋 Planejado, ainda sem implementação completa.
- ⏸️ Decisão adiada deliberadamente.

## Regra de manutenção

Uma mudança de comportamento deve atualizar o documento responsável por ela. Evitar repetir a mesma explicação em vários arquivos: a API descreve o contrato HTTP, enquanto os documentos de domínio explicam o fluxo interno e suas razões.
