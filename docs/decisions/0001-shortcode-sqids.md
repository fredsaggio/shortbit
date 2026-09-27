# 0001 — Shortcodes com ID incremental e Sqids

Status: Accepted (implementação pendente)
Data: 2026-09-26
Revisão: 2026-09-27 — substitui as propostas anteriores de FF1 e permutação aritmética própria por Sqids com comprimento mínimo de seis caracteres.

## Contexto

O projeto precisa de códigos públicos curtos, não sequenciais à primeira vista e derivados do ID incremental de cada URL. O gerador atual de dez caracteres aleatórios é legado; ainda não existe `POST /urls`. O shortcode identifica o link, mas não concede acesso administrativo nem substitui a senha de um link privado.

## Alternativas consideradas

- Base62 aleatório com `UNIQUE` e retry: chance de colisão e necessidade de retry.
- FF1: criptografia de formato preservado, mas dependência e gestão de chave desproporcionais ao requisito atual.
- Permutação aritmética própria: permitiria faixas de comprimento exato, mas introduziria um algoritmo próprio sem benefício suficiente.
- Hashids: antecessor de Sqids, também sem máximo de comprimento e sem proteção criptográfica.

## Decisão

Usar `urls.id BIGINT GENERATED ALWAYS AS IDENTITY` e a [biblioteca oficial Sqids para Go](https://github.com/sqids/sqids-go). Configurar `MinLength: 6` e codificar **um único número**, o ID positivo da URL. O código será alfanumérico, com **no mínimo seis caracteres**; sete ou mais são permitidos. `MinLength` não garante comprimento exato nem limite máximo, e a mudança de tamanho não ocorre necessariamente quando `62^6` IDs tiverem sido usados. Usar inicialmente o alfabeto e a blocklist padrão da versão da biblioteca fixada no `go.mod`, sem salt ou chave criptográfica.

Persistir o resultado em `urls.short_code` com `UNIQUE`. O redirect buscará pelo valor exato armazenado, sem decodificá-lo. Isso também evita tratar como válidas strings não canônicas: Sqids pode decodificar strings diferentes para a mesma entrada numérica. Como `short_code` é `NOT NULL` e depende do ID, reservar um ID da sequence antes do `INSERT` e inserir ID e código juntos; validar a sintaxe SQL em teste de integração. Lacunas na sequence são normais.

## Consequências

- IDs distintos geram códigos distintos com a mesma configuração, conforme a garantia da biblioteca. `UNIQUE` detecta bugs ou conflitos decorrentes de configuração/versão alterada; não há retry aleatório no fluxo normal.
- Sqids não é criptografia nem defesa contra enumeração. Senha de link privado, autenticação, ownership e rate limit continuam independentes do shortcode.
- Fixar a versão da dependência e manter alfabeto, `MinLength` e blocklist consistentes em todas as instâncias. Antes de atualizá-los, conferir vetores de regressão; uma mudança pode produzir códigos diferentes para IDs novos e até conflitar com códigos já persistidos.
- `Encode` retorna erro e ele deve ser tratado. A blocklist pode exigir novas tentativas e, se excessivamente restritiva, fazer a codificação falhar.
- Antes de `POST /urls`, adaptar migration e gerador legados, testar mínimo de seis, ausência de máximo artificial, determinismo, entradas inválidas, concorrência e integração HTTP–PostgreSQL. A migration atual ainda exige no mínimo oito caracteres; ela **não foi alterada nesta etapa documental**.

Detalhes e estado atual: [Links e shortcodes](../short-links.md). O arquivo local `IMPLEMENTATION_PLAN.md` também descreve a ordem da Fase 8, mas está ignorado pelo Git; este documento versionado é a referência para um clone novo.

Referências: [Sqids Go](https://github.com/sqids/sqids-go), [FAQ do Sqids](https://sqids.org/faq).
