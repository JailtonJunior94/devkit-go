# Governança de Regras

- Rule ID: R-GOV-001
- Severidade: hard
- Escopo: Todos os arquivos em `.claude/rules/`, `.claude/commands/` e `.claude/skills/`.

## Objetivo
Definir governança determinística para `.claude/rules/`: precedência, severidade, resolução de conflitos, segurança de execução e política de evidência.

## Escopo das Regras
Todos os commands e skills seguem implicitamente todas as regras em `.claude/rules/`. Regras têm precedência em caso de conflito. Todos os commands e skills têm acesso implícito aos arquivos de `.claude/context/`.

## Metadados de Regra (obrigatório para todas as regras)
Cada arquivo de regra deve declarar:
- `Rule ID`: identificador único (ex.: `R-GOV-001`)
- `Severidade`: `hard` ou `guideline`
- `Escopo`: arquivos/camadas afetados

## Precedência (da mais alta para a mais baixa)
1. `00-governance.md` (este arquivo)
2. `security.md`
3. `architecture.md`
4. `http.md`, `error-handling.md` e `o11y.md`
5. `tests.md` e `code-standards.md`

Se duas regras do mesmo nível conflitarem:
1. Preferir a regra com maior severidade (`hard` > `guideline`).
2. Se mesma severidade, preferir o comportamento mais restritivo para segurança/correção.

## Modelo de Severidade

### hard
Bloqueante para merge. Não pode ser ignorada.
Aplica-se a: segurança, dados sensíveis, exposição de erros, fronteiras de arquitetura, segurança SQL, comportamento determinístico do agente.

### guideline
Não bloqueante por padrão. Pode ser ignorada com justificativa documentada.
Aplica-se a: preferências de nomenclatura, alvos de tamanho de arquivo/função, formatação, convenções de estilo de teste.

## Máquina de Estados Canônica
- Estados de execução permitidos: `pending`, `in_progress`, `needs_input`, `blocked`, `failed`, `done`.
- Vereditos de gate permitidos: `APPROVED`, `APPROVED_WITH_REMARKS`, `REJECTED`, `BLOCKED`.
- Estados e vereditos são enums separados — nunca misturar.

## Restrições de Segurança do Agente
- Todo command/skill deve declarar condições de parada explícitas.
- Workflows de longa duração devem definir ciclos máximos de remediação.
- Se input obrigatório estiver ausente, a execução deve parar com status `needs_input`.
- Operações destrutivas requerem intenção explícita do usuário na thread atual.
- Se a intenção for ambígua, parar com `needs_input`.
- Em dependências externas indisponíveis, continuar apenas com suposições explícitas registradas no relatório de execução.
- Progresso baseado em suposições não pode aprovar gates de segurança ou correção.

## Política de Evidência
- Relatórios de execução devem incluir: comandos executados, arquivos alterados, resultados de validação (pass/fail por gate), suposições e riscos residuais.
- Decisões de gate devem incluir: nome do gate, veredito, razão objetiva.
- Proibido: aprovação sem evidência, status ambíguos como "provavelmente ok".

## Tags de Prompt
- `<critical>`: instrução que o agente nunca deve pular ou despriorizar.
- `<requirements>`: lista de requisitos obrigatórios para a tarefa atual.

## Limite de Remediação Padrão
- Salvo quando sobrescrito, o máximo de ciclos de remediação padrão é **2** por estágio ou por bug.

## Política de Idioma
- Símbolos de código, testes e comentários de código devem estar em inglês.
- Termos legais/de produto do domínio podem preservar a nomenclatura original quando necessário.

## Proibido
- Loops infinitos de remediação sem limite de ciclos.
- Transições de estado implícitas.
- Concluir tarefas com achados críticos/major não resolvidos.
- Nomes de estado ad-hoc fora dos enums canônicos definidos acima.
- Aprovação sem evidência.
