# 0007 — País, visitantes únicos e robôs nos cliques

Status: Accepted
Data: 2026-11-16

## Contexto

A página do link mostra cliques por dia e por origem (ADR 0005). O dono quer saber também de que país vieram os cliques e quantas pessoas diferentes abriram o link. Até aqui a política de privacidade prometia não guardar nada sobre quem abre um link.

Há ainda um erro na contagem atual: quando alguém cola o link no WhatsApp, Telegram, Slack ou numa rede social, o servidor desses apps abre o link para montar a prévia, e essa visita conta como clique. Com o país, ela apareceria como o país do datacenter do app.

País, User-Agent e IP são dados de quem abre o link, não do dono. A LGPD trata IP e User-Agent como dados pessoais; números agregados, sem como voltar à pessoa, são dados anonimizados (art. 12).

## Opções consideradas

Para o país:

1. **Uma API externa de geolocalização por clique.** Acrescenta uma chamada de rede ao redirect, tem limite de uso e entrega o IP de cada visitante a um terceiro.
2. **O cabeçalho `CF-IPCountry` do Cloudflare.** Sem custo, mas o Render não documenta que o repassa, e o desenvolvimento local não tem Cloudflare: o país viria de lugares diferentes em cada ambiente.
3. **Uma base IP→país local**, lida da memória. Entre as grátis, a DB-IP Lite (CC BY 4.0, sem conta, mensal) e a IPinfo Lite (precisa de conta e token). Escolhida a DB-IP Lite.

Para visitantes únicos:

1. **Cookie de visitante.** Identifica o navegador por muito tempo e exigiria aviso de cookies no redirect, que não tem página.
2. **Hash de IP + User-Agent sem sal, ou com sal fixo.** Quem tiver o sal (ou ninguém, sem sal) refaz o hash para qualquer IP e descobre quem visitou.
3. **Hash com sal aleatório diário, apagado ao fim do dia**, como o Plausible. Escolhida.
4. **HyperLogLog no Redis.** O Redis é só cache (ADR 0002) e o plano grátis do Upstash limita comandos.

## Decisão

### Coleta

A API faz tudo no redirect, a partir do IP do cliente (o mesmo do rate limit, `CLIENT_IP_HEADER`) e do User-Agent, e põe no evento de clique só o resultado:

| Campo do evento | Conteúdo |
| --- | --- |
| `robot` | `true` numa requisição `HEAD` ou quando o User-Agent é de um robô conhecido (abaixo) |
| `country` | Código ISO 3166-1 alfa-2 (`BR`), ou ausente quando a base não conhece o IP |
| `visitor` | `HMAC-SHA256(sal do dia, url_id ‖ IP ‖ User-Agent)`, truncado em 16 bytes |

IP e User-Agent não vão para a fila, o banco nem os logs. Um robô não leva país nem `visitor`.

### Robôs

Reconhecidos por trechos fixos do User-Agent, sem diferenciar maiúsculas: robôs de prévia (WhatsApp, Telegram, Slack, Discord, Facebook/Meta, X, LinkedIn, Skype, Pinterest, Reddit, Embedly, Iframely, VK) e de busca (Google, Bing, DuckDuckGo, Yandex, Apple, Baidu). Não se usa o trecho genérico `bot`, que pega aparelhos reais (por exemplo, celulares Cubot). Um User-Agent vazio ou de ferramenta (`curl`) conta como pessoa: a lista pega só robôs que se identificam.

Uma requisição `HEAD` também é robô, qualquer que seja o User-Agent. A rota `GET /{code}` do Go atende `HEAD`, e quem a usa (verificadores de link, monitores, scanners de email) quer saber para onde o link aponta sem abri-lo; navegador nunca abre um link com `HEAD`. Ela recebe o mesmo `302` que um `GET`.

Uma visita de robô não soma em `urls.click_count`, em `url_daily_clicks` nem nos países: soma só em `url_daily_audience.robot_visits`. Antes desta mudança, as prévias contavam como clique; esses números antigos não são corrigidos.

### País

- Base `dbip-country-lite` em formato MMDB, lida com `github.com/oschwald/maxminddb-golang/v2`. O caminho vem de `GEOIP_DB_PATH`, opcional: vazio, ninguém tem país e o resto funciona.
- A imagem Docker baixa a base do mês no build (com a do mês anterior como reserva, para o começo do mês). Localmente, `make geoip` baixa para `backend/.geoip/`.
- Tabela `url_daily_countries (url_id, day, country, clicks)`. Só cliques de pessoas com país conhecido têm linha; os sem país são a diferença para o total de `url_daily_clicks`, calculada na API.
- A licença CC BY 4.0 exige um link para db-ip.com nas páginas que mostram os países. Ele fica no rodapé do site, que aparece em todas as páginas, como um link discreto "DB-IP".

### Visitantes únicos

- Tabela `visitor_salts (day, salt)`: um sal aleatório de 32 bytes por dia de `America/Sao_Paulo`. A API cria o do dia com `INSERT ... ON CONFLICT DO NOTHING` e lê o que ficou, então várias instâncias usam o mesmo sal. Guarda-o em memória até o dia virar. Se o banco falhar, o clique segue sem `visitor`.
- O worker, na mesma transação de `processed_click_events` (ADR 0002), insere `(url_id, day, visitor_hash)` em `url_daily_visitor_hashes` com `ON CONFLICT DO NOTHING`; só se inseriu soma 1 em `url_daily_audience.unique_visitors`. Uma reentrega não conta de novo.
- Os únicos são **por dia**. Quem volta no dia seguinte conta de novo; o total de um período é a soma dos dias e aparece com esse nome.
- O job horário do worker apaga os sais de dias anteriores ao de hoje e os hashes de dias anteriores a ontem. O dia extra dos hashes cobre um evento que espere na fila depois da meia-noite. Sem o sal, um hash guardado não pode ser refeito a partir de um IP.

### Privacidade

A página de privacidade passa a dizer o que é lido de quem abre o link, o que fica guardado e por quanto tempo. Base legal: legítimo interesse do dono do link em saber o alcance dele (LGPD, art. 7º, IX e art. 10).

## Consequências

- Cliques anteriores a esta mudança não têm país, únicos nem separação de robôs. No período que os inclui, aparecem como "sem país".
- O país de quem usa VPN, iCloud Private Relay ou operadora com saída em outro país sai errado; a base Lite também é menos precisa que a paga.
- Pessoas na mesma rede e com o mesmo navegador contam como um visitante; um celular que troca de Wi-Fi para 4G conta como dois. Nenhuma contagem sem cookie evita isso.
- Um robô que se passe por navegador conta como pessoa. A lista de robôs precisa de manutenção quando um app novo aparecer.
- A imagem cresce uns 8 MB com a base, e cada build baixa a base do db-ip.com: se o site estiver fora, o build falha.
- O banco ganha uma linha por visitante e por link nos dias guardados, apagada em até dois dias, e linhas por país que ficam, como `url_daily_clicks`.

Referências: [Plausible — Data policy](https://plausible.io/data-policy), [DB-IP — IP to Country Lite](https://db-ip.com/db/download/ip-to-country-lite), [LGPD (Lei 13.709/2018)](https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm), [OWASP — Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html).
