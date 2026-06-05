<!-- governance-schema: 1.0.0 -->
# Regras para Agentes de IA

Este arquivo e a fonte canonica de governanca para agentes neste repositorio. Use-o para trabalhar com codigo, configuracao, documentacao e validacao sem inventar contexto.

## Contexto do Projeto

- Arquitetura detectada: monolito Go.
- Frameworks detectados: Fiber e gRPC.
- Versao Go: verificar sempre em `go.mod` antes de usar APIs de linguagem, runtime ou dependencias.
- Governanca instalada via `ai-spec-harness 0.27.1`; neste ambiente o binario no PATH e `ai-spec`.
- Nao usar o manifesto local como versao efetiva da CLI quando ele divergir de `ai-spec version`.

## Fluxo Operacional

1. Ler este arquivo antes de analisar ou alterar arquivos.
2. Inspecionar o contexto real com comandos locais antes de propor mudancas.
3. Preservar arquitetura, estilo, nomes e fronteiras existentes.
4. Preferir a menor mudanca segura que resolva a causa raiz.
5. Nao introduzir camadas, dependencias ou abstracoes sem necessidade demonstrada.
6. Atualizar ou adicionar testes quando houver mudanca de comportamento.
7. Rodar validacoes proporcionais ao risco.
8. Registrar bloqueios, falhas de ferramenta e suposicoes explicitamente.

## Arquitetura

- Transporte e adapters dependem de casos de uso ou servicos explicitos, nunca o inverso.
- Dominio nao conhece HTTP, banco, filas, serializacao, drivers ou detalhes de infraestrutura.
- Infraestrutura implementa contratos consumidos pela aplicacao, mantendo dependencia para dentro.
- Crescer a estrutura apenas quando o codigo atual nao comportar a mudanca com clareza.
- Evitar helpers transversais que escondam regra de negocio ou IO.

## Skills Obrigatorias

Para tarefas que alteram codigo ou governanca:

- Carregar `.agents/skills/agent-governance/SKILL.md`.

Para tarefas que alteram codigo Go:

- Carregar `.agents/skills/go-implementation/SKILL.md`.
- Consultar referencias e exemplos de Go apenas sob demanda.
- Nao escrever comentarios em codigo Go novo ou alterado.

Para tarefas especificas:

- Bug fix com remediacao e teste de regressao: carregar `.agents/skills/bugfix/SKILL.md`.
- Review de codigo: carregar `.agents/skills/review/SKILL.md`.
- Refatoracao incremental: carregar `.agents/skills/refactor/SKILL.md`.
- Refatoracao Go guiada por object calisthenics: carregar `.agents/skills/object-calisthenics-go/SKILL.md`.

## Economia de Contexto

Classificar a complexidade antes de carregar referencias:

| Complexidade | Criterio | Contexto |
|---|---|---|
| `trivial` | Rename, typo, import ou formatacao sem comportamento | Apenas este arquivo |
| `standard` | Bug fix, novo metodo ou refactor local | Este arquivo + TL;DR das referencias afetadas |
| `complex` | Nova feature, interface publica, migracao ou mudanca transversal | Este arquivo + referencias completas aplicaveis |

- Quando uma referencia tiver bloco `<!-- TL;DR -->`, preferir esse bloco em tarefas `standard`.
- Override explicito `--complexity=<nivel>` prevalece sobre a classificacao automatica.
- Se mais de uma skill puder ser usada, carregar o conjunto minimo que cobre a tarefa.

## ai-spec-harness 0.27.1

- Confirmar a CLI real com `ai-spec version` e `ai-spec --help`; o output deve identificar `ai-spec-harness 0.27.1`.
- Usar `ai-spec inspect .` e `ai-spec doctor .` para diagnostico da instalacao.
- Usar `ai-spec verify . --tools all --langs go --by-cli` para verificar skills instaladas.
- Usar `ai-spec skills check` somente para checagem de versoes de skills externas quando aplicavel.
- Usar `ai-spec check-spec-drift <tasks.md|diretorio-prd>` para drift entre `prd.md`, `techspec.md` e `tasks.md`.
- Nao assumir comandos historicos sem confirmar na ajuda da versao instalada.
- Nao tentar invocar `ai-spec-harness` diretamente se esse nome nao existir no PATH; usar `ai-spec`, que aponta para `/opt/homebrew/Cellar/ai-spec/0.27.1/bin/ai-spec`.
- Nao seguir em modo degradado quando `verify`, `doctor` ou drift de spec falharem sem registrar a falha.

## Ferramentas

- Claude Code: `CLAUDE.md` deve importar este arquivo e conter apenas deltas especificos do Claude.
- Codex: este arquivo e lido como instrucao de sessao; `.codex/config.toml` e metadado local, nao fonte canonica.
- Gemini CLI: `GEMINI.md` deve apontar para este arquivo e comandos em `.gemini/commands/*.toml` devem delegar para skills canonicas.
- Copilot: `.github/copilot-instructions.md` deve permanecer curto e apontar para esta governanca.
- Skills canonicas vivem em `.agents/skills/`; copias ou symlinks em `.claude/skills/` e `.github/skills/` nao devem divergir intencionalmente.

## Validacao

Antes de concluir mudancas, escolher comandos proporcionais ao risco:

- Governanca ai-spec-harness: `ai-spec lint .`, `ai-spec doctor .`, `ai-spec inspect .`, `ai-spec verify . --tools all --langs go --by-cli`.
- Drift de especificacao: `ai-spec check-spec-drift <tasks.md|diretorio-prd>`.
- Go: `gofmt` nos arquivos alterados, testes direcionados primeiro e `go test ./...` quando proporcional.
- Lint Go: `golangci-lint run` quando disponivel e proporcional ao risco.

Nao rodar formatadores mutantes em todo o repositorio quando a tarefa nao alterou codigo da stack afetada.

## Restricoes

- Nao inventar requisitos, arquitetura, versao de linguagem, runtime ou comportamento de ferramenta.
- Nao alterar comportamento publico sem explicitar a alteracao.
- Nao reverter mudancas existentes do usuario sem pedido explicito.
- Nao copiar exemplos sem adaptar ao contexto real.
- Nao usar documentacao nao oficial como fonte obrigatoria quando houver fonte oficial aplicavel.

## Controle de Profundidade

Skills que invocam outras skills devem verificar `scripts/lib/check-invocation-depth.sh` ou `.agents/lib/check-invocation-depth.sh`.

- Limite padrao: `AI_INVOCATION_MAX=2`.
- Profundidade corrente: `AI_INVOCATION_DEPTH`.
- Ao atingir o limite, parar a cadeia e registrar o bloqueio em vez de contornar.
