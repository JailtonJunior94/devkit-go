# Prompt para analise e plano de refatoracao do `pkg/http_server`

## Prompt original

> Eu quero que analise o pkg/http_server, e veja se realmente é um pacote robusto, eficiente, para importar em projetos pequenos, médios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentários e use de forma obrigatória a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.
>
> Crie um plano de refatoração, reduza se possível a quantidade de código, deixe bem feito, utilizando golike, padrões de projeto e algo portável em outros.
>
> NÃO CRIE NADA, APENAS CRIE/ENRIQUEÇA O PROMPT.

## Comparacao resumida

| Aspecto | Original | Enriquecido |
| --- | --- | --- |
| Escopo | Analise ampla | Analise delimitada ao `pkg/http_server` e seus subpacotes `common`, `chi_server` e `server_fiber` |
| Evidencia | Implicita | Obrigatoria, com citacoes por arquivo/linha e sem conclusoes sem prova |
| Saida | Nao definida | Markdown estruturado com diagnostico, matriz de prontidao e plano de refatoracao |
| Restricoes | Genericas | Nao implementar nada, nao editar nada, nao criar codigo, apenas analisar e planejar |
| Robustez | Subjetiva | Critérios objetivos: API, defaults, seguranca, timeouts, lifecycle, observabilidade, extensibilidade, portabilidade e custo de manutencao |
| Sem falso positivo | Pedido explicito | Regras para separar fato verificado, risco provavel e hipotese pendente |

## Ambiguidades eliminadas

1. "Production-ready" foi convertido em criterios verificaveis e nao em opiniao.
2. "Sem falso positivo" foi convertido em obrigacao de citar evidencias e classificar nivel de confianca.
3. "Remova TODOS comentarios" foi mantido como requisito do plano de refatoracao, sem executar alteracoes agora.
4. "Reduza codigo" foi limitado a reducoes justificadas por duplicacao, complexidade acidental ou API redundante.
5. "Portavel em outros" foi convertido em criterios de portabilidade, acoplamento e reaproveitamento entre adaptadores.

## Prompt enriquecido

```md
Voce vai atuar como um revisor tecnico senior de Go e deve analisar exclusivamente o pacote `pkg/http_server` deste repositorio, incluindo os subpacotes `common`, `chi_server` e `server_fiber`, para responder se ele esta realmente pronto para uso em projetos pequenos, medios e grandes sem conclusoes apressadas e sem falso positivo.

### Carga obrigatoria de contexto

1. Leia `AGENTS.md` antes de qualquer analise.
2. Considere o contexto real do repositorio:
   - arquitetura: monolito Go;
   - frameworks detectados: Fiber e gRPC;
   - versao de Go: confirmar em `go.mod`;
   - o pacote `pkg/http_server` expõe um toolkit HTTP com dois adaptadores (`chi_server` e `server_fiber`) e um nucleo compartilhado em `common`.
3. Carregue obrigatoriamente a skill `go-implementation`, mas consulte exemplos e referencias apenas sob demanda, com foco em economia de contexto.
4. Nao implemente nada. Nao altere arquivos. Nao gere patch. Nao crie codigo novo. Nao reescreva o pacote. Apenas analise e entregue diagnostico com plano de refatoracao.

### Objetivo

Avalie se `pkg/http_server` e de fato um pacote:

- robusto;
- eficiente;
- seguro;
- ergonomico para consumo;
- portavel para outros projetos;
- adequado para projetos de pequeno, medio e grande porte;
- realmente production-ready / production-proof com base em evidencia, e nao em impressao.

### Escopo da analise

Analise no minimo:

1. API publica e ergonomia de uso.
2. Coesao e acoplamento entre `common`, `chi_server` e `server_fiber`.
3. Paridade de features entre os adaptadores.
4. Configuracao padrao e validacao de `Config`.
5. Lifecycle do servidor, startup, shutdown e comportamento sob timeout.
6. Error handling, sanitizacao de erro, RFC 7807 e previsibilidade de resposta.
7. Middlewares, ordem de execucao, seguranca, CORS, health checks, metrics e tracing.
8. Dependencia de `observability` e impacto disso na portabilidade do pacote.
9. Custos de manutencao, duplicacao, complexidade acidental e pontos de simplificacao.
10. Testes existentes, benchmarks e lacunas objetivas de cobertura para suportar claims de production-readiness.
11. Adequacao para consumo em projetos pequenos, medios e grandes, destacando trade-offs por porte.

### Regras para evitar falso positivo

1. Nao diga que algo e "production-ready" sem mostrar evidencias concretas.
2. Toda afirmacao relevante deve ter citacao objetiva no formato `arquivo:linha`.
3. Classifique cada ponto como uma das categorias abaixo:
   - `Verificado`
   - `Risco provavel`
   - `Hipotese pendente`
4. Se faltar evidencia, diga explicitamente que faltou evidencia.
5. Nao confunda "tem implementacao" com "esta pronto para producao".
6. Nao confunda "tem teste" com "esta coberto de forma suficiente".

### Requisitos especificos de avaliacao

Ao revisar o pacote, responda com objetividade:

1. O pacote hoje pode ser importado com seguranca por projetos pequenos? Por que?
2. O pacote hoje escala em clareza e manutencao para projetos medios? Por que?
3. O pacote hoje suporta cenarios de grande porte sem gerar acoplamento excessivo, duplicacao ou fragilidade operacional? Por que?
4. A coexistencia entre `chi_server` e `server_fiber` esta bem desenhada ou gera custo estrutural desnecessario?
5. Ha excesso de codigo, responsabilidades duplicadas ou API que poderia ser reduzida sem perda funcional?
6. O pacote esta idiomatico em Go ("golike") ou ha sinais de design que destoam do ecossistema?
7. O pacote esta portavel para outros projetos ou depende demais de detalhes internos deste repositorio?

### Tratamento do requisito "remover TODOS comentarios"

Nao remova comentarios agora.

Em vez disso:

1. avalie se existem comentarios em excesso, comentarios redundantes, comentarios que mascaram design ruim ou comentarios que deveriam virar nomes melhores e codigo mais claro;
2. inclua no plano de refatoracao uma etapa obrigatoria para remover todos os comentarios desnecessarios do codigo Go, preservando apenas o que for inevitavel por contrato externo, se houver;
3. se voce precisar mostrar qualquer pseudocodigo ou exemplo, nao use comentarios no codigo.

### Formato obrigatorio da resposta

Entregue a resposta em Markdown com esta estrutura:

## 1. Contexto carregado

- arquivos e areas inspecionadas;
- premissas validas;
- limites da analise.

## 2. Inventario objetivo do pacote

- subpacotes;
- responsabilidades;
- superficie publica principal;
- dependencias estruturais importantes.

## 3. Diagnostico tecnico

Use uma tabela com as colunas:

`Tema | Status | Evidencias | Impacto | Observacao`

Temas minimos:

- API publica
- Configuracao e defaults
- Lifecycle e shutdown
- Timeout e concorrencia
- Error handling
- Seguranca
- Observabilidade
- Portabilidade
- Duplicacao entre adaptadores
- Testes e benchmarks
- Prontidao para pequeno porte
- Prontidao para medio porte
- Prontidao para grande porte

## 4. Veredito de production-readiness

Informe:

- veredito atual: `Aprovado`, `Aprovado com restricoes` ou `Nao aprovado`;
- justificativa curta e objetiva;
- principais bloqueadores;
- quais claims podem ser feitos com seguranca e quais ainda nao podem.

## 5. Oportunidades reais de reducao de codigo

Liste apenas reducoes com justificativa clara, por exemplo:

- duplicacao entre adapters;
- wrappers desnecessarios;
- configuracoes redundantes;
- pontos em que a mesma regra aparece em mais de um lugar;
- comentarios que indicam complexidade evitavel.

Para cada item, informe `ganho esperado`, `risco` e `prioridade`.

## 6. Plano de refatoracao

Monte um plano incremental, production-safe e sem reescrita total, dividido por fases.

Cada fase deve conter:

- objetivo;
- motivacao;
- mudancas propostas;
- reducao de codigo esperada;
- risco;
- criterio objetivo de conclusao.

Inclua obrigatoriamente frentes para:

1. simplificacao estrutural;
2. reducao de duplicacao;
3. melhoria de API e portabilidade;
4. endurecimento de comportamento para producao;
5. remocao de comentarios desnecessarios;
6. validacao adicional para sustentar claims de production-readiness.

## 7. Ordem recomendada de execucao

Liste a sequencia ideal de implementacao, do menor risco para o maior impacto.

## 8. Conclusao executiva

Responda em no maximo 10 linhas:

- se vale importar o pacote hoje em projetos pequenos, medios e grandes;
- onde ele esta forte;
- onde ele ainda nao esta pronto;
- qual o primeiro passo de refatoracao com melhor custo-beneficio.

### Restricoes finais

- Nao escrever codigo.
- Nao propor reescrita ampla sem evidencias.
- Nao inventar requisitos.
- Nao omitir trade-offs.
- Nao usar linguagem vaga como "parece bom", "aparenta estar pronto" ou "provavelmente atende" sem evidencias.
- Priorizar precisao, objetividade e honestidade tecnica.
```

## Justificativa curta das adicoes

1. O escopo foi fechado no pacote real e nos subpacotes existentes para evitar analise generica.
2. O formato de saida foi fixado em Markdown estruturado para tornar a resposta auditavel.
3. Os criterios de aceitacao transformam "robusto" e "production-ready" em avaliacao verificavel.
4. As regras de evidencia reduzem o risco de falso positivo.
5. O requisito sobre comentarios foi preservado sem transformar a tarefa em implementacao.
6. O plano de refatoracao foi orientado a fases, risco e reducao real de codigo, sem incentivar reescrita.

## Variante curta

Se voce quiser uma versao mais enxuta para colar direto em outro agente, use apenas a secao `Prompt enriquecido`.
