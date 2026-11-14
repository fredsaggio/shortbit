# 0004 — Regras de composição na senha da conta

Status: Accepted
Data: 2026-11-14

## Contexto

Até aqui, a senha da conta exigia só tamanho: de 8 caracteres a 1024 bytes. O produto passou a pedir também uma letra maiúscula, um número e um caractere especial no cadastro.

## Opções consideradas

1. **Só tamanho**, como estava. É o que recomendam o NIST SP 800-63B (seção 3.1.1.2: o verificador *não deve* impor regras de composição) e o OWASP Authentication Cheat Sheet. Composição tende a gerar senhas previsíveis, como `Senha@123`.
2. **Subir o mínimo** (12 ou 15 caracteres), sem composição. É o que mais aumenta a força, segundo as mesmas fontes.
3. **Exigir composição**: 8+ caracteres, uma maiúscula, um número e um especial.

## Decisão

Opção 3, por decisão de produto, sabendo que vai contra o NIST e o OWASP.

- Vale para toda senha de conta nova ou trocada: cadastro, recuperação e troca. Valer só no cadastro deixaria trocar depois por uma senha mais fraca.
- **Maiúscula:** qualquer letra maiúscula Unicode (`É` conta). **Número:** qualquer dígito decimal. **Especial:** qualquer caractere que não seja letra, número nem espaço (símbolos e emoji contam; espaço não).
- A API é a autoridade (`services/password_policy.go`), e o frontend repete as mesmas classes para mostrar o checklist ao vivo.
- Senhas existentes continuam entrando: a regra só é checada quando uma senha é definida.
- A senha de link privado não muda: só o mínimo de 8 caracteres.

## Consequências

- Cada regra que falha tem sua mensagem, então a pessoa sabe o que falta.
- Senhas como `Senha@123` passam pela regra embora sejam fracas. Se o produto quiser força real, o caminho é aumentar o mínimo ou checar senhas vazadas (o OWASP recomenda bloquear senhas de listas de vazamento).
