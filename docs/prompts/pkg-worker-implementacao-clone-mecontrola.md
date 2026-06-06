# Prompt enriquecido: implementacao de `pkg/worker` a partir do `mecontrola`

## Comparacao rapida

| Aspecto | Prompt original | Prompt enriquecido |
|---|---|---|
| Objetivo | "copiar exatamente tudo" | Escopo fechado em `pkg/worker`, com replicacao fiel do modulo de origem e limites claros de adaptacao |
| Fonte | URL solta | Fonte explicitada como referencia obrigatoria via `gh` CLI |
| Execucao | Pedido generico de implementacao | Sequencia operacional, criterios de equivalencia e regras de integracao ao repositorio atual |
| Qualidade | Termos subjetivos | Requisitos auditaveis de robustez, concorrencia, memoria, leaks, shutdown, observabilidade e readiness |
| Governanca | Parcialmente implicita | `AGENTS.md` + skill `go-implementation` obrigatoria com exemplos sob demanda |
| Comentarios Go | Regra separada | Tratado como restricao inegociavel de implementacao |
| Falso positivo | Intencao | Regra objetiva de nao declarar pronto sem evidencia concreta |

## Prompt original

```text
Eu quero implementar em: pkg o worker, e obrigatório utilizar gh cli para copiar exatamente tudo que foi implementado em: https://github.com/LimaTeixeiraTecnologia/mecontrola/tree/main/internal/platform/worker para esse projeto, o foco é projetos pequenos, médios e de grande porte, 0 alocação de memória de forma desnecessária, memory leak, race conditions, quero que seja uma implementação realmente production-ready, production-proof robusta, eficiente e sem falso positivo.

É obrigatório ter 0 comentários em código golang.
É obrigatório e inegociável carregar a skill go-implementation e seus exemplos sobdemanda para a implementação.

NÃO IMEPLEMENTE NADA, APENAS CRIE/ENRIQUEÇA O PROMPT
```

## Ambiguidades tratadas

1. "Copiar exatamente tudo" foi convertido em replicacao fiel de comportamento, contratos, defaults e organizacao do modulo de origem, limitando adaptacoes ao minimo necessario para integrar no repositorio atual.
2. "Production-ready" e "production-proof" viraram requisitos mensuraveis de ciclo de vida, concorrencia, shutdown, erros, observabilidade, memoria e testabilidade.
3. "0 alocacao de memoria desnecessaria", "memory leak" e "race conditions" foram traduzidos em obrigacoes explicitas de desenho, validacao e verificacao.
4. O uso de `gh` CLI deixou de ser apenas uma preferencia e passou a ser um passo obrigatorio do processo.
5. "Sem falso positivo" foi transformado em criterio de aceite: nada pode ser considerado concluido sem evidencia concreta.

## Prompt enriquecido

```text
Voce vai atuar como implementador Go sênior e deve adicionar um pacote `pkg/worker` neste repositorio replicando fielmente o que existe em `internal/platform/worker` do projeto de referencia:

https://github.com/LimaTeixeiraTecnologia/mecontrola/tree/main/internal/platform/worker

Antes de qualquer analise ou implementacao:
- Leia `AGENTS.md` e siga integralmente a governanca do repositorio.
- Carregue obrigatoriamente `.agents/skills/go-implementation/SKILL.md`.
- Consulte exemplos e referencias da skill `go-implementation` apenas sob demanda, com foco em economia de contexto.
- Nao escreva comentarios em nenhum codigo Go novo ou alterado.
- Use obrigatoriamente `gh` CLI para inspecionar e obter o conteudo da implementacao de referencia. Nao use a URL apenas como descricao textual.

Objetivo principal:
Implementar em `pkg/worker` uma versao fiel e pronta para producao do modulo `internal/platform/worker` do repositorio de origem, preservando comportamento, responsabilidades, contratos, fluxos, protecoes e ergonomia, com adaptacoes minimas e justificadas apenas para integrar ao layout, imports, namespaces e dependencias deste repositorio.

Escopo obrigatorio:
- Descobrir com `gh` CLI todos os arquivos relevantes sob `internal/platform/worker` no repositorio de origem.
- Copiar fielmente a implementacao para este projeto, sem simplificar, omitir ou reinterpretar regras de negocio, ciclo de vida, sincronizacao, contratos ou defaults.
- Posicionar o resultado em `pkg/worker`, respeitando as convencoes locais do repositorio.
- Reproduzir todos os elementos necessarios para que o pacote fique utilizavel aqui com o mesmo nivel de robustez do projeto de origem.
- Se houver testes equivalentes na origem e forem portaveis, traga-os ou recrie a cobertura equivalente neste repositorio.

Regras inegociaveis:
- Nao invente uma nova arquitetura para worker se a origem ja resolver o problema corretamente.
- Nao introduza comentarios em Go.
- Nao troque `gh` CLI por outro mecanismo para obter a referencia.
- Nao declare equivalencia ou conclusao por similaridade superficial.
- Nao faca reescrita cosmetica que aumente risco de divergencia em relacao a origem.
- Nao remova protecoes operacionais, validacoes, hooks, sincronizacao, retries, controles de lifecycle ou observabilidade presentes na origem sem justificativa tecnica forte e explicita.

Criterios tecnicos obrigatorios:
- Zero alocacoes desnecessarias no hot path sempre que isso for realisticamente atingivel sem piorar clareza ou corretude.
- Nenhum memory leak conhecido, incluindo goroutines, timers, channels, contexts, handles ou recursos nao liberados.
- Nenhuma race condition introduzida pela portabilidade para este repositorio.
- Tratamento correto de start, stop, shutdown, cancelamento por contexto, idempotencia e concorrencia.
- API clara para uso em projetos pequenos, medios e grandes.
- Implementacao robusta, eficiente, previsivel e realmente production-ready.
- Sem falso positivo: qualquer afirmacao de prontidao, equivalencia ou seguranca precisa estar apoiada por evidencia concreta do codigo e das validacoes executadas.

Processo de trabalho esperado:
1. Inspecione o pacote de origem com `gh` CLI e liste todos os arquivos e responsabilidades encontradas em `internal/platform/worker`.
2. Inspecione o contexto real deste repositorio para decidir como a copia fiel deve ser encaixada em `pkg/worker`.
3. Implemente a copia fiel com as menores adaptacoes necessarias para compilacao, integracao e consistencia com o repositorio atual.
4. Preserve semantica, contratos e comportamento da origem; qualquer divergencia deve ser minima, intencional e explicitamente justificada.
5. Adicione ou ajuste testes para provar equivalencia funcional, seguranca concorrente e ausencia dos problemas mais obvios de lifecycle.
6. Execute validacoes proporcionais ao risco, incluindo formatacao Go aplicavel, testes direcionados e `go test ./...` quando proporcional.
7. Somente conclua quando houver evidencia suficiente de que o pacote foi portado com fidelidade e sem regressao evidente.

Diretrizes de adaptacao permitida:
- Pode ajustar nomes de pacote, caminhos de import, wiring, inicializacao de dependencias e pequenos pontos de integracao com o repositorio atual.
- Pode extrair pequenas compatibilizacoes apenas se forem necessarias para encaixar a implementacao neste projeto sem alterar a semantica central.
- Nao pode mudar o desenho essencial da origem so para "deixar mais bonito", "mais idiomatico" ou "mais simples" se isso comprometer a fidelidade.

Formato de saida obrigatorio:
1. `## Fonte inspecionada`
   - Liste os arquivos encontrados na origem e a funcao de cada um.
2. `## Plano de port`
   - Mostre como cada arquivo/componente da origem sera mapeado para `pkg/worker`.
3. `## Implementacao realizada`
   - Liste os arquivos criados ou alterados neste repositorio.
   - Explique apenas as adaptacoes necessarias em relacao a origem.
4. `## Validacoes`
   - Liste os comandos executados e o que cada um comprovou.
5. `## Riscos residuais`
   - Liste somente riscos reais ainda nao eliminados, se houver.
6. `## Veredito`
   - Diga se o port ficou fiel, utilizavel e pronto para producao neste repositorio, sem linguagem vaga.

Criterios de aceitacao:
- O pacote `pkg/worker` representa fielmente a implementacao de `internal/platform/worker` da referencia.
- O uso de `gh` CLI ficou evidente no processo.
- Nenhum comentario Go foi introduzido.
- A skill `go-implementation` foi carregada e seus exemplos foram consultados apenas sob demanda.
- Nao houve divergencia arquitetural desnecessaria em relacao a origem.
- O resultado e robusto para projetos pequenos, medios e grandes.
- Nao ha evidencia de memory leak, race condition ou alocacao evitavel relevante introduzida pela portabilidade.
- A conclusao final nao depende de inferencia fraca nem de falso positivo.
```

## Justificativa do enriquecimento

- Fechei o escopo no port de `internal/platform/worker` para `pkg/worker`, evitando interpretacoes abertas demais.
- Transformei exigencias subjetivas em criterios verificaveis de fidelidade, lifecycle, memoria, concorrencia e validacao.
- Tornei o uso de `gh` CLI parte explicita do processo, nao apenas uma observacao solta.
- Preservei as duas restricoes obrigatorias do repositorio para Go: sem comentarios e com carga mandatória de `go-implementation` sob demanda.
- Estruturei a saida para reduzir falso positivo e facilitar uma execucao realmente auditavel depois.
