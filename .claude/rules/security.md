# Segurança

- Rule ID: R-SEC-001
- Severidade: hard
- Escopo: Todo código-fonte, testes, configuração, logs e handlers HTTP.

## Objetivo
Definir controles de segurança baseline para implementação e comportamento do agente.

## Requisitos

### Validação de Input
- Todo input externo deve ser validado na fronteira (handler HTTP, consumer, input de job).
- Erros de validação devem retornar mensagens seguras voltadas ao cliente.

### Autenticação e Autorização
- Rotas protegidas devem exigir middleware de autenticação.
- Verificações de autorização devem ser explícitas para propriedade de recurso ou acesso baseado em role.
- Nunca confiar em identificadores de usuário fornecidos pelo cliente sem verificação server-side.

### Segredos e Credenciais
- Segredos não devem estar hardcoded no código-fonte.
- Segredos não devem ser logados, rastreados ou escritos em mensagens de erro.
- Usar variáveis de ambiente ou provedores dedicados de segredos.

### Segurança SQL e de Queries
- Usar apenas queries parametrizadas.
- Nunca concatenar input do usuário em SQL.

### Proteção de Dados Sensíveis
- Nunca expor stack traces internos ou erros de infraestrutura para clientes.
- Logs e traces devem evitar PII e valores de segredos.

### Segurança de Dependências e Supply Chain
- Preferir dependências estáveis e mantidas.
- Fixar versões onde aplicável e evitar fontes não verificadas.

## Proibido
- Chaves de API, tokens ou senhas hardcoded.
- Logar credenciais, dados pessoais brutos ou payloads completos de autenticação.
- Autorização apenas por confiança do lado do cliente.
- Fallback silencioso que enfraqueça controles de segurança.
