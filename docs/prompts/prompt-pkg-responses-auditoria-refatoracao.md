# Prompt para auditoria e plano de refatoracao de `pkg/responses`

## Prompt original

> Eu quero que analise o pkg/responses, e veja se realmente e um pacote robusto, eficiente, para importar em projetos pequenos, medios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentarios e use de forma obrigatoria a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.
>
> Crie um plano de refatoracao, reduza se possivel a quantidade de codigo, deixe bem feito, utilizando golike, padroes de projeto e algo portavel em outros.
>
> NAO CRIE NADA, APENAS CRIE/ENRIQUECA O PROMPT.

## Prompt enriquecido

```text
Objetivo
Voce vai auditar o pacote `pkg/responses` deste repositorio Go e produzir apenas uma analise tecnica rigorosa + um plano de refatoracao. Nao implemente codigo, nao edite arquivos, nao abra PR, nao gere patch e nao faca mudancas no workspace. O foco e responder, com evidencia real e sem falso positivo, se `pkg/responses` e um pacote robusto, eficiente, portavel e realmente pronto para uso em projetos pequenos, medios e grandes.

Contrato obrigatorio de carga
1. Ler `AGENTS.md` antes de qualquer analise.
2. Respeitar a governanca do repositorio como fonte canonica.
3. Tratar esta tarefa como analise/refatoracao de codigo Go e carregar obrigatoriamente:
   - `.agents/skills/agent-governance/SKILL.md`
   - `.agents/skills/go-implementation/SKILL.md`
4. Ao carregar `go-implementation`, cumprir o baseline minimo da skill:
   - ler `go.mod`;
   - executar `bash scripts/verify-go-mod.sh`;
   - carregar referencias e exemplos apenas sob demanda, com foco explicito em economia de contexto.
5. Considerar como requisito nao negociavel para qualquer plano futuro de refatoracao: remover TODOS os comentarios do codigo Go alterado, em linha com a governanca local e com a exigencia deste prompt.

Contexto minimo conhecido
- Repositorio Go monolitico.
- Versao declarada em `go.mod`: Go 1.26.2.
- O pacote `pkg/responses` possui, no minimo, 2 arquivos: `responses.go` e `responses_test.go`.
- O pacote exporta, no minimo, 3 funcoes publicas: `JSON`, `Error` e `ErrorWithDetails`.
- A implementacao atual usa somente stdlib relevante para resposta HTTP/JSON (`net/http`, `encoding/json`, `log`).
- Existem testes unitarios e benchmarks no pacote, incluindo cenarios com `httptest`, payloads complexos, dados nao serializaveis e uso concorrente.
- O objetivo nao e avaliar marketing do pacote; e validar readiness real para producao e reuso em diferentes portes de sistema.

Escopo da auditoria
Analise `pkg/responses` de ponta a ponta e determine se ele e apto para ser importado por:
- projetos pequenos;
- projetos medios;
- projetos grandes e com maior carga operacional.

Voce deve avaliar, no minimo:
1. API publica, ergonomia e clareza de uso.
2. Corretude do contrato HTTP/JSON: headers, status code, corpo, comportamento em erro de serializacao e compatibilidade com `http.ResponseWriter`.
3. Robustez operacional: comportamento sob falha, observabilidade, logging, efeitos colaterais e risco de resposta parcial/corrompida.
4. Testabilidade, qualidade dos testes, valor real dos benchmarks e confiabilidade dos cenarios concorrentes.
5. Performance, alocacoes evitaveis, duplicacao, boilerplate e oportunidades reais de reducao de codigo.
6. Portabilidade do design para outros projetos, frameworks e camadas HTTP.
7. Aderencia a design Go-like, simplicidade, coesao e ausencia de abstracoes desnecessarias.
8. Riscos de manutencao, regressao, concorrencia e uso em producao.

Regra anti-falso-positivo
Nao conclua que o pacote e `production-ready`, `production-proof`, `robusto` ou `eficiente` com base em impressao geral, no fato de ser pequeno, ou apenas porque possui testes. Cada afirmacao precisa estar ancorada em evidencia concreta observada no codigo, nos testes, nos contratos publicos e nas superficies operacionais.

Se faltar evidencia suficiente, declare explicitamente:
- o que esta bom;
- o que esta inconclusivo;
- o que impede cravar readiness de producao;
- qual risco real ainda existe.

Nao suavize diagnostico. Nao invente garantias. Nao trate cobertura parcial, benchmark superficial ou ausencia de panic como prova de robustez total.

Diretrizes de analise
1. Comece pelo estado real do codigo local, nao por suposicoes.
2. Priorize causa raiz e impactos sistemicos, nao apenas detalhes cosmeticos.
3. Diferencie claramente:
   - problema confirmado;
   - risco plausivel;
   - melhoria opcional.
4. Se identificar comentarios no codigo Go, trate sua remocao como requisito obrigatorio do plano de refatoracao, mas nao altere nada agora.
5. Prefira solucoes Go-like, portaveis, com baixa complexidade acidental e alinhadas a boas praticas de design.
6. So proponha pattern quando ele reduzir acoplamento, branching recorrente, duplicacao ou custo operacional. Nao introduza pattern por status.
7. Se houver chance de reduzir codigo, proponha simplificacao apenas quando nao comprometer legibilidade, extensibilidade, compatibilidade ou seguranca operacional.
8. Verifique se o pacote realmente merece existir como pacote reutilizavel independente, ou se a API atual e pequena/direta demais e poderia ser absorvida de forma mais simples por consumidores sem perda de portabilidade.
9. Considere explicitamente trade-offs de um helper HTTP extremamente pequeno: conveniencia vs acoplamento, simplicidade vs extensibilidade, stdlib pura vs integracao com observabilidade/erros estruturados.

Perguntas obrigatorias a responder
1. `pkg/responses` e de fato um pacote robusto e production-ready, ou ainda esta abaixo desse nivel?
2. O design atual e suficientemente portavel para projetos pequenos, medios e grandes?
3. Existe duplicacao, complexidade acidental ou codigo que pode ser reduzido com seguranca?
4. A estrategia atual de erro/logging evita falso positivo de robustez?
5. O pacote deve continuar existindo como esta, ser simplificado, ou ser redesenhado em partes?

Formato obrigatorio de saida
Responda em Markdown, em pt-BR, com as secoes abaixo:

# Veredito
- Classifique o pacote como uma das opcoes:
  - `Aprovado para producao`
  - `Aprovado com restricoes`
  - `Nao aprovado para producao`
- Traga um resumo executivo curto e direto.

# Matriz de avaliacao
Monte uma tabela com estas colunas:
`Dimensao | Nota (0-5) | Evidencias concretas | Riscos | Impacto por porte (pequeno/medio/grande)`

Avalie pelo menos:
- API e ergonomia
- Corretude HTTP/JSON
- Robustez operacional
- Performance e eficiencia
- Testabilidade
- Portabilidade
- Manutenibilidade
- Prontidao para producao

# Evidencias objetivas
Liste achados tecnicos concretos, sempre apontando o componente ou area analisada.

# Lacunas que impedem falso positivo
Liste tudo que impede cravar qualidade ou readiness quando a evidencia for insuficiente.

# Plano de refatoracao
Monte um plano priorizado em fases, com foco em menor mudanca segura e maior retorno:
`Fase | Objetivo | Mudancas principais | Ganho esperado | Risco | Esforco | Dependencias`

O plano deve:
- reduzir codigo quando isso for seguro;
- melhorar robustez e eficiencia;
- remover comentarios como requisito obrigatorio do eventual diff;
- preservar portabilidade;
- evitar abstracoes desnecessarias;
- privilegiar design Go-like e patterns somente quando houver ganho claro.

# Refatoracoes concretas recomendadas
Liste as refatoracoes mais importantes, com antes/depois conceitual, sem escrever codigo.

# Criticos para production-ready
Liste os criterios objetivos que ainda precisam ser atendidos para chamar o pacote de production-ready sem falso positivo.

# Ordem sugerida de execucao
Forneca uma sequencia pratica e incremental para executar o plano com menor risco.

Restricoes finais
- Nao criar arquivos.
- Nao modificar codigo.
- Nao propor reescrita ampla sem justificativa tecnica forte.
- Nao fazer elogio generico sem evidencia.
- Nao responder de forma superficial.
- Nao omitir trade-offs.
- Nao usar "tem teste" como argumento suficiente de qualidade.
```

## Melhorias aplicadas

| Ajuste | Justificativa |
|---|---|
| Carga obrigatoria de `AGENTS.md`, `agent-governance` e `go-implementation` | Alinha o prompt com a governanca real e com a exigencia explicita do usuario. |
| Baseline minimo da `go-implementation` incluido no prompt | Garante uso correto da skill sem carregar contexto desnecessario fora de demanda real. |
| Contexto tecnico real de `pkg/responses` | Reduz ambiguidade ao ancorar a analise em superficie publica, testes e dependencias observadas. |
| Regra anti-falso-positivo reforcada | Forca conclusoes baseadas em evidencia concreta, nao em impressao ou tamanho do pacote. |
| Pergunta explicita se o pacote deve mesmo existir como pacote reutilizavel | Endereca a robustez/portabilidade sem presumir que mais abstracao e sempre melhor. |
| Formato de saida prescrito | Aumenta previsibilidade, comparabilidade e valor pratico do diagnostico. |
| Plano de refatoracao por fases e com trade-offs | Direciona um resultado acionavel sem implementar nada agora. |
