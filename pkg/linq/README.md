# linq

LINQ-style operations for Go slices using generics.

## Quick Start

```go
numbers := []int{1, 2, 3, 4, 5}

// Filter
evens := linq.Filter(numbers, func(n int) bool { return n%2 == 0 })
// [2, 4]

// Map
doubled := linq.Map(numbers, func(n int) int { return n * 2 })
// [2, 4, 6, 8, 10]

// Sum
total := linq.Sum(numbers, func(n int) float64 { return float64(n) })
// 15.0
```

## API

```go
Filter[T any](items []T, fn PredicateFunc[T]) []T
Find[T any](items []T, fn PredicateFunc[T]) T
Remove[T any](items []T, fn PredicateFunc[T]) []T
Map[I, O any](items []I, fn MapFunc[I, O]) []O
GroupBy[T any, K comparable](items []T, fn GroupByFunc[T, K]) map[K][]T
Sum[T any](items []T, fn SumFunc[T]) float64
```

## Examples

```go
type Product struct {
    Name  string
    Price float64
}

products := []Product{
    {"Apple", 1.50},
    {"Banana", 0.75},
    {"Orange", 2.00},
}

// Filter expensive products
expensive := linq.Filter(products, func(p Product) bool {
    return p.Price > 1.00
})

// Map to prices
prices := linq.Map(products, func(p Product) float64 {
    return p.Price
})

// Total price
total := linq.Sum(products, func(p Product) float64 {
    return p.Price
})
```
