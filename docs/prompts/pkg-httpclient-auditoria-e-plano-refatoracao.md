# Prompt para auditoria e plano de refatoracao do `pkg/httpclient`

## Prompt original

> Eu quero que analise o pkg/httpclient, e veja se realmente e um pacote robusto, eficiente, para importar em projetos pequenos, medios e de grande porte, sem falso positivo, quero que seja production-proof/ready sem falso positivo, remova TODOS comentarios e use de forma obrigatoria a skill go-implementation carregando seus exemplos e skill sobdemanda com foco em economia.
>
> Crie um plano de refatoracao, reduza se possivel a quantidade de codigo, deixe bem feito, utilizando golike, padroes de projeto e algo portavel em outros.
>
> Nao crie nada, apenas crie/enriqueca o prompt.

## Prompt enriquecido

```text
Voce vai atuar como auditor tecnico e arquiteto Go, sem implementar nenhuma mudanca. Sua tarefa e analisar o pacote `pkg/httpclient` deste repositorio e entregar um diagnostico rigoroso, sem falso positivo, sobre se ele esta realmente pronto para uso em producao e adequado para importacao em projetos pequenos, medios e grandes.

Antes de analisar:
1. Leia `AGENTS.md` e siga a governanca do repositorio.
2. Carregue obrigatoriamente a skill `go-implementation`.
3. Consulte exemplos e referencias dessa skill apenas sob demanda, com foco em economia de contexto.
4. Nao escreva nem sugira codigo novo nesta etapa.
5. Nao proponha mudancas especulativas; toda conclusao precisa estar ancorada em evidencias reais do codigo.

Escopo minimo a inspecionar em `pkg/httpclient`:
- `client.go`
- `observableclient.go`
- `request.go`
- `request_options.go`
- `client_options.go`
- `retry_policy.go`
- `retryable.go`
- `transport_retry.go`
- `transport_observable.go`
- `instrumentation.go`
- `constants.go`
- testes relacionados do pacote

Contexto observado do pacote:
- Ha uma interface `HTTPClient` com wrappers simples baseados em `net/http`.
- Ha um `ObservableClient` com retry, observabilidade e composicao de transports.
- Ha helpers genericos para request/response JSON em `request.go`.
- O pacote mistura preocupacoes de transporte HTTP, retry, observabilidade, limites de payload e ergonomia de API.

Objetivo da analise:
Determinar se o pacote e realmente robusto, eficiente, previsivel, portavel e production-ready para diferentes escalas de uso, identificando apenas problemas reais, com causa raiz e impacto concreto.

O que voce deve avaliar:
1. Robustez de API publica:
   - clareza da API
   - coesao entre tipos e responsabilidades
   - facilidade de uso correto e dificuldade de uso incorreto
   - estabilidade para consumidores externos
2. Eficiencia e comportamento operacional:
   - timeouts
   - connection pooling
   - reuso de transporte
   - retry/backoff/jitter
   - consumo de memoria
   - leitura, bufferizacao e drenagem de body
3. Confiabilidade em producao:
   - seguranca contra vazamentos de conexao ou goroutine
   - cancelamento por contexto
   - comportamento em falhas transientes e nao transientes
   - compatibilidade com workloads pequenos, medios e de alto volume
4. Portabilidade e acoplamento:
   - dependencia de observabilidade
   - facilidade de reutilizar o pacote em outros projetos
   - pontos que dificultam adocao em bibliotecas, servicos ou SDKs
5. Qualidade de desenho Go-like:
   - nomes, fronteiras e composicao
   - excesso de abstracao ou falta dela
   - duplicacao
   - oportunidades reais de reduzir codigo sem perder clareza
6. Higiene de codigo:
   - todo comentario Go atual que deveria desaparecer em uma refatoracao futura
   - trechos cuja documentacao em comentario mascara design ruim ou excesso de API

Regras obrigatorias:
- Nao implemente nada.
- Nao escreva diff.
- Nao gere codigo de exemplo.
- Nao invente problema so porque algo "parece estranho".
- Se nao houver evidencia suficiente para afirmar um problema, diga explicitamente que a evidencia e insuficiente.
- Cada achado precisa citar arquivo e linha.
- Diferencie claramente:
  - problema confirmado
  - risco plausivel ainda nao comprovado
  - decisao aceitavel mesmo que haja trade-off
- Considere como criterio forte de qualidade a possibilidade de reduzir codigo, simplificar a API e manter portabilidade.
- Considere como diretriz obrigatoria de eventual plano futuro a remocao de TODOS os comentarios em codigo Go alterado.

Formato de saida obrigatorio em Markdown:

# Auditoria do pkg/httpclient

## 1. Resumo executivo
- Veredito objetivo: `aprovado`, `aprovado com ressalvas` ou `reprovado para uso production-ready`
- Justificativa curta e direta

## 2. Mapa do pacote
- Liste os componentes principais e a responsabilidade real de cada um
- Aponte sobreposicoes, duplicidades e acoplamentos

## 3. Avaliacao por criterio
Para cada criterio abaixo, atribua nota de 0 a 5 e justifique com evidencias:
- robustez
- eficiencia
- ergonomia
- portabilidade
- testabilidade
- prontidao para producao

## 4. Achados confirmados
Para cada achado, use este formato:
- **Titulo**
  - Evidencia: `arquivo:linha`
  - Impacto real
  - Por que isso importa em producao
  - Severidade: baixa, media, alta ou critica
  - Escala afetada: pequena, media, grande ou todas

## 5. Pontos aceitaveis
- Liste decisoes que estao boas o suficiente e nao devem ser tratadas como problema

## 6. Riscos que exigem validacao adicional
- Liste apenas o que nao pode ser afirmado sem benchmark, teste de carga ou cenario adicional

## 7. Plano de refatoracao
Monte um plano incremental, sem executar nada, contendo:
- ordem recomendada das etapas
- objetivo de cada etapa
- simplificacao esperada
- reducao de codigo esperada, quando aplicavel
- impacto em compatibilidade publica
- risco da etapa
- criterio objetivo de pronto

O plano deve priorizar:
1. remocao de complexidade desnecessaria
2. reducao real de codigo
3. desenho Go-like
4. portabilidade para outros projetos
5. previsibilidade operacional em producao

## 8. Estado alvo
Descreva como o pacote deveria ficar apos a refatoracao, incluindo:
- API publica desejada
- responsabilidades por arquivo ou modulo
- como tratar retry, observabilidade e request helpers
- quais comentarios devem desaparecer por completo

## 9. Recomendacao final
- Diga se hoje voce recomendaria ou nao esse pacote para uso em projetos pequenos, medios e grandes
- Separe a recomendacao por escala, se necessario
- Nao use linguagem diplomatica; seja preciso

Criterios de aceitacao da sua resposta:
- Nenhum falso positivo
- Nenhuma sugestao sem evidencias
- Nenhuma implementacao
- Nenhum codigo gerado
- Nenhuma generalidade vazia
- Todo julgamento relevante referenciado em arquivos/linhas
- Plano de refatoracao acionavel e incremental
```

## O que foi enriquecido

- Fechei o escopo da analise para o `pkg/httpclient` real do repositorio, incluindo os arquivos centrais e os testes.
- Transformei "production-proof/ready" em criterios verificaveis: robustez, eficiencia, confiabilidade, portabilidade, ergonomia e prontidao para producao.
- Forcei saida estruturada em Markdown com veredito, notas, achados confirmados, riscos nao comprovados e plano incremental de refatoracao.
- Adicionei exigencia explicita de evidencias por arquivo/linha para reduzir falso positivo.
- Mantive a obrigatoriedade de usar `go-implementation`, mas com consulta sob demanda para economia de contexto.
- Reforcei que a tarefa e apenas analisar e planejar, sem criar codigo nem aplicar mudancas.
