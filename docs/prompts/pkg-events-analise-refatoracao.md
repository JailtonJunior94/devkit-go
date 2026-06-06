# Prompt enriquecido - analise e plano de refatoracao para `pkg/events`

## Prompt original

```text
Eu quero que analise o pkg/events, e veja se realmente e um pacote robusto, eficiente, para importar em projetos pequenos, medios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentarios e use de forma obrigatoria a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.

Crie um plano de refatoracao, reduza se possivel a quantidade de codigo, deixe bem feito, utilizando golike, padroes de projeto e algo portavel em outros.

NAO CRIE NADA, APENAS CRIE/ENRIQUECA O PROMPT.
```

## Sintese do enriquecimento

| Aspecto | Original | Enriquecido |
| --- | --- | --- |
| Escopo | Analise ampla, sem delimitacao operacional | Delimita que o alvo e apenas `pkg/events` e que o entregavel e somente diagnostico + plano |
| Governanca | Cita `go-implementation`, mas sem ordem de carga completa | Exige leitura de `AGENTS.md` e carga obrigatoria de `go-implementation`, com exemplos apenas sob demanda |
| Saida | Nao define estrutura | Define formato markdown com secoes, scorecard, achados, riscos e plano incremental |
| Anti-falso-positivo | Menciona objetivo, sem criterio verificavel | Exige evidencia por arquivo/linha, separacao entre fato, risco e oportunidade, e proibicao de inferencias sem prova |
| Refatoracao | Pede plano generico | Exige plano portavel, golike, com reducao de codigo quando fizer sentido, impacto, risco e validacao por etapa |
| Restricoes | Diz para nao criar nada | Deixa explicito que nao deve alterar codigo, criar patch, nem executar refatoracao |

## Prompt enriquecido

```text
Quero uma analise tecnica rigorosa do pacote `pkg/events` deste repositorio Go para determinar se ele esta realmente pronto para uso em producao e se e suficientemente robusto, eficiente e portavel para ser importado em projetos pequenos, medios e grandes, sem falso positivo.

Antes de qualquer analise:
1. Leia `AGENTS.md` e siga a governanca do repositorio.
2. Carregue obrigatoriamente a skill `.agents/skills/go-implementation/SKILL.md`.
3. Consulte exemplos e referencias da skill `go-implementation` somente sob demanda, com foco em economia de contexto.
4. Nao implemente nada, nao altere arquivos, nao gere patch e nao crie codigo novo. O entregavel e apenas analise + plano de refatoracao.

Objetivo:
- Avaliar se `pkg/events` e um pacote production-ready / production-proof de verdade.
- Identificar apenas problemas reais, riscos reais e oportunidades reais, sempre com evidencia.
- Evitar completamente falso positivo: nao invente defeitos, nao extrapole sem prova e nao marque como problema aquilo que for apenas preferencia de estilo.
- Propor um plano de refatoracao enxuto, golike, portavel e aplicavel em outros projetos, reduzindo codigo quando isso melhorar clareza, manutencao ou desempenho.

Escopo obrigatorio da avaliacao:
- API publica e ergonomia de uso.
- Concorrencia, sincronizacao, seguranca de race conditions e previsibilidade.
- Custos de alocacao, copias, lock contention e impacto de performance.
- Semantica de erros, contratos publicos e comportamento em edge cases.
- Escalabilidade para poucos e muitos handlers/eventos.
- Acoplamento, portabilidade e facilidade de importar o pacote em outros projetos.
- Qualidade e cobertura dos testes, exemplos e benchmarks existentes.
- Coerencia com estilo Go idiomatico e desenho reutilizavel.
- Necessidade de remover TODOS os comentarios Go do escopo analisado em uma futura refatoracao, mantendo clareza sem comentarios.

Regras de avaliacao:
- Cada achado deve citar evidencias concretas com `arquivo:linha`.
- Diferencie explicitamente:
  - `Fato comprovado`
  - `Risco plausivel`
  - `Oportunidade de melhoria`
- Se algo nao puder ser comprovado a partir do codigo, testes ou benchmarks atuais, diga claramente que nao ha evidencia suficiente.
- Nao use termos vagos como "talvez", "parece ruim" ou "provavelmente problematica" sem justificativa objetiva.
- Se o pacote ja estiver adequado em algum ponto, registre isso de forma positiva e objetiva.
- Considere portabilidade real: evitar solucoes que prendam o pacote a framework, infraestrutura ou contexto especifico do repositorio.

Formato obrigatorio da saida em markdown:
1. `Resumo executivo`
   - Veredito: `Aprovado`, `Aprovado com ressalvas` ou `Nao aprovado`
   - Resposta objetiva: o pacote esta pronto para producao? por que?
2. `Scorecard`
   - Robustez
   - Eficiencia
   - Escalabilidade
   - Portabilidade
   - Clareza da API
   - Testabilidade
   Para cada item, use nota de 0 a 5 com justificativa curta.
3. `Inventario do pacote`
   - Arquivos relevantes
   - Responsabilidades atuais
   - Contratos publicos
4. `Achados com evidencia`
   Para cada achado, informe:
   - Severidade: `critico`, `alto`, `medio`, `baixo` ou `oportunidade`
   - Classificacao: `Fato comprovado`, `Risco plausivel` ou `Oportunidade de melhoria`
   - Evidencia com `arquivo:linha`
   - Impacto pratico
   - Se gera ou nao risco de falso positivo
5. `Analise de production-readiness`
   - O que sustenta uso em producao
   - O que bloqueia uso em producao
   - O que e aceitavel apenas em cenarios pequenos
   - O que precisaria mudar para suportar cenarios medios/grandes
6. `Plano de refatoracao incremental`
   Para cada etapa, informe:
   - Objetivo
   - Mudanca proposta
   - Beneficio esperado
   - Risco de regressao
   - Validacao recomendada
   - Estimativa qualitativa de reducao ou simplificacao de codigo
7. `Quick wins`
   - Liste apenas melhorias pequenas, seguras e com alto retorno
8. `Nao fazer`
   - Liste mudancas tentadoras, mas desnecessarias, superengenheiradas ou sem evidencia

Criterios de aceitacao da sua resposta:
- A resposta deve cobrir todo o pacote `pkg/events`, incluindo implementacao, testes e exemplos.
- Nenhum achado pode existir sem evidencia concreta.
- O plano precisa ser executavel, incremental e portavel para outros projetos Go.
- O plano deve priorizar o menor conjunto de mudancas que resolva a causa raiz dos problemas.
- Se houver chance de reduzir codigo sem perder clareza, robustez ou performance, isso deve ser destacado.
- Se comentarios atuais forem desnecessarios, isso deve aparecer no plano como item explicito de limpeza.
- Nao proponha abstrações, camadas ou padroes de projeto sem necessidade demonstrada.
- Nao proponha reescrita total se refatoracoes menores resolverem.

Importante:
- Quero uma avaliacao de engenharia senior, conservadora contra falso positivo.
- Prefira precisao a quantidade de observacoes.
- Se concluir que o pacote ja esta bom em partes importantes, diga isso.
- Se concluir que nao esta production-ready, explique exatamente os bloqueadores.
```
