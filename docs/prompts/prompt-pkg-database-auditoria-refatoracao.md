# Prompt para auditoria e plano de refatoracao de `pkg/database`

## Prompt original

> Eu quero que analise o pkg/database, e veja se realmente e um pacote robusto, eficiente, para importar em projetos pequenos, medios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentarios e use de forma obrigatoria a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.
>
> Crie um plano de refatoracao, reduza se possivel a quantidade de codigo, deixe bem feito, utilizando golike, padroes de projeto e algo portavel em outros.
>
> NAO CRIE NADA, APENAS CRIE/ENRIQUECA O PROMPT.

## Prompt enriquecido

```text
Objetivo
Voce vai auditar o pacote `pkg/database` deste repositorio Go e produzir apenas uma analise tecnica rigorosa + um plano de refatoracao. Nao implemente codigo, nao edite arquivos, nao abra PR, nao gere patch e nao faca mudancas no workspace. O foco e responder, com evidencia real e sem falso positivo, se `pkg/database` e um pacote robusto, eficiente, portavel e realmente pronto para uso em projetos pequenos, medios e grandes.

Contrato obrigatorio de carga
1. Ler `AGENTS.md` antes de qualquer analise.
2. Respeitar a governanca do repositorio como fonte canonica.
3. Tratar esta tarefa como analise/refatoracao de codigo Go e carregar obrigatoriamente:
   - `.agents/skills/agent-governance/SKILL.md`
   - `.agents/skills/go-implementation/SKILL.md`
4. Carregar referencias e exemplos da skill `go-implementation` apenas sob demanda, com foco explicito em economia de contexto.
5. Considerar como requisito nao negociavel para qualquer plano futuro de refatoracao: remover TODOS os comentarios do codigo Go alterado, em linha com a governanca local.

Contexto minimo conhecido
- Repositorio Go monolitico.
- Versao declarada em `go.mod`: Go 1.26.2.
- O pacote `pkg/database` possui, no minimo, estas areas: `manager`, `migration`, `uow`, adapters para `postgres`, `mysql`, `mssql` e `cockroach`, alem de tipos base, erros, contexto, isolamento, `internal/pool` e testes variados.
- O objetivo nao e avaliar marketing do pacote; e validar readiness real para producao e reuso em diferentes portes de sistema.

Escopo da auditoria
Analise `pkg/database` de ponta a ponta e determine se ele e apto para ser importado por:
- projetos pequenos;
- projetos medios;
- projetos grandes e com maior carga operacional.

Voce deve avaliar, no minimo:
1. API publica, ergonomia e clareza de uso.
2. Acoplamento, coesao, responsabilidades e fronteiras entre pacotes.
3. Pooling, lifecycle, shutdown, contexto, cancelamento e uso seguro de recursos.
4. Transacoes, unit of work, migracoes, adapters e extensibilidade.
5. Tratamento de erros, observabilidade, testabilidade e cobertura de cenarios criticos.
6. Performance, alocacoes evitaveis, duplicacao, boilerplate e oportunidades reais de reducao de codigo.
7. Portabilidade do design para outros projetos e outros cenarios de implantacao.
8. Riscos de manutencao, regressao, concorrencia, configuracao e operacao em producao.

Regra anti-falso-positivo
Nao conclua que o pacote e `production-ready`, `production-proof`, `robusto` ou `eficiente` com base em impressao geral. Cada afirmacao precisa estar ancorada em evidencia concreta observada no codigo, nos testes, nos contratos publicos e nas superficies operacionais.

Se faltar evidencia suficiente, declare explicitamente:
- o que esta bom;
- o que esta inconclusivo;
- o que impede cravar readiness de producao;
- qual risco real ainda existe.

Nao suavize diagnostico. Nao invente garantias. Nao trate cobertura parcial como prova de robustez total.

Diretrizes de analise
1. Comece pelo estado real do codigo local, nao por suposicoes.
2. Priorize causa raiz e impactos sistemicos, nao apenas detalhes cosmeticos.
3. Diferencie claramente:
   - problema confirmado;
   - risco plausivel;
   - melhoria opcional.
4. Se identificar comentarios no codigo, trate sua remocao como requisito do plano de refatoracao, mas nao altere nada agora.
5. Prefira solucoes Go-like, portaveis, com baixa complexidade acidental e alinhadas a boas praticas de design.
6. So proponha pattern quando ele reduzir acoplamento, branching recorrente, duplicacao ou custo operacional.
7. Se houver chance de reduzir codigo, proponha simplificacao apenas quando nao comprometer legibilidade, extensibilidade ou seguranca operacional.

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
- evitar abstrações desnecessarias;
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
```

## Melhorias aplicadas

| Ajuste | Justificativa |
|---|---|
| Carga obrigatoria de `AGENTS.md`, `agent-governance` e `go-implementation` | Alinha o prompt com a governanca real do repositorio. |
| Escopo explicitado como auditoria + plano, sem implementacao | Evita que o agente execute mudancas quando o objetivo e apenas analise. |
| Regra anti-falso-positivo | Forca conclusoes baseadas em evidencia concreta, nao em impressao. |
| Formato de saida prescrito | Aumenta previsibilidade e facilita comparacao objetiva do resultado. |
| Contexto tecnico de `pkg/database` e da versao Go | Reduz ambiguidade e melhora a qualidade da leitura arquitetural. |
| Criticos de producao e impactos por porte | Foca na pergunta central: reuso seguro em projetos pequenos, medios e grandes. |
| Plano de refatoracao com fases e trade-offs | Direciona um resultado acionavel sem gerar codigo agora. |
