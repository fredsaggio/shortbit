# 0005 — Cliques por dia e por origem (link ou QR code)

Status: Accepted
Data: 2026-11-14

## Contexto

A página do link mostrava só o total de cliques. O dono quer ver os cliques de cada dia e saber quantos vieram de quem leu o QR code e quantos de quem abriu o link. O QR code guarda o link curto (nunca o destino), então, para o servidor, uma leitura do QR é um GET em `/{code}` como outro qualquer.

## Opções consideradas

Para guardar os cliques por dia:

1. **Uma linha por clique** (horário e origem). Permite qualquer recorte depois, mas cresce sem limite com o tráfego, e o banco é o Neon grátis, com pouco espaço.
2. **Um contador por link, dia e origem.** Cresce no máximo duas linhas por link por dia com clique e responde o gráfico com uma leitura pela chave primária.

Para marcar a leitura do QR code:

1. **Um caminho próprio**, como `/q/{code}`. Exige uma rota nova e a reserva de mais um caminho entre os códigos.
2. **Uma marca na query string**, `/{code}?qr=1`. A rota é a mesma, o destino não muda e o cache de destinos (chaveado pelo código) não é afetado.

## Decisão

- Tabela `url_daily_clicks (url_id, day, source, clicks)`, com chave primária `(url_id, day, source)` e `source` em `link` ou `qr`. O dia é a data do clique em `America/Sao_Paulo`, calculada pelo PostgreSQL a partir do `occurred_at` do evento, então um evento que espera na fila conta no dia em que o clique aconteceu.
- O worker e o fallback síncrono somam o dia na **mesma transação** que grava o `event_id` em `processed_click_events` e soma `urls.click_count` (ADR 0002). Uma reentrega não conta em lugar nenhum.
- O evento de clique ganha `source`. Um evento sem `source` (publicado antes desta mudança) conta como `link`; um valor desconhecido vai para a dead-letter.
- O QR code do front codifica `https://shortbit.com.br/{code}?qr=1`. No link privado, a página de senha mantém a marca no formulário e no redirect de volta, para o desbloqueio não perder a origem.
- Os dados por dia são guardados sem prazo de expiração.

## Consequências

- Os QR codes baixados antes desta mudança não têm a marca e seguem contando como link.
- A marca é pública: quem quiser pode abrir `?qr=1` à mão. Ela só rotula o clique, como o próprio clique já é anônimo e forjável.
- Os cliques anteriores a esta mudança ficam só no total (`click_count`); o gráfico começa no dia do deploy.
- A marca `?qr=1` vai impressa em QR codes: trocá-la depois deixaria os códigos já impressos contando como link. Ela deve continuar aceita para sempre.
- Se o espaço do Neon apertar, uma limpeza das linhas mais antigas que um prazo resolve sem mudar o formato.
