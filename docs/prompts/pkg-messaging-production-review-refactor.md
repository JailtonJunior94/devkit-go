# Prompt enriquecido: auditoria e plano de refatoracao de `pkg/messaging`

## Comparacao rapida

| Aspecto | Prompt original | Prompt enriquecido |
|---|---|---|
| Escopo | Analise ampla de `pkg/messaging` | Escopo fechado em `pkg/messaging`, sem implementacao nem edicao |
| Robustez | Termo subjetivo | Dimensoes mensuraveis com notas, evidencias e risco |
| "Sem falso positivo" | Intencao | Regra explicita de evidencias, confianca e marcacao de "nao comprovado" |
| Refatoracao | Pedido generico | Plano em fases curtas, verificaveis e orientadas a causa raiz |
| Reducao de codigo | "Se possivel" | Apenas quando houver ganho concreto de simplicidade e portabilidade |
| Go / skills | Obrigatoriedade mencionada | Leitura de `AGENTS.md` + carga obrigatoria de `go-implementation` com exemplos sob demanda |
| Comentarios | Remover todos | Tratado como requisito obrigatorio do plano futuro, sem executar agora |

## Prompt original

```text
Eu quero que analise o pkg/messaging, e veja se realmente e um pacote robusto, eficiente, para importar em projetos pequenos, medios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentarios e use de forma obrigatoria a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.

Crie um plano de refatoracao, reduza se possivel a quantidade de codigo, deixe bem feito, utilizando golike, padroes de projeto e algo portavel em outros.

NAO CRIE NADA, APENAS CRIE/ENRIQUECA O PROMPT.
```

## Ambiguidades tratadas

1. "Robusto", "eficiente" e "production-proof/ready" foram convertidos em criterios auditaveis.
2. "Sem falso positivo" virou regra objetiva de evidencia, confianca e proibicao de inferencia fraca.
3. "Reduza se possivel a quantidade de codigo" foi limitado a reducoes justificadas, sem reescrita especulativa.
4. "Remova TODOS comentarios" foi preservado como requisito obrigatorio do plano futuro, nao como execucao imediata.
5. "Go-like", "padroes de projeto" e "portavel" foram traduzidos para idiomatismo Go com justificativa concreta, sem abstrair em excesso.

## Prompt enriquecido

```text
Analise exclusivamente o pacote `pkg/messaging` deste repositorio Go e produza um diagnostico tecnico rigoroso, sem implementar nada e sem editar arquivos.

Antes de qualquer analise:
- Leia `AGENTS.md` e siga a governanca do repositorio.
- Carregue obrigatoriamente `.agents/skills/go-implementation/SKILL.md`.
- Consulte referencias e exemplos da skill `go-implementation` apenas sob demanda, com foco em economia de contexto.
- Nao gere codigo, nao aplique refactor, nao abra PR, nao crie testes e nao altere arquivos. Entregue apenas analise e plano.
- Considere como requisito obrigatorio do futuro refactor a remocao de TODOS os comentarios do codigo Go afetado.

Contexto minimo do escopo:
- O pacote exposto por `pkg/messaging` possui abstracoes centrais em `pkg/messaging/publisher.go` e `pkg/messaging/consumer.go`.
- Ha implementacoes e adapters em `pkg/messaging/kafka/` e `pkg/messaging/rabbitmq/`, incluindo configuracao, options, auth, otel, dlq, lifecycle, health e testes.
- O repositorio usa Go 1.26.2.

Objetivo:
Verificar se `pkg/messaging` esta realmente pronto para uso em projetos pequenos, medios e grandes, com foco em robustez, eficiencia, ergonomia de API, extensibilidade, observabilidade, concorrencia, seguranca operacional, tratamento de erros, shutdown/reconnect, testabilidade e portabilidade. A conclusao nao pode conter falso positivo: toda afirmacao deve estar ancorada em evidencia concreta do codigo.

Criterios de analise:
- Avalie acoplamento interno e externo, clareza das abstracoes publicas, consistencia entre Kafka e RabbitMQ, custo operacional, risco de leaks/races, defaults perigosos, duplicacao, excesso de codigo e capacidade de reutilizacao em outros projetos.
- Se algo parecer positivo, prove com evidencias concretas. Se algo parecer negativo, mostre a causa raiz, o impacto e os arquivos envolvidos.
- Quando a evidencia for insuficiente, marque explicitamente como `nao comprovado` em vez de assumir.
- Proponha reducao de codigo apenas quando houver ganho claro de simplicidade, manutenibilidade ou portabilidade.
- Prefira recomendacoes idiomaticas de Go, padroes de projeto ja justificados pelo contexto e solucoes portaveis, sem abstracoes desnecessarias.
- Preserve API publica sempre que possivel; se algum ponto exigir ruptura, sinalize explicitamente.

Formato de saida obrigatorio em Markdown e PT-BR:
1. `## Veredito final`
   - Classifique o pacote como `Aprovado`, `Aprovado com ressalvas` ou `Nao aprovado para production-ready`.
   - Resuma em no maximo 10 linhas.
2. `## Matriz de avaliacao`
   - Use uma tabela com colunas: `Dimensao | Nota (0-5) | Evidencias | Risco | Confianca`.
   - Inclua no minimo: robustez, eficiencia, ergonomia, concorrencia, observabilidade, seguranca operacional, testabilidade, portabilidade, simplicidade e readiness para pequena/media/grande escala.
3. `## Achados priorizados`
   - Liste apenas achados reais, com prioridade `P0` a `P3`.
   - Para cada item, informe: problema, causa raiz, impacto, evidencias (`arquivo:linha`), risco de falso positivo (`baixo/medio/alto`) e por que isso importa em producao.
4. `## Comentarios a remover`
   - Liste os grupos de arquivos onde comentarios devem ser removidos no futuro refactor.
   - Nao remova nada agora; apenas identifique e priorize.
5. `## Plano de refatoracao`
   - Monte um plano em fases curtas e verificaveis.
   - Para cada fase, informe: objetivo, arquivos/pacotes afetados, reducao de codigo esperada (se aplicavel), risco, estrategia de validacao e criterio de conclusao.
   - O plano deve priorizar menor mudanca segura, causa raiz e reaproveitamento portavel.
6. `## Recomendacoes de desenho`
   - Indique ajustes de design e padroes somente quando houver justificativa concreta.
   - Explique como cada recomendacao melhora portabilidade, manutencao e readiness.
7. `## Nao fazer`
   - Liste refactors que pareceriam bons, mas seriam arriscados, prematuros ou potencialmente falso-positivos.

Regras de qualidade:
- Nao use linguagem vaga como "parece bom", "esta ok" ou "talvez seja melhor" sem evidencias.
- Nao elogie nem critique por intuicao.
- Nao confunda quantidade de features com robustez.
- Nao trate comentarios existentes como documentacao necessaria: considere sua remocao como requisito do plano.
- Se encontrar testes relevantes, use-os como evidencia; se houver lacunas, aponte a lacuna como lacuna, nao como bug confirmado.
- Considere explicitamente se o pacote e importavel e sustentavel para projetos pequenos, medios e grandes, sem duplicar a analise.

Criterios de aceitacao da sua resposta:
- Toda conclusao relevante possui evidencia concreta.
- Todo risco relevante tem causa raiz e impacto.
- O plano de refatoracao e executavel, incremental e sem reescrita ampla por impulso.
- A resposta separa claramente fatos observados, inferencias fortes e pontos nao comprovados.
- A resposta evita falso positivo de forma explicita.
```

## Justificativa do enriquecimento

- Converte objetivos vagos em dimensoes e criterios mensuraveis.
- Impoe um formato de resposta que reduz subjetividade e falsa confianca.
- Delimita o escopo para impedir implementacao ou refactor prematuro.
- Explicita a obrigatoriedade da skill `go-implementation` com consulta sob demanda.
- Transforma o pedido em um prompt acionavel, auditavel e reutilizavel.
