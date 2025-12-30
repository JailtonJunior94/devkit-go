package linq

import (
	"reflect"
	"sync"
	"testing"
)

// Test Filter function.
func TestFilter(t *testing.T) {
	t.Run("filters even numbers", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6}
		result := Filter(numbers, func(n int) bool { return n%2 == 0 })
		expected := []int{2, 4, 6}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Filter() = %v, want %v", result, expected)
		}
	})

	t.Run("returns empty slice when no matches", func(t *testing.T) {
		numbers := []int{1, 3, 5}
		result := Filter(numbers, func(n int) bool { return n%2 == 0 })

		if len(result) != 0 {
			t.Errorf("Filter() = %v, want empty slice", result)
		}
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var numbers []int
		result := Filter(numbers, func(n int) bool { return n%2 == 0 })

		if result != nil {
			t.Errorf("Filter(nil) = %v, want nil", result)
		}
	})

	t.Run("returns all when all match", func(t *testing.T) {
		numbers := []int{2, 4, 6, 8}
		result := Filter(numbers, func(n int) bool { return n%2 == 0 })

		if !reflect.DeepEqual(result, numbers) {
			t.Errorf("Filter() = %v, want %v", result, numbers)
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		original := []int{1, 2, 3, 4, 5}
		backup := make([]int, len(original))
		copy(backup, original)

		Filter(original, func(n int) bool { return n%2 == 0 })

		if !reflect.DeepEqual(original, backup) {
			t.Errorf("Filter() modified original slice: got %v, want %v", original, backup)
		}
	})

	t.Run("works with strings", func(t *testing.T) {
		words := []string{"apple", "banana", "apricot", "cherry"}
		result := Filter(words, func(s string) bool { return s[0] == 'a' })
		expected := []string{"apple", "apricot"}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Filter() = %v, want %v", result, expected)
		}
	})
}

// Test Find function.
func TestFind(t *testing.T) {
	t.Run("finds first matching element", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		result := Find(numbers, func(n int) bool { return n > 3 })

		if result != 4 {
			t.Errorf("Find() = %v, want 4", result)
		}
	})

	t.Run("returns zero value when no match", func(t *testing.T) {
		numbers := []int{1, 2, 3}
		result := Find(numbers, func(n int) bool { return n > 10 })

		if result != 0 {
			t.Errorf("Find() = %v, want 0", result)
		}
	})

	t.Run("returns zero value for nil input", func(t *testing.T) {
		var numbers []int
		result := Find(numbers, func(n int) bool { return n > 0 })

		if result != 0 {
			t.Errorf("Find(nil) = %v, want 0", result)
		}
	})

	t.Run("finds first element in multiple matches", func(t *testing.T) {
		numbers := []int{1, 5, 3, 7, 9}
		result := Find(numbers, func(n int) bool { return n > 4 })

		if result != 5 {
			t.Errorf("Find() = %v, want 5 (first match)", result)
		}
	})

	t.Run("works with structs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		people := []Person{
			{"Alice", 25},
			{"Bob", 30},
			{"Charlie", 35},
		}

		result := Find(people, func(p Person) bool { return p.Age > 28 })

		if result.Name != "Bob" {
			t.Errorf("Find() = %v, want Bob", result.Name)
		}
	})
}

// Test Remove function.
func TestRemove(t *testing.T) {
	t.Run("removes matching elements", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		result := Remove(numbers, func(n int) bool { return n > 3 })
		expected := []int{1, 2, 3}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Remove() = %v, want %v", result, expected)
		}
	})

	t.Run("removes multiple matching elements", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6, 7, 8}
		result := Remove(numbers, func(n int) bool { return n%2 == 0 })
		expected := []int{1, 3, 5, 7}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Remove() = %v, want %v", result, expected)
		}
	})

	t.Run("returns empty slice when all removed", func(t *testing.T) {
		numbers := []int{2, 4, 6}
		result := Remove(numbers, func(n int) bool { return n%2 == 0 })

		if len(result) != 0 {
			t.Errorf("Remove() = %v, want empty slice", result)
		}
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var numbers []int
		result := Remove(numbers, func(n int) bool { return n > 0 })

		if result != nil {
			t.Errorf("Remove(nil) = %v, want nil", result)
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		original := []int{1, 2, 3, 4, 5}
		backup := make([]int, len(original))
		copy(backup, original)

		Remove(original, func(n int) bool { return n > 3 })

		if !reflect.DeepEqual(original, backup) {
			t.Errorf("Remove() modified original slice: got %v, want %v", original, backup)
		}
	})

	t.Run("returns all when none match", func(t *testing.T) {
		numbers := []int{1, 3, 5}
		result := Remove(numbers, func(n int) bool { return n%2 == 0 })

		if !reflect.DeepEqual(result, numbers) {
			t.Errorf("Remove() = %v, want %v", result, numbers)
		}
	})
}

// Test Map function.
func TestMap(t *testing.T) {
	t.Run("maps numbers to doubled values", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4}
		result := Map(numbers, func(n int) int { return n * 2 })
		expected := []int{2, 4, 6, 8}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Map() = %v, want %v", result, expected)
		}
	})

	t.Run("maps int to string", func(t *testing.T) {
		numbers := []int{1, 2, 3}
		result := Map(numbers, func(n int) string { return string(rune(n + 64)) })
		expected := []string{"A", "B", "C"}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Map() = %v, want %v", result, expected)
		}
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var numbers []int
		result := Map(numbers, func(n int) int { return n * 2 })

		if result != nil {
			t.Errorf("Map(nil) = %v, want nil", result)
		}
	})

	t.Run("preserves length", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		result := Map(numbers, func(n int) int { return n * 2 })

		if len(result) != len(numbers) {
			t.Errorf("Map() length = %v, want %v", len(result), len(numbers))
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		original := []int{1, 2, 3}
		backup := make([]int, len(original))
		copy(backup, original)

		Map(original, func(n int) int { return n * 2 })

		if !reflect.DeepEqual(original, backup) {
			t.Errorf("Map() modified original slice: got %v, want %v", original, backup)
		}
	})

	t.Run("works with structs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		people := []Person{{"Alice", 25}, {"Bob", 30}}
		result := Map(people, func(p Person) string { return p.Name })
		expected := []string{"Alice", "Bob"}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Map() = %v, want %v", result, expected)
		}
	})
}

// Test GroupBy function.
func TestGroupBy(t *testing.T) {
	t.Run("groups numbers by even/odd", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6}
		result := GroupBy(numbers, func(n int) bool { return n%2 == 0 })

		if len(result[true]) != 3 || len(result[false]) != 3 {
			t.Errorf("GroupBy() = %v, want 3 evens and 3 odds", result)
		}
	})

	t.Run("groups strings by first letter", func(t *testing.T) {
		words := []string{"apple", "apricot", "banana", "blueberry", "cherry"}
		result := GroupBy(words, func(s string) rune { return rune(s[0]) })

		if len(result['a']) != 2 || len(result['b']) != 2 || len(result['c']) != 1 {
			t.Errorf("GroupBy() = %v, incorrect grouping", result)
		}
	})

	t.Run("returns empty map for nil input", func(t *testing.T) {
		var numbers []int
		result := GroupBy(numbers, func(n int) int { return n })

		if result == nil || len(result) != 0 {
			t.Errorf("GroupBy(nil) = %v, want empty map", result)
		}
	})

	t.Run("returns empty map for empty slice", func(t *testing.T) {
		numbers := []int{}
		result := GroupBy(numbers, func(n int) int { return n })

		if len(result) != 0 {
			t.Errorf("GroupBy([]) = %v, want empty map", result)
		}
	})

	t.Run("works with structs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		people := []Person{
			{"Alice", 25},
			{"Bob", 25},
			{"Charlie", 30},
			{"David", 30},
		}

		result := GroupBy(people, func(p Person) int { return p.Age })

		if len(result[25]) != 2 || len(result[30]) != 2 {
			t.Errorf("GroupBy() = %v, incorrect grouping", result)
		}
	})
}

// Test Sum function.
func TestSum(t *testing.T) {
	t.Run("sums integers", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		result := Sum(numbers, func(n int) float64 { return float64(n) })

		if result != 15.0 {
			t.Errorf("Sum() = %v, want 15.0", result)
		}
	})

	t.Run("returns 0 for nil input", func(t *testing.T) {
		var numbers []int
		result := Sum(numbers, func(n int) float64 { return float64(n) })

		if result != 0 {
			t.Errorf("Sum(nil) = %v, want 0", result)
		}
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		numbers := []int{}
		result := Sum(numbers, func(n int) float64 { return float64(n) })

		if result != 0 {
			t.Errorf("Sum([]) = %v, want 0", result)
		}
	})

	t.Run("works with floats", func(t *testing.T) {
		numbers := []float64{1.5, 2.5, 3.0}
		result := Sum(numbers, func(n float64) float64 { return n })
		expected := 7.0

		if result != expected {
			t.Errorf("Sum() = %v, want %v", result, expected)
		}
	})

	t.Run("works with structs", func(t *testing.T) {
		type Product struct {
			Name  string
			Price float64
		}
		products := []Product{
			{"A", 10.5},
			{"B", 20.3},
			{"C", 5.2},
		}

		result := Sum(products, func(p Product) float64 { return p.Price })
		expected := 36.0

		if result != expected {
			t.Errorf("Sum() = %v, want %v", result, expected)
		}
	})
}

// Test concurrent safety - ensures no race conditions.
func TestConcurrentSafety(t *testing.T) {
	t.Run("Filter is safe with concurrent reads", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Filter(numbers, func(n int) bool { return n%2 == 0 })
			}()
		}

		wg.Wait()
	})

	t.Run("Map is safe with concurrent reads", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Map(numbers, func(n int) int { return n * 2 })
			}()
		}

		wg.Wait()
	})

	t.Run("Remove is safe with concurrent reads", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Remove(numbers, func(n int) bool { return n > 5 })
			}()
		}

		wg.Wait()
	})

	t.Run("GroupBy is safe with concurrent reads", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				GroupBy(numbers, func(n int) bool { return n%2 == 0 })
			}()
		}

		wg.Wait()
	})
}

// Benchmarks.
func BenchmarkFilter(b *testing.B) {
	numbers := make([]int, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Filter(numbers, func(n int) bool { return n%2 == 0 })
	}
}

func BenchmarkMap(b *testing.B) {
	numbers := make([]int, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Map(numbers, func(n int) int { return n * 2 })
	}
}

func BenchmarkRemove(b *testing.B) {
	numbers := make([]int, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Remove(numbers, func(n int) bool { return n > 500 })
	}
}

func BenchmarkGroupBy(b *testing.B) {
	numbers := make([]int, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GroupBy(numbers, func(n int) int { return n % 10 })
	}
}

func BenchmarkSum(b *testing.B) {
	numbers := make([]int, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Sum(numbers, func(n int) float64 { return float64(n) })
	}
}
