package linq

// PredicateFunc define uma função que testa uma condição sobre um elemento.
// Retorna true se o elemento satisfaz a condição, false caso contrário.
type PredicateFunc[T any] func(T) bool

// MapFunc define uma função que transforma um elemento do tipo I para o tipo O.
type MapFunc[I, O any] func(I) O

// GroupByFunc define uma função que extrai uma chave comparável de um elemento.
type GroupByFunc[T any, K comparable] func(T) K

// SumFunc define uma função que converte um elemento para um valor numérico float64.
type SumFunc[T any] func(T) float64

// Filter retorna um novo slice contendo apenas os elementos que satisfazem o predicado fn.
// Não modifica o slice original. É seguro para uso concorrente desde que o slice
// original não seja modificado por outras goroutines.
//
// Retorna nil se items for nil, ou um slice vazio se nenhum elemento satisfizer o predicado.
//
// Exemplo:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	evens := linq.Filter(numbers, func(n int) bool { return n%2 == 0 })
//	// evens = []int{2, 4}
func Filter[T any](items []T, fn PredicateFunc[T]) []T {
	if items == nil {
		return nil
	}

	var result []T
	for _, item := range items {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Find retorna o primeiro elemento que satisfaz o predicado fn.
// Retorna o valor zero do tipo T se nenhum elemento for encontrado ou se items for nil.
// Não modifica o slice original.
//
// Exemplo:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	found := linq.Find(numbers, func(n int) bool { return n > 3 })
//	// found = 4
func Find[T any](items []T, fn PredicateFunc[T]) T {
	var empty T
	if items == nil {
		return empty
	}

	for _, item := range items {
		if fn(item) {
			return item
		}
	}
	return empty
}

// Remove retorna um novo slice sem os elementos que satisfazem o predicado fn.
// Não modifica o slice original. É seguro para uso concorrente desde que o slice
// original não seja modificado por outras goroutines.
//
// Retorna nil se items for nil, ou um slice vazio se todos os elementos forem removidos.
//
// Exemplo:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	filtered := linq.Remove(numbers, func(n int) bool { return n > 3 })
//	// filtered = []int{1, 2, 3}
func Remove[T any](items []T, fn PredicateFunc[T]) []T {
	if items == nil {
		return nil
	}

	var result []T
	for _, item := range items {
		if !fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforma cada elemento do slice usando a função fn e retorna um novo slice
// com os elementos transformados. Não modifica o slice original.
// É seguro para uso concorrente desde que o slice original não seja modificado por outras goroutines.
//
// Retorna nil se items for nil.
//
// Exemplo:
//
//	numbers := []int{1, 2, 3}
//	doubled := linq.Map(numbers, func(n int) int { return n * 2 })
//	// doubled = []int{2, 4, 6}
func Map[I, O any](items []I, fn MapFunc[I, O]) []O {
	if items == nil {
		return nil
	}

	result := make([]O, len(items))
	for index, item := range items {
		result[index] = fn(item)
	}
	return result
}

// GroupBy agrupa os elementos do slice por uma chave extraída pela função fn.
// Retorna um mapa onde as chaves são os valores retornados por fn e os valores
// são slices com os elementos que compartilham a mesma chave.
// Não modifica o slice original.
//
// Retorna um mapa vazio se items for nil ou vazio.
//
// Exemplo:
//
//	type Person struct { Name string; Age int }
//	people := []Person{{"Alice", 25}, {"Bob", 25}, {"Charlie", 30}}
//	byAge := linq.GroupBy(people, func(p Person) int { return p.Age })
//	// byAge = map[int][]Person{25: {{"Alice", 25}, {"Bob", 25}}, 30: {{"Charlie", 30}}}
func GroupBy[T any, K comparable](items []T, fn GroupByFunc[T, K]) map[K][]T {
	grouped := make(map[K][]T)

	if items == nil {
		return grouped
	}

	for _, item := range items {
		key := fn(item)
		grouped[key] = append(grouped[key], item)
	}
	return grouped
}

// Sum calcula a soma de todos os elementos do slice usando a função fn para
// converter cada elemento em um valor float64.
// Retorna 0 se items for nil ou vazio.
//
// Exemplo:
//
//	type Product struct { Name string; Price float64 }
//	products := []Product{{"A", 10.5}, {"B", 20.3}, {"C", 5.2}}
//	total := linq.Sum(products, func(p Product) float64 { return p.Price })
//	// total = 36.0
func Sum[T any](items []T, fn SumFunc[T]) float64 {
	var sum float64

	if items == nil {
		return sum
	}

	for _, item := range items {
		sum += fn(item)
	}
	return sum
}
