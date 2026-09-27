# Glossário

> Status: atual
> Última atualização: 24 de setembro de 2026

## Autenticação

Comprovar quem está fazendo o request. Na ShortBit, isso normalmente significa validar o cookie `user_session` e localizar a sessão no PostgreSQL.

## Autorização

Decidir o que o usuário autenticado pode fazer. Exemplo: estar autenticado não basta para consultar analytics; a URL também precisa pertencer ao usuário.

## Credencial

Algo usado para autenticar. `password_credentials` contém o hash que permite validar uma senha. Uma identidade Google é outro tipo de credencial, armazenada em `auth_identities`.

## Identidade

Representação de uma pessoa em determinado sistema. `users` é a identidade local; `provider=google + sub` é a identidade externa vinculada.

## Sessão

Estado criado depois do login. Associa um token aleatório a um usuário e possui expiração. Permite autenticar vários requests sem reenviar a senha.

## Token opaco

String aleatória sem informações interpretáveis pelo cliente. Diferente de JWT, não contém `user_id`, email ou permissões. A API precisa consultar a sessão no banco.

## Hash

Transformação unidirecional. O sistema armazena `SHA-256(token)` para localizar tokens sem guardar o segredo puro. Senhas usam Argon2id, que é propositalmente caro e apropriado para passwords.

## HMAC

Código de autenticação calculado com uma função hash e uma chave secreta. No cadastro, o token da tentativa funciona como chave e o código recebido por email como mensagem. Isso permite verificar o código sem armazená-lo puro.

## TTL

“Time to live”: quanto tempo algo permanece válido. Exemplos: código por 10 minutos, tentativa por 30 minutos e sessão pelo `USER_SESSION_TTL`.

## Cookie `HttpOnly`

Cookie que JavaScript da página não consegue ler diretamente. O navegador ainda o anexa automaticamente aos requests aplicáveis.

## Cookie `Secure`

Cookie enviado somente por HTTPS. Deve estar `false` apenas durante desenvolvimento local via HTTP.

## `SameSite`

Política que limita quando o navegador envia um cookie em navegações iniciadas por outro site. As sessões usam `Lax`; o cadastro usa `Strict`.

## OAuth 2.0

Protocolo de autorização usado no fluxo com Google. Ele fornece o authorization code e os endpoints necessários para trocá-lo por tokens.

## OpenID Connect (OIDC)

Camada de identidade sobre OAuth 2.0. O OIDC fornece o ID token que permite à ShortBit verificar quem autenticou no Google.

## Authorization code

Código curto e descartável que o Google envia ao callback. O backend o troca por tokens diretamente com o Google. Não é o token de sessão da ShortBit.

## ID token

JWT assinado pelo Google com informações de identidade, como `sub`, email e `email_verified`. É validado no callback e descartado; não vira a sessão da aplicação.

## Access token

Token que autorizaria chamadas a APIs do Google. A ShortBit não precisa consumir APIs Google e, portanto, não persiste access ou refresh tokens.

## `sub` / Subject

Identificador estável da conta dentro do provedor OIDC. É armazenado como `auth_identities.provider_user_id` e usado no login, em vez de tratar email como identidade permanente.

## `state`

Valor aleatório enviado no início do OAuth e exigido de volta no callback. Liga a resposta à tentativa iniciada pelo mesmo navegador e protege contra troca de fluxo/CSRF.

## `nonce`

Valor aleatório enviado na autorização e devolvido dentro do ID token assinado. Liga aquele token à tentativa atual e dificulta replay de um token de outro login.

## PKCE

Prova de posse aplicada à troca do authorization code. A aplicação cria um `code_verifier` secreto e envia ao Google somente um `code_challenge` derivado dele. No callback, apresenta o verifier original.

## `code_verifier`

Segredo temporário do PKCE guardado em cookie. Não é o token verifier da biblioteca e não é o authorization code.

## Token verifier

Componente da biblioteca OIDC que valida assinatura, algoritmo, issuer, audience e expiração do ID token usando as chaves públicas divulgadas pelo Google.

## Claim

Campo dentro de um token. Exemplos: `sub`, `aud`, `iss`, `exp`, `nonce`, `email` e `email_verified`.

## Provider

Sistema externo de identidade. Atualmente o valor implementado é `google`.

## Composition root

Lugar onde implementações concretas são criadas e conectadas: repositories, services, handlers, cliente Google e jobs. Na aplicação, fica em `internal/app/app.go`.

## Middleware

Componente que envolve handlers para executar comportamento comum, como request ID, logging, recovery, body limit, rate limit ou autenticação.

## Contexto do request

`context.Context` acompanha o request. O middleware de autenticação coloca nele o `user_id`, e handlers protegidos recuperam esse valor.

## Idempotência

Propriedade de repetir uma operação sem criar efeitos adicionais indesejados. Exemplo: repetir logout continua deixando a sessão revogada.

## Race condition

Resultado incorreto causado pela ordem imprevisível de operações concorrentes. Constraints, updates atômicos, transações e `ON CONFLICT` são usados para controlar essas corridas.

## Ownership

Relação de propriedade. Uma URL pode ser administrada somente quando `urls.user_id` corresponde ao usuário autenticado.

## Rate limit

Limite de requisições por chave e período. O projeto possui limite global por IP e limites específicos por IP ou email para operações sensíveis.

## Enumeração de contas

Ataque em que respostas diferentes revelam se um email está cadastrado. Login e recuperação devem evitar mensagens que confirmem a existência da conta.

## Shortcode

Identificador curto usado na URL pública. `POST /urls` codifica o ID incremental com Sqids e `MinLength: 6`, persistindo o resultado em `urls.short_code`. O código tem **pelo menos seis caracteres**, sem máximo fixo, e não é um segredo criptográfico. Ver [Links e shortcodes](short-links.md).

## Base62

Alfabeto formado por dez dígitos, 26 letras maiúsculas e 26 minúsculas, totalizando 62 caracteres.

## Cache-aside

Estratégia em que a aplicação consulta o cache primeiro e recorre ao PostgreSQL em caso de ausência ou falha. O banco continua sendo a fonte da verdade.
