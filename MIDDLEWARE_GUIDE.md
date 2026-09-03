# Guia de middlewares HTTP em Go

Este documento é uma referência para iniciar APIs usando `net/http`. A ideia não é adicionar todos os middlewares possíveis a todo projeto, mas entender quais problemas eles resolvem e escolher conscientemente.

## 1. O que é um middleware

Um middleware é uma função que recebe um `http.Handler`, acrescenta um comportamento antes e/ou depois dele e devolve outro `http.Handler`.

```go
func Example(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Executado antes do handler seguinte.

		next.ServeHTTP(w, r)

		// Executado depois do handler seguinte.
	})
}
```

Ele permite aplicar uma regra a várias rotas sem repetir o mesmo código em cada handler.

Sem middleware:

```text
Handler A -> cria request ID, registra log e trata panic
Handler B -> cria request ID, registra log e trata panic
Handler C -> cria request ID, registra log e trata panic
```

Com middleware:

```text
Request ID -> Access log -> Recovery -> qualquer handler
```

Middleware não é uma funcionalidade exclusiva de frameworks. Bibliotecas como Chi, Gin e Echo apenas fornecem implementações prontas e mecanismos para conectá-las.

## 2. Middlewares fundamentais

### 2.1 Request ID

Cria um identificador diferente para cada requisição e normalmente o coloca:

- no contexto da requisição, para que handlers e services possam consultá-lo;
- no header `X-Request-ID` da resposta, para que cliente e servidor consigam falar sobre a mesma requisição;
- nos logs gerados durante aquela requisição.

Exemplo:

```text
request_id=abc123 method=POST path=/urls status=500
request_id=abc123 error="failed to insert URL"
```

Os dois registros podem ser relacionados porque possuem o mesmo `request_id`.

Sem Request ID, ainda existem logs, mas é mais difícil descobrir quais deles pertencem à mesma requisição quando várias requisições acontecem simultaneamente.

O Request ID:

- não precisa ser armazenado no banco por padrão;
- não identifica o usuário;
- não concede autorização;
- normalmente existe apenas durante a requisição e nos logs;
- pode ser gerado pela aplicação ou recebido de um proxy confiável.

### 2.2 Access log

Registra uma linha resumindo cada requisição atendida pela aplicação. Campos comuns:

```text
request_id
method
path
status
duration_ms
response_bytes
```

Exemplo:

```text
request_id=abc123 method=GET path=/X8pQa91K status=302 duration_ms=4 response_bytes=41
```

Sem esse middleware, a API continua funcionando, mas a própria aplicação não mantém um registro uniforme de todas as requisições. Uma plataforma como Render ou AWS pode gerar logs externos, porém ela normalmente não conhece todo o contexto interno da aplicação.

O `http.ResponseWriter` não oferece um método para perguntar, depois da resposta, qual status foi enviado. Por isso, um access log manual costuma envolver o writer para registrar chamadas a `WriteHeader` e `Write`.

Cuidados:

- o status HTTP implícito é `200` quando o handler chama `Write` sem chamar antes `WriteHeader`;
- apenas a primeira chamada a `WriteHeader` define o status enviado;
- o wrapper deve encaminhar corretamente a resposta ao writer original;
- recursos opcionais como streaming, WebSocket e `http.Flusher` exigem atenção adicional do wrapper.

### 2.3 Recovery

Executa o próximo handler dentro de uma função com `defer` e `recover`. Se acontecer um `panic`, o middleware:

1. recupera o controle daquela execução;
2. registra o erro e a stack trace;
3. devolve uma resposta `500 Internal Server Error`, quando ainda for possível;
4. evita que o panic interrompa o restante da cadeia daquela requisição de maneira descontrolada.

Sem Recovery próprio, o servidor `net/http` possui proteção no nível da conexão, portanto um panic de um handler normalmente não encerra todo o processo. Entretanto, a aplicação perde controle sobre o formato da resposta e sobre como aquele erro será registrado.

Recovery é uma última barreira para erros inesperados. Ele não substitui o tratamento normal de erros:

```go
url, err := service.Create(...)
if err != nil {
	// Tratar e retornar o status apropriado.
}
```

Também é importante não enviar ao cliente a mensagem do panic ou a stack trace. Esses dados ficam somente nos logs.

## 3. Ordem dos middlewares

Ao envolver handlers manualmente, a última atribuição se torna a camada mais externa e, por isso, é executada primeiro na entrada da requisição.

```go
handler := http.Handler(mux)
handler = middleware.Recovery(handler)
handler = middleware.AccessLog(handler)
handler = middleware.RequestID(handler)
```

A requisição percorre:

```text
RequestID -> AccessLog -> Recovery -> Router/Handler
```

A resposta retorna no sentido contrário:

```text
Router/Handler -> Recovery -> AccessLog -> RequestID
```

Essa ordem produz comportamentos importantes:

- `RequestID` roda primeiro e disponibiliza o ID para as outras camadas;
- `Recovery` transforma um panic do handler em resposta `500`;
- `AccessLog` recebe essa resposta e registra o status `500`;
- o header com Request ID também aparece na resposta de erro.

Se `AccessLog` estivesse dentro de `Recovery`, um panic poderia impedir a execução da parte do access log que acontece depois de `next.ServeHTTP`.

## 4. Middlewares que dependem do projeto

Nem toda API precisa dos middlewares abaixo desde o primeiro dia.

### Autenticação

Identifica o usuário por cookie de sessão, token ou outro mecanismo. Deve ser aplicado somente às rotas que exigem login.

Sem autenticação, qualquer cliente pode acessar as rotas, a menos que a proteção seja feita dentro de cada handler.

### Autorização

Verifica se o usuário autenticado pode realizar determinada ação. Autenticação responde “quem é você?”; autorização responde “você pode fazer isso?”.

No URL Shortener, por exemplo, um usuário autenticado não deve conseguir consultar ou alterar uma URL pertencente a outro usuário.

### Rate limiting

Limita requisições por algum critério, como IP, usuário ou combinação de IP e recurso. É útil especialmente em:

- login;
- desbloqueio por senha;
- recuperação de conta;
- endpoints públicos caros;
- prevenção de abuso.

Sem rate limiting, esses endpoints ficam mais expostos a força bruta e consumo excessivo de recursos.

### CORS

Define quais aplicações web, executadas em outros origins, podem chamar a API pelo navegador. Só é necessário quando frontend e API usam origins diferentes ou quando a API será consumida por sites externos.

CORS é uma proteção aplicada pelo navegador. Ele não substitui autenticação nem impede chamadas feitas por clientes como `curl`.

### Limite de tamanho do corpo

Impede que o servidor aceite corpos de requisição grandes demais. Pode ser aplicado globalmente ou somente em rotas que recebem JSON ou arquivos.

Sem limite, um cliente pode consumir memória, largura de banda e tempo de processamento enviando payloads excessivos.

### Timeout

Timeouts principais costumam ser configurados no `http.Server`, não necessariamente como middleware:

```go
server := &http.Server{
	Addr:              ":8080",
	Handler:           handler,
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       10 * time.Second,
	WriteTimeout:      15 * time.Second,
	IdleTimeout:       60 * time.Second,
}
```

Também podem existir timeouts por operação usando `context`, por exemplo em consultas ao banco ou chamadas HTTP externas.

## 5. O que a infraestrutura pode fornecer

Proxies, plataformas de deploy e API gateways podem oferecer:

- access logs;
- geração de Request ID;
- TLS/HTTPS;
- rate limiting;
- métricas;
- proteção contra certos tipos de tráfego abusivo.

Isso não significa automaticamente que a aplicação não precise desses mecanismos.

```text
Infraestrutura enxerga:
cliente -> proxy -> aplicação -> status externo

Aplicação enxerga:
usuário -> handler -> service -> repository -> erro interno
```

Antes de retirar um middleware porque a plataforma oferece algo semelhante, confirme:

- quais campos são registrados;
- por quanto tempo os registros são mantidos;
- se o mesmo Request ID chega à aplicação;
- se os logs permitem relacionar erros internos à requisição externa;
- se o comportamento continuará existindo ao trocar de plataforma.

## 6. Como testar middlewares

Middlewares próprios devem ser testados porque fazem parte do código da aplicação. Middlewares fornecidos por uma biblioteca não precisam ter sua implementação interna testada novamente, mas pode valer um teste de integração confirmando que foram conectados corretamente.

Use `net/http/httptest` para executar requests sem iniciar uma porta real:

```go
request := httptest.NewRequest(http.MethodGet, "/health", nil)
response := httptest.NewRecorder()

handler.ServeHTTP(response, request)
```

### Casos mínimos

#### Request ID

- adiciona `X-Request-ID` à resposta;
- disponibiliza o mesmo valor no contexto;
- gera valores diferentes para requisições diferentes.

#### Access log

- preserva o status e o corpo do handler;
- registra método, path, status e bytes;
- registra `200` quando o handler usa apenas `Write`;
- mantém o primeiro status quando `WriteHeader` é chamado mais de uma vez.

#### Recovery

- transforma um panic em `500`;
- não expõe stack trace ao cliente;
- registra o panic e a stack trace;
- não modifica uma resposta normal quando não existe panic.

### O que não testar

Evite testes dependentes de detalhes irrelevantes:

- não compare a duração com um número exato;
- não dependa da ordem textual dos campos de um log JSON;
- não teste funções internas de uma biblioteca de terceiros;
- não teste apenas que “a função foi chamada” quando é possível verificar o resultado observado pelo cliente.

Se um teste altera o logger global com `slog.SetDefault`, restaure-o com `t.Cleanup` e não execute esse teste em paralelo.

## 7. Checklist para uma API nova

### API pequena ou protótipo

- configurar `http.Server` com timeouts básicos;
- Recovery;
- algum access log, próprio ou da infraestrutura;
- Request ID se houver mais de um ponto relevante de log;
- limites de corpo nas rotas que recebem dados.

### API em produção

- Request ID;
- access log estruturado;
- Recovery;
- timeouts do servidor e das dependências;
- autenticação e autorização onde necessário;
- rate limiting nos endpoints suscetíveis a abuso;
- limite de tamanho de request;
- CORS, caso existam chamadas cross-origin pelo navegador;
- métricas e health checks;
- testes dos middlewares próprios;
- confirmação de como proxy e aplicação propagam IP e Request ID.

## 8. Regra prática

Não comece perguntando “quais middlewares toda API deve possuir?”. Para cada um, pergunte:

1. Qual problema ele resolve?
2. O que acontece concretamente se eu não usá-lo?
3. A aplicação, uma biblioteca ou a infraestrutura já resolve esse problema?
4. Preciso do comportamento em todas as rotas ou apenas em um grupo?
5. Como comprovo esse comportamento com um teste?

Para a maioria das APIs em produção, Request ID, access log e Recovery formam uma base pequena e útil. Os demais devem ser adicionados conforme as rotas e os riscos reais do sistema.
