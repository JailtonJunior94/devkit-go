# Padrões de Código

- Rule ID: R-CODE-001
- Severidade: guideline (hard apenas quando ligado a correção/segurança/arquitetura)
- Escopo: Todos os arquivos `.go`.

## Objetivo
Garantir estilo de código e convenções de nomenclatura consistentes em todo o codebase Go.

## Requisitos

### Idioma
- Ver Política de Idioma em `00-governance.md`.

### Convenções de Nomenclatura
- `camelCase`: variáveis locais, parâmetros de função, campos não exportados.
- `PascalCase`: funções, métodos, structs, interfaces e constantes exportados.
- `snake_case`: nomes de arquivos e diretórios.
- Nomes de interface devem ser baseados em comportamento, sem prefixo `I`.

### Clareza de Nomenclatura
- Evitar abreviações obscuras.
- Abreviações idiomáticas permitidas: `ctx`, `err`, `id`, `db`, `http`, `tx`.
- Preferir nomes com menos de 30 caracteres, salvo quando clareza exigir mais.
- Funções devem começar com verbo.
- Variáveis booleanas devem ser lidas como asserções (`isActive`, `hasPermission`).

### Design de Funções
- Preferir guard clauses e early returns.
- Evitar condicionais aninhadas com mais de 2 níveis.
- Evitar `else` após `return` explícito.
- Não usar parâmetros booleanos de flag para alternar comportamento.

### Constantes e Parâmetros
- Substituir magic numbers sem explicação por constantes nomeadas.
- Preferir até 3 parâmetros posicionais; usar struct de params quando a legibilidade melhorar.

### Alvos de Tamanho (guideline)
- Funções: alvo de até 50 linhas.
- Arquivos: alvo de até 300 linhas.

### Comentários
- Preferir código autoexplicativo.
- Adicionar comentários para invariantes, trade-offs e restrições externas.
- Manter comentários godoc para símbolos exportados.

## Proibido
- Símbolos em português no código.
- Parâmetros booleanos de flag que alternam comportamento de função.
- Aninhamento profundo sem razão forte.
- Comentários que apenas restam código óbvio.
