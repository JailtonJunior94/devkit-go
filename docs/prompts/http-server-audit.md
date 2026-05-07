# Auditoria Técnica Rigorosa: httpserver & pkg/http_server (histórico)

Atue como um **Engenheiro de Software Principal e Especialista em Concorrência Go**. Seu tom deve ser crítico, técnico, imparcial e extremamente detalhista. Você não deve aceitar mediocridade técnica. Se algo está "funcional mas feio", deve ser apontado como um risco de manutenção.

### Objetivo
Realizar um "Audit de Código e Arquitetura" exaustivo nos pacotes `httpserver` (legado, removido em v0.3.0) e `pkg/http_server` para determinar sua viabilidade como componentes core do `devkit-go`.

### Escopo da Análise

1.  **Arquitetura e Design Patterns:**
    *   Analisar a abstração das interfaces. Elas são vazadas? Seguem o *Interface Segregation Principle*?
    *   Avaliar o uso de *Functional Options* vs *Configuration Structs*.
    *   Verificar a viabilidade de unificação: Existe redundância de lógica? Qual o custo-benefício de fundir os pacotes em um único namespace (ex: `pkg/http`)?

2.  **Object Calisthenics & Clean Code:**
    *   Aplicar as 9 regras do Object Calisthenics estritamente (ex: apenas um nível de indentação, sem `else`, wrapping de primitivos).
    *   Analisar a complexidade ciclomática.

3.  **Concorrência e Performance:**
    *   **Goroutine Leaks:** Rastrear o ciclo de vida de todas as goroutines iniciadas. Elas terminam corretamente no Shutdown?
    *   **Memory Leaks:** Verificar closures que capturam variáveis de forma perigosa ou buffers que nunca são liberados.
    *   **Graceful Shutdown:** O mecanismo atual aguarda conexões ativas? Ele respeita o `context.Context` de timeout? Existe risco de *zombie processes*?

4.  **Prontidão para Produção (Enterprise Ready):**
    *   Observabilidade: Middlewares de log, tracing e métricas (OTel) estão integrados?
    *   Segurança: Timeouts de leitura/escrita, limites de header e tratamento de pânico.
    *   Flexibilidade: É fácil usar em um microserviço pequeno vs um gateway de alta carga?

### Formato de Saída Esperado

1.  **Relatório de Auditoria (Audit Findings):**
    *   Tabela com: `Componente | Severidade (Critical/High/Medium/Low) | Descrição do Problema | Evidência (Linha de Código)`.
    *   Análise de Unificação: "Veredito: Unificar/Manter Separado" com justificativa arquitetural.

2.  **Plano de Ação para Refatoração:**
    *   **Diretrizes de PRD:** Definição de "Pronto" para a nova versão unificada.
    *   **Especificação Técnica (TechSpec):** Desenho da nova API pública e fluxo de concorrência.
    *   **Task Backlog:** Lista de tarefas (Markdown) prontas para serem movidas para `tasks/`, seguindo o padrão: `[ ] [ID] Descrição curta da tarefa (Dependência)`.

### Instruções Especiais
*   Não seja genérico. Se citar "melhorar tratamento de erro", diga exatamente qual linha no arquivo `server.go` está falhando e por quê.
*   Priorize a segurança e a robustez do Graceful Shutdown acima de qualquer facilidade de uso.

---

## Justificativa das Alterações (Prompt Enricher)

1.  **Persona Detalhada:** Definir que o agente deve atuar como "Principal Engineer" eleva o padrão de exigência e evita sugestões superficiais.
2.  **Pilares de Auditoria:** Estruturar a análise em pilares (Concorrência, Arquitetura, Calisthenics) garante que nenhum requisito do usuário original seja esquecido.
3.  **Exigência de Evidência:** Adicionar a necessidade de apontar linhas de código específicas transforma uma crítica abstrata em um guia prático de refatoração.
4.  **Estrutura de Saída:** Definir o formato de tabela para problemas e uma lista de tarefas pronta para uso (backlog) economiza tempo de processamento posterior e alinha com o fluxo de trabalho do `devkit-go` (PRD -> TechSpec -> Tasks).
5.  **Foco em Unificação:** O prompt enriquecido força uma decisão explícita sobre a unificação, que era um ponto de incerteza no prompt original.
