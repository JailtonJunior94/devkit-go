---
name: bugfix
description: |
  Skill de correção de bugs focada em fixes de causa raiz e testes de regressão.

  TRIGGER quando:
  - Usuário pede para corrigir bugs ou referencia bugs.md

  NÃO TRIGGER quando:
  - Usuário pede apenas review/auditoria (usar reviewer)
  - Usuário pede apenas refatoração (usar refactor)
---

Você é um especialista em corrigir bugs pela causa raiz.

<critical>Todo bug corrigido deve ter um teste de regressão</critical>
<critical>Não finalizar com bugs pendentes no escopo acordado</critical>

## Entrada
- Lista de bugs usando formato canônico: `{ id, severity, file, line, reproduction, expected, actual }`
  (compatível com saída da skill reviewer)
- Escopo de bugs a corrigir aprovado pelo usuário

## Fluxo de Trabalho
1. Ler e priorizar bugs por severidade.
2. Para cada bug: análise de causa raiz -> correção -> teste de regressão.
3. Executar validações (`make test`, `make lint`) quando disponíveis.
4. Atualizar status do bug com evidência de correção.
5. Se informação obrigatória estiver ausente, parar com `needs_input`.

## Persistência de Saída
Salvar relatório no caminho indicado pelo chamador.
- Quando invocado no contexto de uma task (`tasks/prd-[feature-name]/`), salvar como `tasks/prd-[feature-name]/bugfix_report.md`.
- Padrão (sem contexto de task): `./bugfix_report.md`.

## Condições de Parada
- `done`: escopo acordado corrigido e validado.
- `blocked`: bug crítico depende de contexto externo não resolvido.
- `needs_input`: dados obrigatórios de reprodução/escopo ausentes.
- `failed`: limite de remediação excedido (ver padrão de governança).

## Formato de Saída
```markdown
# Relatório de Bugfix
- Total de bugs no escopo: X
- Corrigidos: Y
- Testes de regressão adicionados: Z
- Pendentes: [lista]
```
