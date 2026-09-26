# ShortBit

ShortBit is a URL shortener for public or password-protected links with optional expiration. It tracks clicks, provides analytics, and supports password or Google sign-in.

> Projeto em desenvolvimento, construído em Go e PostgreSQL para estudo de backend, segurança, concorrência e evolução de uma API até produção.

## Estado atual

- cadastro com senha e confirmação de email;
- login com senha ou Google OIDC;
- sessões opacas em cookies, com `remember_me` opcional para login por senha e sessão lembrada por padrão no Google;
- autenticação, `/me` e logout;
- rate limiting e limpeza de cadastros pendentes;
- recuperação de senha por código enviado por email;
- domínio HTTP de URLs curtas ainda planejado.

## Documentação

O índice completo está em [`docs/README.md`](docs/README.md).

- [Autenticação](docs/authentication.md)
- [Links e shortcodes](docs/short-links.md)
- [API HTTP](docs/api.md)
- [Modelo de dados](docs/data-model.md)
- [Glossário](docs/glossary.md)
- [Desenvolvimento local](docs/development.md)
- [Decisões arquiteturais](docs/decisions/README.md)

## Execução rápida

Depois de configurar as variáveis de ambiente:

```bash
make infra/up
make migration/up
make run
```

Testes:

```bash
make test
make test/integration
```

Consulte [Desenvolvimento local](docs/development.md) para configuração do PostgreSQL, Resend e Google OAuth.
