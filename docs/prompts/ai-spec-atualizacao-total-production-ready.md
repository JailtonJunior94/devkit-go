# Prompt Enriquecido - ai-spec para atualizacao total do projeto

## Objetivo deste arquivo

Este arquivo nao executa mudancas. Ele apenas registra um prompt enriquecido, em pt-BR, para ser usado depois por um agente que devera conduzir a atualizacao ampla deste repositorio com `ai-spec`, de forma eficiente, robusta e production-ready.

## Comparativo rapido

| Item | Prompt original | Prompt enriquecido |
|---|---|---|
| Escopo | "usar ai-spec e atualizar tudo" | Delimita fases, entregaveis, criterios de aceitacao e fontes obrigatorias |
| Governanca | implicita | obriga leitura de `AGENTS.md`, uso economico de contexto e anti-alucinacao |
| AGENTS.md / CLAUDE.md | pedido generico para atualizar | exige pesquisa em documentacao oficial atual, com prioridade e rastreabilidade |
| Go | regra adicional solta | vira regra explicita de execucao: carregar `go-implementation` sob demanda e nunca escrever comentarios em codigo Go |
| ai-spec | mencionado sem fluxo | define preflight, descoberta da versao, comandos compativeis e fluxo PRD -> techspec -> tasks -> execucao |
| Seguranca contra erro | baixa | bloqueia suposicoes, proibe degradacao silenciosa e exige parar quando houver ambiguidade relevante |

## Prompt original

```text
Usar ai-spec e atualizar TUDO deste projeto, de forma eficiente, robusta e production-ready.

Ao terminar, o segundo passo e atualizar AGENTS.md e CLAUDE.md, buscando em 2026 as melhores e mais recomendadas praticas em documentacoes oficiais para uso melhor, mandatorio, eficiente e economico, reduzindo alucinacoes e implementacoes erradas.

Outra regra mandatoria: em todo codigo Go implementado, e obrigatorio carregar go-implementation e seus exemplos de forma sob demanda economica; em hipotese nenhuma escrever comentarios em codigo Go.
```

## Prompt enriquecido - versao pronta para uso

```text
Voce vai atuar neste repositorio como um agente de execucao orientado por `ai-spec`. O objetivo e conduzir uma atualizacao ampla, consistente e production-ready do projeto, cobrindo governanca, especificacao, planejamento e implementacao, sem inventar contexto e sem fazer mudancas impulsivas.

## Missao principal

Usar `ai-spec` para diagnosticar o estado atual do repositorio, identificar lacunas reais, estruturar o trabalho em artefatos verificaveis e executar as mudancas necessarias com foco em:

1. robustez
2. baixo risco de regressao
3. economia de contexto
4. eliminacao de alucinacoes
5. aderencia a documentacao oficial atual
6. consistencia entre governanca, codigo, testes e docs

## Resultado esperado

Ao final, o repositorio deve ficar mais correto, mais consistente, mais previsivel para agentes e mais pronto para uso em producao, sem reescritas desnecessarias e sem overengineering.

## Ordem obrigatoria de execucao

1. Ler `AGENTS.md` como fonte canonica do repositorio.
2. Validar a versao do Go em `go.mod` antes de qualquer decisao tecnica de linguagem ou dependencia.
3. Fazer preflight do `ai-spec` e descobrir os comandos realmente disponiveis na versao instalada.
4. Inventariar o repositorio antes de editar qualquer arquivo.
5. Estruturar ou atualizar artefatos `ai-spec` necessarios.
6. Atualizar `AGENTS.md` e `CLAUDE.md` com base em documentacao oficial atual e evidencias do proprio repositorio.
7. So depois executar mudancas de codigo, em fatias pequenas, verificaveis e com validacao proporcional ao risco.

## Regras mandatarias de contexto e governanca

- Nao assumir contexto ausente.
- Nao inventar requisitos.
- Nao mudar comportamento publico sem explicitar.
- Nao degradar silenciosamente quando uma verificacao obrigatoria falhar.
- Preferir a menor mudanca segura que resolva a causa raiz.
- Preservar arquitetura, estilo e fronteiras existentes antes de propor expansoes.
- Carregar apenas o contexto minimo necessario para cada etapa.
- Se a tarefa afetar codigo Go, carregar obrigatoriamente:
  - `AGENTS.md`
  - `.agents/skills/agent-governance/SKILL.md`
  - `.agents/skills/go-implementation/SKILL.md`
- Consultar exemplos de `go-implementation` apenas sob demanda, de forma economica.
- Em hipotese nenhuma escrever comentarios em codigo Go novo ou alterado.

## Regras especificas para `AGENTS.md` e `CLAUDE.md`

Voce deve atualizar `AGENTS.md` e `CLAUDE.md` como prioridade de governanca, usando somente documentacao oficial e atual, disponivel em 2026, mais evidencias observadas neste repositorio.

### Fontes oficiais obrigatorias

Priorize estas fontes oficiais:

1. Anthropic Claude Code docs sobre `CLAUDE.md`, memoria, escopo, concisao, imports e regras:
   - https://code.claude.com/docs/en/memory
2. GitHub Docs sobre repository custom instructions:
   - https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions
3. GitHub Docs sobre custom agents no Copilot CLI:
   - https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-custom-agents-for-cli
4. Especificacao aberta de `AGENTS.md`:
   - https://agents.md
   - https://github.com/agentsmd/agents.md
5. Ajuda e comportamento real do binario instalado de `ai-spec`:
   - `ai-spec --help`
   - `ai-spec skills --help`
   - comandos disponiveis na versao instalada

### Regras de atualizacao desses arquivos

- `AGENTS.md` deve permanecer como fonte canonica, neutra entre ferramentas e focada em:
  - fluxo operacional
  - arquitetura
  - validacao
  - restricoes
  - economia de contexto
  - regras por linguagem
- `CLAUDE.md` deve seguir as recomendacoes oficiais atuais do Claude Code.
- Se for mais consistente com a documentacao oficial, prefira fazer `CLAUDE.md` importar `@AGENTS.md` e manter apenas deltas especificos do Claude, evitando duplicacao desnecessaria.
- Elimine contradicoes, redundancias e instrucoes longas demais.
- Mantenha ambos objetivos, curtos, verificaveis e com alta aderencia.
- Toda instrucao adicionada deve ser rastreavel a:
  1. documentacao oficial atual
  2. evidencia do repositorio
  3. regra explicita solicitada neste prompt

## Fluxo obrigatorio com `ai-spec`

### Etapa 0 - preflight real

Antes de qualquer alteracao:

1. Confirmar se `ai-spec` existe no PATH.
2. Descobrir a sintaxe real da versao instalada.
3. Nao presumir que comandos historicos continuam validos.
4. Se `ai-spec skills --verify` nao existir, descobrir e usar o comando compativel disponivel.
5. Se houver drift de spec, tratar isso explicitamente e nao seguir em modo degradado.

### Etapa 1 - descoberta do repositorio

Fazer uma leitura ampla e objetiva de:

- `AGENTS.md`
- `CLAUDE.md`
- `go.mod`
- `.ai_spec_harness.json`
- `.github/`
- `.agents/skills/`
- `.specs/`
- `tasks/`
- `README*`, docs, workflows, scripts e arquivos de configuracao relevantes

Produzir:

1. inventario do estado atual
2. mapa de riscos
3. lacunas reais
4. inconsistencias de governanca
5. problemas de especificacao, execucao ou validacao

### Etapa 2 - planejamento com fatias eficientes

Se o trabalho for grande, quebrar em fatias pequenas, eficientes e verificaveis.

Cada fatia deve:

- ter objetivo claro
- ser validavel de forma independente
- reduzir risco de falso positivo
- manter rastreabilidade para PRD, techspec e tasks
- evitar lotes grandes e difusos

### Etapa 3 - artefatos `ai-spec`

Criar ou atualizar o minimo necessario de artefatos para que a execucao fique governada:

- `prd.md`
- `techspec.md`
- `tasks.md`
- arquivos de tarefa quando necessario

Regras:

- nao criar artefatos cosmeticos
- usar hashes e verificacoes suportadas pela ferramenta instalada
- manter compatibilidade com o fluxo real deste repositorio
- explicitar bloqueios quando faltar contexto

### Etapa 4 - governanca primeiro

Antes de mexer em codigo de forma ampla:

1. atualizar `AGENTS.md`
2. atualizar `CLAUDE.md`
3. reduzir redundancia
4. reforcar economia de contexto
5. reforcar regras anti-alucinacao
6. reforcar instrucoes obrigatorias para Go:
   - carregar `go-implementation` quando houver mudanca Go
   - consultar exemplos sob demanda
   - nao escrever comentarios em codigo Go

### Etapa 5 - execucao tecnica

Executar apenas mudancas justificadas pelo inventario e pelos artefatos `ai-spec`.

Regras de execucao:

- trabalhar em incrementos pequenos
- validar cada incremento com comandos proporcionais ao risco
- nao "modernizar tudo" sem motivacao concreta
- nao introduzir dependencia, camada ou abstracao sem necessidade demonstrada
- nao duplicar regra entre arquivos de governanca quando um import ou referencia resolver melhor

## Criterios de aceitacao obrigatorios

Considere a tarefa concluida apenas se todos os itens abaixo forem verdadeiros:

1. O fluxo usado e compativel com a versao real do `ai-spec` instalada.
2. O escopo foi quebrado em fatias verificaveis quando necessario.
3. `AGENTS.md` foi revisado com base em documentacao oficial atual e no contexto real do repositorio.
4. `CLAUDE.md` foi revisado com base em documentacao oficial atual do Claude Code e sem redundancia desnecessaria.
5. As instrucoes desses arquivos ficaram mais curtas, mais claras, menos contraditorias e mais acionaveis.
6. O repositorio nao recebeu regras inventadas, vagas ou impossiveis de verificar.
7. Toda alteracao Go respeitou a obrigatoriedade de `go-implementation` e a proibicao de comentarios em codigo Go.
8. Nao houve degradacao silenciosa quando uma ferramenta, comando ou evidencia nao estava disponivel.
9. O resultado final esta rastreavel: cada mudanca relevante aponta para evidencia do repositorio ou fonte oficial.

## Formato de saida obrigatorio

Responda sempre nesta ordem:

1. **Preflight**
   - versao e comandos reais do `ai-spec`
   - riscos ou bloqueios
2. **Descoberta**
   - resumo do estado atual
   - lacunas prioritarias
3. **Fontes oficiais usadas**
   - lista com URL e decisao derivada
4. **Plano em fatias**
   - etapas pequenas e verificaveis
5. **Mudancas propostas em `AGENTS.md` e `CLAUDE.md`**
   - antes/depois resumido
   - justificativa objetiva
6. **Execucao**
   - o que foi alterado
7. **Validacao**
   - comandos rodados
   - resultado objetivo
8. **Pendencias**
   - somente o que realmente bloqueou

## Regras anti-alucinacao

- Se uma pratica nao estiver confirmada por documentacao oficial atual ou por evidencia local, nao trate como obrigatoria.
- Se duas fontes conflitarem, explicite o conflito e escolha a alternativa mais especifica e oficial para a ferramenta afetada.
- Se o repositorio ja tiver uma convencao valida, preserve-a a menos que exista motivo concreto para ajuste.
- Se uma decisao impactar comportamento, arquitetura ou escopo, pare e peça confirmacao.
- Nao responda com generalidades. Sempre ligue decisao a arquivo, comando, doc oficial ou comportamento observado.

## Observacoes especificas deste repositorio

- O projeto usa Go e `go.mod` declara `go 1.26.2`.
- `AGENTS.md` ja define `AGENTS.md` como fonte canonica.
- O repositorio ja possui skills e estrutura `ai-spec` instaladas.
- Ha historico local indicando que a sintaxe de verificacao de skills do `ai-spec` pode variar por versao; confirme a CLI real antes de automatizar o fluxo.

Se faltar contexto relevante, pare no ponto certo, liste exatamente o que falta e nao improvise.
```

## Justificativa das adicoes

1. **Fluxo em fases**: transforma "atualizar tudo" em uma sequencia segura, auditavel e com menos risco de erro.
2. **Fontes oficiais nomeadas**: reduz alucinacao e ajuda o agente a justificar mudancas em `AGENTS.md` e `CLAUDE.md`.
3. **Preflight real do `ai-spec`**: evita assumir comandos incorretos para a versao instalada.
4. **Criterios de aceitacao mensuraveis**: impede resposta vaga ou "parece pronto".
5. **Regras anti-alucinacao**: forcam rastreabilidade e bloqueiam invencao de contexto.
6. **Regra Go explicita**: incorpora sua exigencia de carregar `go-implementation` sob demanda e de nunca escrever comentarios em codigo Go.
7. **Economia de contexto**: reforca leitura minima necessaria, reduz ruido e melhora aderencia.

## Variantes validas

### Variante recomendada

Use o prompt acima como esta. Ele prioriza governanca primeiro, depois execucao tecnica.

### Variante conservadora

Se quiser ainda menos risco, acrescente esta linha logo apos "Ordem obrigatoria de execucao":

```text
Antes de qualquer mudanca fora de governanca, apresente o inventario, o plano em fatias e aguarde aprovacao humana.
```
