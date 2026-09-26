# 0001 — Shortcode de sete caracteres com ID incremental e FF1

Status: Accepted (implementação pendente)
Data: 2026-09-26

## Contexto

O projeto precisa de códigos públicos com **exatamente sete caracteres Base62**, sem colisões normais de geração e sem expor diretamente a sequência dos IDs. Há links públicos e privados; o código não é um mecanismo de autorização. O gerador atual usa dez caracteres aleatórios e ainda não há `POST /urls`.

## Opções consideradas

- Base62 aleatório com `UNIQUE` e retry: comprimento fixo, mas há chance de colisão e necessidade de retry.
- Sqids sobre o ID: sem colisões de codificação no domínio suportado, porém `MinLength: 7` não impõe máximo de sete e a transformação não é criptográfica.
- Base62 direto do ID com preenchimento: sem colisões, mas a enumeração é trivial.
- Permutação com FF1 do ID representado em sete posições Base62: comprimento fixo e bijeção no domínio, com proteção contra derivar a sequência a partir dos códigos observados.

## Decisão

Usar `urls.id BIGINT GENERATED ALWAYS AS IDENTITY`. Para cada ID entre 1 e `62^7 = 3.521.614.606.208`, representar `id - 1` em sete posições com alfabeto `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz` e aplicar FF1 com chave secreta estável. Persistir o resultado de exatamente sete caracteres em `urls.short_code`, com constraint `UNIQUE`. Acima do limite, recusar criação — jamais emitir oito caracteres, truncar ou reutilizar IDs. A constraint de comprimento da migration existente deve passar de mínimo oito para exatamente sete.

Como o código depende do ID e a coluna é `NOT NULL`, reservar o próximo ID da sequence antes do `INSERT` e inserir ID e código juntos. Validar a sintaxe escolhida e a concorrência por teste de integração. Lacunas na sequence são normais.

O redirect futuro buscará pelo valor exato da coluna, sem decodificar o shortcode. O código armazenado será imutável no MVP. A chave ficará em configuração externa, igual em todas as instâncias, fora de commits e logs, com backup. Biblioteca Go, representação da chave e eventual tweak serão escolhidos e validados na implementação; não implementar FF1 do zero.

## Consequências

- Com algoritmo, alfabeto, parâmetros e chave fixos, a transformação é uma permutação: IDs diferentes no domínio não colidem. Não há retry aleatório; `UNIQUE` serve como defesa contra bugs e incompatibilidades de configuração.
- FF1 dificulta enumeração sistemática a partir dos IDs, mas não impede força bruta do domínio nem torna códigos segredos. Links privados continuam protegidos por senha; operações administrativas por autenticação e ownership. Rate limit do redirect ajuda operacionalmente, sem substituir essas proteções.
- O limite é de IDs **consumidos** pela sequence ao longo da instalação, não de URLs ativas. Exclusões não recuperam capacidade.
- Alterar chave, alfabeto ou tweak na produção exige um plano de versionamento/migração e verificação de conflito com códigos existentes. O lookup exato preserva links antigos, mas, sozinho, não garante que novos códigos não conflitem com eles.
- Antes de criar a rota, substituir migration, gerador e testes legados. Testar vetores oficiais de FF1, round-trip, alfabeto, comprimento, limites, concorrência e configuração inválida.

Detalhamento operacional e estado atual: [Links e shortcodes](../short-links.md). O arquivo local `IMPLEMENTATION_PLAN.md` também descreve a ordem da Fase 8, mas está ignorado pelo Git; o documento versionado é a referência para uma nova sessão ou clone.

Referências: [NIST SP 800-38G](https://csrc.nist.gov/pubs/sp/800/38/g/upd1/final), [PostgreSQL `INSERT`](https://www.postgresql.org/docs/current/sql-insert.html), [PostgreSQL `pg_get_serial_sequence`](https://www.postgresql.org/docs/current/functions-info.html) e [Sqids FAQ](https://sqids.org/faq).
