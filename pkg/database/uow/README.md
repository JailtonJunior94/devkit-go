# pkg/database/uow

O pacote `uow` fornece uma Unit of Work (`UnitOfWork[T]`) genérica e atômica para operações transacionais em banco de dados.

**ADRs:** [ADR-002](../../../tasks/prd-database-manager-uow-refactor/adr-002-dbtx-generics.md) · [ADR-004](../../../tasks/prd-database-manager-uow-refactor/adr-004-tx-propagation.md)  
**Tech Spec:** [techspec.md](../../../tasks/prd-database-manager-uow-refactor/techspec.md)

---

## Visão Geral

O `UnitOfWork[T]` executa uma função arbitrária dentro de uma transação de banco de dados e garante exatamente um resultado:

| Caminho | Resultado |
|---------|-----------|
| `fn` retorna `(valor, nil)` | `Commit` → retorna `(valor, nil)` |
| `fn` retorna `(_, err)` | `Rollback` → retorna `(zero, err)` |
| `fn` entra em pânico | `recover` → `Rollback` → `panic(original)` re-propagado |
| `ctx` cancelado durante `fn` | `Rollback` → retorna `(zero, ctx.Err())` |

Chamadas aninhadas (`Do` dentro de `Do` na mesma goroutine usando a mesma instância, ou quando o `ctx` já carrega uma transação ativa) retornam `database.ErrNestedTransaction` imediatamente, sem iniciar uma nova transação (RF-32).

---

## Início Rápido

### Resultado tipado

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
)

type User struct { ID int64; Name string }

uw := uow.New[User](mgr)

user, err := uw.Do(ctx, func(ctx context.Context, tx database.DBTX) (User, error) {
    row := tx.QueryRowContext(ctx,
        "INSERT INTO users(name) VALUES($1) RETURNING id, name", "Alice")
    var u User
    if err := row.Scan(&u.ID, &u.Name); err != nil {
        return User{}, err
    }
    return u, nil
})
```

### Void (sem valor de retorno)

Use `NewVoid` quando a função transacional não produz um resultado tipado:

```go
uw := uow.NewVoid(mgr)

_, err := uw.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
    _, err := tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, id)
    return struct{}{}, err
})
```

---

## Opções

| Opção | Padrão | Descrição |
|-------|--------|-----------|
| `WithIsolation(level)` | `sql.LevelDefault` | Default de nível de isolamento aplicado a cada chamada `Do`. Pode ser sobrescrito por chamada (RF-11). |
| `WithReadOnly(true)` | false | Default de modo somente leitura aplicado a cada chamada `Do`. Pode ser sobrescrito por chamada (RF-36). |

```go
uw := uow.New[Report](mgr,
    uow.WithIsolation(sql.LevelSerializable),
    uow.WithReadOnly(true),
)

report, err := uw.Do(ctx, fn,
    uow.WithIsolation(sql.LevelReadCommitted), // override apenas desta transação
)
```

---

## Propagação de Transação via context

O `Do` injeta a transação ativa no `ctx` antes de chamar a `fn` (propagação implícita ADR-004).
Chamadores subsequentes que recebem o `ctx` podem recuperar a transação diretamente:

```go
// Dentro de fn:
func(ctx context.Context, tx database.DBTX) (MyResult, error) {
    // tx e database.FromContext(ctx) são o mesmo DBTX aqui.
    callRepository(ctx) // o repositório pode usar database.FromContext(ctx) internamente
    return result, nil
}
```

Alternativamente, passe `tx` explicitamente como um parâmetro. Ambos os estilos são suportados; prefira o estilo via contexto para repositórios que apenas consomem `DBTX`.

### Manager.DBTX(ctx) dentro de fn

```go
// manager.DBTX(ctx) também respeita a tx no ctx:
dbtx := mgr.DBTX(ctx) // retorna a tx ativa, não uma conexão do pool
```

---

## Segurança contra Pânico (Panic Safety)

O pânico dentro de `fn` é totalmente contido:

```go
uw := uow.New[struct{}](mgr)

defer func() {
    if r := recover(); r != nil {
        // r é o valor original do pânico re-propagado após o rollback
        log.Printf("recuperado após pânico: %v", r)
    }
}()

_, _ = uw.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
    tx.ExecContext(ctx, "INSERT INTO audit_log(event) VALUES($1)", "start")
    panic("falha inesperada") // o rollback acontece automaticamente
})
```

O rollback utiliza um contexto desacoplado do cancelamento do chamador para garantir a tentativa de cleanup mesmo quando o `ctx` original já expirou.

---

## Proteção contra Transações Aninhadas

O `UnitOfWork[T]` não suporta transações aninhadas (RF-32). A detecção de aninhamento ocorre em dois níveis:

1. **Mesma instância:** se o `Do` for chamado de forma reentrante no mesmo `*unitOfWork`, ele retorna `database.ErrNestedTransaction` imediatamente.
2. **Entre instâncias:** se o `ctx` já carregar uma transação de outro `Do`, a nova chamada também retorna `database.ErrNestedTransaction`.

```go
outer := uow.New[struct{}](mgr)
inner := uow.New[int](mgr)

_, _ = outer.Do(ctx, func(ctx context.Context, _ database.DBTX) (struct{}, error) {
    n, err := inner.Do(ctx, ...) // retorna ErrNestedTransaction
    _ = n; _ = err
    return struct{}{}, nil
})
```

---

## Referência de Erros

| Sentinela | Significado |
|-----------|-------------|
| `database.ErrNestedTransaction` | `Do` aninhado detectado; nenhuma nova transação foi iniciada. |
| `database.ErrManagerClosed` | O Manager foi encerrado antes que o `Do` pudesse iniciar uma transação. |
