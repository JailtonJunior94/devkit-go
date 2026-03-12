# Documento de Requisitos do Produto (PRD)

## Visão Geral

Desenvolvedores que trabalham com DDD precisam representar campos opcionais de domínio — campos que podem legitimamente não ter valor — sem recorrer a ponteiros (`*int`, `*string`) espalhados pelo código. Ponteiros nulos obrigam nil checks manuais, rompem a imutabilidade esperada de value objects, e não se integram bem com JSON (`0` em vez de `null`) nem com drivers SQL sem boilerplate extra.

Esta feature entrega um pacote `pkg/nullable` com value objects nulábeis para os tipos primitivos mais comuns (`int`, `int64`, `float32`, `float64`, `string`, `time.Time`). Os tipos são seguros em tempo de compilação, expressam intenção de domínio de forma clara, e são compatíveis de forma nativa com serialização JSON e a camada de acesso a dados (`database/sql`).

O pacote é uma biblioteca pública reutilizável por qualquer projeto Go que adote `devkit-go`.

---

## Objetivos

- **O1** — Eliminar nil checks ad-hoc para campos opcionais em value objects e entidades de domínio.
- **O2** — Garantir serialização JSON correta: valor presente → valor serializado; ausente → `null`.
- **O3** — Garantir compatibilidade com `database/sql` via `sql.Scanner` e `driver.Valuer` sem código extra no repositório.
- **O4** — Oferecer API mínima e expressiva: construir, verificar presença, extrair valor, converter para ponteiro.
- **O5** — Cobertura de testes abrangendo todos os tipos, casos nil/non-nil, round-trip JSON e Scan/Value.

---

## Histórias de Usuário

**US-01 — Campo opcional em entidade de domínio**
Como desenvolvedor Go escrevendo uma entidade de domínio, quero declarar `MiddleName nullable.String` em vez de `MiddleName *string`, para que o código expresse claramente que o campo é opcional sem expor um ponteiro que pode ser nilado acidentalmente.

**US-02 — Serialização JSON sem surpresas**
Como desenvolvedor integrando com uma API REST, quero que um campo `nullable.Int` presente serialize como `42` e um campo ausente serialize como `null`, sem precisar implementar `MarshalJSON` manualmente na minha struct.

**US-03 — Leitura do banco sem boilerplate**
Como desenvolvedor escrevendo um repositório SQL, quero que `nullable.String` implemente `sql.Scanner` para que eu possa fazer `rows.Scan(&entity.OptionalField)` diretamente, sem criar uma variável intermediária `sql.NullString` e converter depois.

**US-04 — Escrita no banco sem boilerplate**
Como desenvolvedor escrevendo uma query de insert/update, quero que `nullable.String` implemente `driver.Valuer` para que o driver SQL receba `nil` quando o campo está ausente e o valor concreto quando presente.

**US-05 — Extração segura de valor**
Como desenvolvedor consumindo um value object nulável, quero chamar `field.ValueOr("default")` para obter o valor ou um fallback declarativo, sem blocos `if field.IsNull()` repetidos.

**US-06 — Conversão para ponteiro quando necessário**
Como desenvolvedor fazendo bridge com uma API externa que aceita `*int64`, quero chamar `field.Ptr()` e receber `*int64` (nil quando ausente), sem fazer a conversão manual.

---

## Funcionalidades Core

### F-01 — Tipos nulábeis para os primitivos suportados

**O que faz:** Fornece tipos concretos `nullable.Int`, `nullable.Int64`, `nullable.Float32`, `nullable.Float64`, `nullable.String`, `nullable.Time`, cada um encapsulando um valor do tipo correspondente e um flag de presença.

**Por que é importante:** Cada tipo primitivo tem semântica de comparação e serialização distinta. Tipos concretos (em vez de um genérico único) permitem compatibilidade explícita com `encoding/json` e `database/sql` sem reflection excessiva e mantêm erros de tipo em tempo de compilação.

**Requisitos funcionais:**
- RF-01: Cada tipo deve ter um construtor `Of(value T) NullableT` que retorna uma instância com valor presente.
- RF-02: Cada tipo deve ter um construtor `Empty() NullableT` que retorna uma instância com ausência de valor.
- RF-03: O método `IsNull() bool` deve retornar `true` quando o valor está ausente.
- RF-04: O método `ValueOr(fallback T) T` deve retornar o valor quando presente ou `fallback` quando ausente.
- RF-05: O método `Ptr() *T` deve retornar ponteiro para o valor quando presente ou `nil` quando ausente.

### F-02 — Serialização e desserialização JSON

**O que faz:** Integra com `encoding/json` para que campos nulábeis sejam serializados como `null` (ausente) ou como o valor primitivo (presente), sem necessidade de implementação adicional no consumer.

**Por que é importante:** O comportamento padrão de campos com zero-value (`0`, `""`, `false`) é diferente de `null` em JSON. A distinção é semanticamente importante em APIs REST e contratos de mensagens.

**Requisitos funcionais:**
- RF-06: `MarshalJSON` deve produzir `null` quando `IsNull() == true`.
- RF-07: `MarshalJSON` deve produzir o valor serializado como JSON quando o valor está presente.
- RF-08: `UnmarshalJSON` deve definir o valor como ausente quando o JSON for `null`.
- RF-09: `UnmarshalJSON` deve definir o valor como presente e correto quando o JSON contiver um valor válido do tipo.
- RF-10: `UnmarshalJSON` deve retornar erro se o JSON contiver um valor de tipo incompatível (ex.: string para `nullable.Int`).

### F-03 — Compatibilidade com camada de acesso a dados (SQL)

**O que faz:** Implementa `sql.Scanner` e `driver.Valuer` em cada tipo para permitir `rows.Scan` e uso direto em queries parametrizadas.

**Por que é importante:** Repositórios SQL são a principal fronteira onde ausência de valor vem do banco (`NULL`). Sem essa integração, os desenvolvedores precisam de tipos intermediários (`sql.NullString`, etc.) e conversão manual — exatamente o boilerplate que esta feature elimina.

**Requisitos funcionais:**
- RF-11: `Scan(value any) error` deve definir o campo como ausente quando `value == nil` (coluna NULL no banco).
- RF-12: `Scan(value any) error` deve definir o campo como presente e converter corretamente os tipos que drivers SQL retornam (ex.: `[]byte`, `int64`, `float64`, `string`, `time.Time`).
- RF-13: `Scan` deve retornar erro descritivo para tipos incompatíveis.
- RF-14: `Value() (driver.Value, error)` deve retornar `nil` quando `IsNull() == true`.
- RF-15: `Value() (driver.Value, error)` deve retornar o valor como tipo compatível com `driver.Value` quando presente.

### F-04 — Comparação e utilidades

**O que faz:** Oferece métodos utilitários para comparação e depuração.

**Requisitos funcionais:**
- RF-16: `Equal(other NullableT) bool` deve retornar `true` quando ambos são ausentes ou ambos têm o mesmo valor.
- RF-17: `String() string` deve retornar representação legível: `"<null>"` para ausente ou o valor formatado para presente.

---

## Experiência do Usuário

_Omitida — feature exclusivamente backend/biblioteca._

---

## Restrições Técnicas de Alto Nível

- **RT-01** — O pacote deve residir em `pkg/nullable/` dentro do módulo `github.com/JailtonJunior94/devkit-go`.
- **RT-02** — Compatibilidade com `encoding/json` da stdlib (sem dependência de bibliotecas JSON externas).
- **RT-03** — Compatibilidade com `database/sql` da stdlib; não há dependência de driver específico (pgx, lib/pq, etc.).
- **RT-04** — Sem dependências externas além da stdlib Go; o pacote deve ser importável por qualquer projeto Go sem puxar deps transitivas.
- **RT-05** — Go 1.21+ (mínimo do módulo atual); uso de generics é permitido internamente se não exposto de forma que quebre APIs futuras.
- **RT-06** — A API pública deve ser mínima e estável: construtores `Of`/`Empty`, métodos `IsNull`, `ValueOr`, `Ptr`, `Equal`, `String`, mais as interfaces de JSON e SQL.

---

## Fora de Escopo

- Tipos numéricos adicionais (`uint`, `uint64`, `byte`, `rune`, `complex64`, etc.) — podem ser adicionados em versões futuras.
- Validação de domínio embutida (ex.: `NullableString` com tamanho máximo) — responsabilidade do consumer.
- Suporte a `encoding/xml` ou outros formatos de serialização.
- Operações aritméticas ou comparação de ordem (`Less`, `Greater`) sobre valores nulábeis.
- Integração com ORMs específicos (GORM, Ent) — a compatibilidade via `sql.Scanner`/`driver.Valuer` é suficiente e genérica.
- Tipo genérico `Nullable[T any]` público — pode ser considerado em iteração futura após validação dos tipos concretos.

---

## Suposições e Questões em Aberto

| # | Suposição / Questão | Impacto se errada |
|---|---|---|
| A1 | `nullable.Time` usa `time.Time` sem timezone embutido — o consumer é responsável pela localização. | Baixo; comportamento consistente com stdlib. |
| A2 | Os construtores `Of`/`Empty` são funções de pacote (não métodos de tipo), seguindo o padrão do `pkg/vos` existente. | Baixo; mudança de nomenclatura apenas. |
| A3 | A API não precisa suportar update parcial (patch semantics) diferente de ausência total — `IsNull` cobre ambos os casos. | Médio; se patch semântico for necessário, um terceiro estado `Undefined` pode ser requerido no futuro. |
| Q1 | `nullable.Float32` é necessário além de `float64`? Drivers SQL geralmente retornam `float64`. | Se não necessário, pode ser omitido para simplificar; mantido por ora conforme solicitado. |

---

## Critérios de Aceitação Resumidos

| ID | Critério |
|---|---|
| CA-01 | Todos os 6 tipos compilam e passam `go vet` e `staticcheck`. |
| CA-02 | Round-trip JSON: `Of(v) → Marshal → Unmarshal → ValueOr(zero) == v` para todos os tipos. |
| CA-03 | JSON null: `Empty() → Marshal` produz `null`; `Unmarshal(null) → IsNull() == true`. |
| CA-04 | SQL round-trip: `Scan(nil) → IsNull() == true`; `Scan(v) → ValueOr(zero) == v` para todos os tipos. |
| CA-05 | `Value()` retorna `nil` para ausente e valor compatível com `driver.Value` para presente. |
| CA-06 | `Equal` é reflexivo e simétrico; `ValueOr` não aloca quando valor está presente. |
| CA-07 | Testes cobrem casos: presente, ausente, zero-value presente (ex.: `Of(0)` ≠ `Empty()`). |
| CA-08 | Nenhuma dependência externa além da stdlib Go. |
