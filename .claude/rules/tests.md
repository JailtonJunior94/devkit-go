# Testes

- Rule ID: R-TEST-001
- Severidade: hard para correção/isolamento, guideline para estilo.
- Escopo: Todos os arquivos `*_test.go`.

## Objetivo
Garantir qualidade, estrutura e confiabilidade consistentes nos testes.

## Requisitos

### Framework (guideline)
- Usar testify (`suite`, `require`, `mock`) onde melhorar a consistência.
- Testes unitários no nível de domínio podem usar `testing` padrão + `require`.

### Isolamento e Determinismo (hard)
- Testes não devem compartilhar estado mutável.
- Setup por teste deve resetar dependências.
- Testes não devem depender da ordem de execução.

### Estrutura (guideline)
- Preferir AAA (Arrange, Act, Assert).
- Usar cenários table-driven para lógica com múltiplas variações.
- Nomes de teste devem descrever o comportamento esperado em inglês.

### Mocks e Fakes (guideline)
- Usar mocks gerados ao mockar interfaces de repositório/serviço.
- Expectativas de mock devem definir contagem de chamadas quando relevante.

### Cobertura e Execução (hard)
- Use cases e comportamento de domínio devem ser cobertos por testes.
- Gate de execução padrão: `make test` (ver `tooling.md`).

## Proibido
- Testes sem asserções.
- Chamadas reais de rede/banco em testes unitários puros.
- Estado mutável compartilhado entre casos de teste.
- Suposições de timing não determinísticas sem controle explícito.
