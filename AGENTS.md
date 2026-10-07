# Regras do Projeto

## Segurança e Privacidade
- **Arquivos sensíveis / `.env`:** NUNCA leia, visualize, exiba ou inspecione arquivos `.env` ou qualquer arquivo contendo credenciais e segredos sem a permissão explícita do usuário.
- **Alternativa para checagem:** Para verificar variáveis necessárias ou configurações de ambiente, consulte exclusivamente arquivos de exemplo como `.env.example` ou os esquemas de configuração do código (ex: `app/core/config.py`).

## Pesquisa e Padrões de Segurança
- **Consulta a Padrões da Indústria:** Sempre que projetar, refatorar, implementar ou revisar tópicos técnicos de segurança (como autenticação, autorização, hashing de senhas, controle de acesso RBAC, proteção contra injeções, CORS, tokens JWT/OAuth2, gerenciamento de sessões, cabeçalhos HTTP e validação de entradas), consulte e fundamente as decisões em fontes e guias de autoridade reconhecidos, em especial:
  - **OWASP** (OWASP Top 10, OWASP API Security Top 10, Cheat Sheet Series, ASVS).
  - **NIST** (ex: NIST SP 800-63 para gerenciamento de identidades e autenticação).
  - **RFCs e especificações oficiais** (ex: RFC 7519 para JWT, RFC 6749 para OAuth 2.0).
- **Abordagem Proativa:** Se houver dúvida ou ambiguidade sobre a abordagem mais segura para um problema, utilize ferramentas de pesquisa e documentação para validar a melhor prática atual antes de gerar código ou sugerir soluções.

## Comunicação e Prevenção de Alucinações
- **Esclarecimento Prévio de Dúvidas:** Se houver qualquer dúvida, ambiguidade ou especificação incompleta no prompt/pedido do usuário, a IA DEVE fazer perguntas e esclarecer os pontos ANTES de traçar planos ou gerar/modificar qualquer código.
- **Tolerância Zero a Suposições:** Nunca assuma regras de negócio, escopos ou comportamentos incertos. Confirme os detalhes com o usuário para evitar alucinações e retrabalho.
