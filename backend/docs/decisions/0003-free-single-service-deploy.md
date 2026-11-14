# 0003 — Deploy gratuito em um único serviço web

Status: Accepted
Data: 2026-11-14

## Contexto

O shortbit precisa ir ao ar em `shortbit.com.br` sem custo ou perto disso, mantendo o essencial: API, contagem de cliques pela fila com worker idempotente (ADR 0002), cache Redis, login com Google e email de confirmação.

Duas restrições vêm do próprio projeto:

- os cookies de sessão dependem de app e API na mesma origem, sem CORS;
- os links curtos ficam na raiz (`/{code}`), no mesmo domínio das telas do app.

## Opções consideradas

1. **VM Oracle Cloud Always Free** com o `docker compose` de hoje. Custo zero e nenhuma mudança de código. Mas a Oracle pode recuperar instâncias ociosas, e a documentação não garante que uma conta Pay As You Go fique de fora; recriar a VM pode esbarrar em falta de capacidade ARM na região.
2. **VPS paga** com o mesmo compose. Sem riscos de recuperação, mas custa todo mês.
3. **Render (plano grátis) + Neon + Upstash + CloudAMQP.** Custo zero e sem servidor para manter. O plano grátis do Render só tem serviço web (worker separado é pago) e dorme após 15 minutos sem tráfego.

## Decisão

Opção 3, com um único serviço no Render:

- a API roda o worker de cliques no mesmo processo (`RUN_CLICK_WORKER=true`), com o mesmo código do `cmd/worker`;
- a API serve o build do frontend (`FRONTEND_DIST_DIR`), então app, API e links dividem a origem `https://shortbit.com.br`; o gerador de códigos nunca devolve um caminho do app;
- as migrations rodam ao subir (`RUN_MIGRATIONS=true`), com trava consultiva do Postgres;
- um monitor externo chama `/health/live` a cada poucos minutos para o serviço não dormir. Um serviço ligado o mês todo (até 744 h) cabe nas 750 h grátis do Render;
- o rate limit usa o IP de `True-Client-IP` (`CLIENT_IP_HEADER`), porque atrás do proxy todos chegariam com o mesmo IP, e o `X-Forwarded-For` do Render pode ser forjado pelo cliente.

Tudo novo é opcional e desligado por padrão: o desenvolvimento local e o `cmd/worker` continuam iguais, e a mesma imagem Docker roda numa VPS se o projeto mudar de hospedagem.

## Consequências

- Custo zero, mas dependente de quatro planos grátis que podem mudar. Trocar de hospedagem é só trocar variáveis de ambiente: nada no código depende do Render.
- O worker divide CPU e memória com a API. Com pouco tráfego, isso não pesa; se crescer, basta rodar o `cmd/worker` em outro serviço e desligar `RUN_CLICK_WORKER`.
- Se o monitor externo falhar, a primeira visita depois de 15 minutos parados leva cerca de um minuto.
- O Neon grátis suspende o banco ocioso; a primeira consulta depois disso fica algumas centenas de milissegundos mais lenta. O health check não consulta o banco para não impedir essa suspensão.
- O CloudAMQP grátis limita mensagens por mês e conexões. Se o broker cair ou recusar a fila, a API conta o clique direto no PostgreSQL (ADR 0002).
- `True-Client-IP` vem do Cloudflare na frente do Render, comportamento relatado pelo suporte do Render mas não documentado por ele. Se o Cloudflare deixar de sobrescrever o cabeçalho, um cliente poderia mandar o seu e escolher a própria chave de rate limit. Vale conferir de tempos em tempos: um `True-Client-IP` forjado num `curl` não pode aparecer como o IP do cliente.
