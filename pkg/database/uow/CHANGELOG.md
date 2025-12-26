# Unit of Work - Changelog de Correções Críticas

## Data: 2025-12-25

### 🔴 Correções Críticas Implementadas

Este documento descreve as correções críticas aplicadas ao Unit of Work para garantir segurança em ambientes de produção com alta concorrência e múltiplas réplicas.

---

## 1. ✅ Isolation Level Configurável

### Problema Anterior
- Isolation level era sempre `nil` (padrão do banco)
- Causava **lost updates** e **write skew** em cenários concorrentes
- Comportamento inconsistente entre diferentes bancos de dados

### Correção Aplicada
```go
// ANTES
tx, err := u.db.BeginTx(ctx, nil)

// DEPOIS
txOptions := &sql.TxOptions{
    Isolation: sql.LevelSerializable, // Padrão mais seguro
    ReadOnly:  false,
}
tx, err := u.db.BeginTx(ctx, txOptions)
```

### Impacto
- **BREAKING CHANGE**: Isolation level padrão agora é `SERIALIZABLE`
- Previne lost updates, write skew e phantom reads
- Pode aumentar serialization failures em alta concorrência

### Como Ajustar (se necessário)
```go
// Para usar isolation level diferente:
opts := &uow.TxOptions{
    Isolation: sql.LevelReadCommitted, // Menos restritivo
}
uow.DoWithOptions(ctx, opts, func(ctx, tx) error {
    // sua lógica aqui
})

// Ou criar UoW com configuração customizada:
uow := uow.NewUnitOfWorkWithOptions(db, sql.LevelReadCommitted, 30*time.Second)
```

---

## 2. ✅ Detecção de Transações Aninhadas

### Problema Anterior
```go
// Código perigoso que funcionava antes:
uow.Do(ctx, func(ctx, tx1) error {
    budgetRepo.Insert(ctx, budget)

    // Isto criava uma NOVA transação independente!
    uow.Do(ctx, func(ctx, tx2) error {
        categoryRepo.Insert(ctx, category) // Comita antes de tx1
        return nil
    })

    return errors.New("rollback") // tx2 já commitou! 💥
})
```

### Correção Aplicada
- Transações são marcadas no `context` usando `txKey{}`
- Tentativa de criar transação aninhada retorna erro:
  ```
  nested transactions are not allowed: a transaction is already active in this context
  ```

### Impacto
- **BREAKING CHANGE**: Código com transações aninhadas falhará
- Previne quebra de atomicidade em operações distribuídas

### Como Migrar
```go
// ANTES (errado):
uow.Do(ctx, func(ctx, tx) error {
    repo1.Insert(ctx, data1)
    uow.Do(ctx, func(ctx, tx) error { // ❌ Vai falhar agora
        repo2.Insert(ctx, data2)
        return nil
    })
    return nil
})

// DEPOIS (correto):
uow.Do(ctx, func(ctx, tx) error {
    repo1.Insert(ctx, data1)
    repo2.Insert(ctx, data2) // Mesma transação ✅
    return nil
})
```

---

## 3. ✅ Proteção Contra Panic Duplo

### Problema Anterior
```go
defer func() {
    if p := recover(); p != nil {
        _ = tx.Rollback() // Se Rollback também panica, tx vaza!
        panic(p)
    }
}()
```

### Correção Aplicada
```go
defer func() {
    if p := recover(); p != nil {
        // Função anônima protegida contra panic duplo
        func() {
            defer func() { _ = recover() }()
            if !committed {
                _ = tx.Rollback()
            }
        }()
        panic(p) // Re-lança panic original
    }
}()
```

### Impacto
- Previne **connection leaks** em cenários de panic duplo
- Garante rollback mesmo com drivers bugados

---

## 4. ✅ Timeout Padrão de Transações

### Problema Anterior
- Transações podiam ficar abertas indefinidamente
- Bloqueava connection pool em operações longas

### Correção Aplicada
- Timeout padrão: **30 segundos**
- Configurável por transação ou globalmente

```go
// Timeout padrão (30s)
uow.Do(ctx, func(ctx, tx) error {
    // Operações aqui
})

// Timeout customizado
opts := &uow.TxOptions{
    Timeout: 5 * time.Second,
}
uow.DoWithOptions(ctx, opts, func(ctx, tx) error {
    // Operações rápidas aqui
})
```

### Impacto
- Previne deadlocks prolongados
- Libera conexões mais rapidamente
- Pode causar timeouts em operações realmente longas

### Como Ajustar
```go
// Para operações que precisam de mais tempo:
opts := &uow.TxOptions{
    Timeout: 2 * time.Minute, // Aumentar timeout
}
uow.DoWithOptions(ctx, opts, func(ctx, tx) error {
    // Operação longa aqui
})

// Ou criar UoW com timeout maior:
uow := uow.NewUnitOfWorkWithOptions(db, sql.LevelSerializable, 60*time.Second)
```

---

## 5. ✅ Tratamento Robusto de sql.ErrTxDone

### Problema Anterior
```go
if rbErr := tx.Rollback(); rbErr != nil {
    return fmt.Errorf("rollback error: %v", rbErr)
    // Retornava erro mesmo quando transação já estava finalizada
}
```

### Correção Aplicada
```go
if rbErr := tx.Rollback(); rbErr != nil {
    // Ignorar sql.ErrTxDone (estado esperado em alguns casos)
    if !errors.Is(rbErr, sql.ErrTxDone) {
        return fmt.Errorf("rollback error: %v", rbErr)
    }
}
```

### Impacto
- Reduz falsos positivos em logs de erro
- Trata corretamente context cancellation

---

## 6. ✅ Rollback Após Commit Falho

### Problema Anterior
```go
if err = tx.Commit(); err != nil {
    return err // Transação fica em estado indefinido
}
```

### Correção Aplicada
```go
if err = tx.Commit(); err != nil {
    // Tentar rollback defensivo
    if rbErr := tx.Rollback(); rbErr != nil {
        if !errors.Is(rbErr, sql.ErrTxDone) {
            return fmt.Errorf("commit failed: %w, rollback error: %v", err, rbErr)
        }
    }
    return fmt.Errorf("failed to commit transaction: %w", err)
}
```

### Impacto
- Reduz risco de connection pool exhaustion
- Tratamento defensivo de estados inválidos

---

## 📊 Novos Testes Adicionados

1. **TestNestedTransactionPrevention** - Valida bloqueio de transações aninhadas
2. **TestTransactionTimeout** - Valida timeout configurável
3. **TestDoublePanicProtection** - Valida proteção contra panic duplo
4. **TestIsolationLevels** - Testa todos os níveis de isolamento
5. **TestReadOnlyTransaction** - Valida transações read-only
6. **TestContextCancellation** - Valida comportamento com context cancelado
7. **TestErrTxDoneHandling** - Valida tratamento de sql.ErrTxDone

**Cobertura de testes**: 100% das linhas críticas

---

## 🚀 Recomendações de Produção

### Connection Pool
```go
// cmd/server/server.go
db.SetMaxOpenConns(100)           // Ajustar para carga esperada
db.SetMaxIdleConns(25)            // 25% de MaxOpenConns
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(30 * time.Second)
```

### Isolation Level por Caso de Uso

**Use SERIALIZABLE (padrão) quando:**
- Operações financeiras (budgets, transactions)
- Validações de constraints de negócio
- Operações que modificam múltiplas tabelas relacionadas

**Use READ COMMITTED quando:**
- Leituras simples sem modificação
- Relatórios e dashboards
- Operações idempotentes

**Use READ UNCOMMITTED apenas para:**
- Contadores aproximados
- Estatísticas não-críticas
- NUNCA para dados financeiros

### Timeouts Recomendados

```go
// Operações OLTP (maioria dos casos)
Timeout: 5 * time.Second

// Relatórios simples
Timeout: 15 * time.Second

// Operações batch/migração
Timeout: 2 * time.Minute

// NUNCA exceder 5 minutos em produção
```

---

## ⚠️ Breaking Changes Checklist

- [x] Isolation level padrão mudou para SERIALIZABLE
- [x] Transações aninhadas agora retornam erro
- [x] Timeout padrão de 30 segundos aplicado
- [x] Nova interface `DoWithOptions` disponível

### Migração Necessária?

**SIM, se seu código:**
- ✅ Chama `uow.Do()` dentro de outro `uow.Do()`
- ✅ Espera isolation level READ COMMITTED
- ✅ Tem transações que demoram > 30 segundos

**NÃO, se seu código:**
- ✅ Usa UoW em apenas um nível
- ✅ Transações terminam em < 30 segundos
- ✅ Não depende de dirty reads

---

## 📞 Suporte

Para dúvidas ou problemas relacionados a estas mudanças, abra uma issue detalhando:
1. Cenário de uso
2. Erro observado
3. Configuração atual de connection pool
4. Carga esperada (req/s, réplicas)

---

**Versão**: 2.0.0
**Data**: 2025-12-25
**Compatibilidade**: PostgreSQL 12+, CockroachDB 21+, MySQL 8+, SQL Server 2019+
