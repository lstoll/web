package gobson

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"testing"
)

// Generate test data map with alternating Person and Address pointers
func generateTestData(count int) map[string]any {
	data := make(map[string]any, count)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("item_%d", i)
		if i%2 == 0 {
			data[key] = &Person{Name: fmt.Sprintf("Name_%d", i), Age: 30 + i}
		} else {
			data[key] = &Address{Street: fmt.Sprintf("%d Main St", i), City: fmt.Sprintf("City_%d", i)}
		}
	}
	return data
}

var (
	testData1  map[string]any
	testData5  map[string]any
	testData20 map[string]any

	// Use DynamicMap type for JSON benchmarks
	dynamicTestData1  DynamicMap
	dynamicTestData5  DynamicMap
	dynamicTestData20 DynamicMap
)

// Initialize and Register types ONCE
func init() {
	log.Println("Initializing benchmark data and registering types...")
	// Register types for both JSON dynamic and Gob
	Register("person", Person{})
	Register("address", Address{})

	gob.Register(Person{})
	gob.Register(Address{})

	// Generate test data
	testData1 = generateTestData(1)
	testData5 = generateTestData(5)
	testData20 = generateTestData(20)

	// Prepare DynamicMap versions for JSON benchmarks
	dynamicTestData1 = DynamicMap(testData1)
	dynamicTestData5 = DynamicMap(testData5)
	dynamicTestData20 = DynamicMap(testData20)
	log.Println("Initialization complete.")
}

// --- Benchmark Functions ---

// -- Custom JSON Dynamic Benchmarks --

func benchmarkJSONDynamic(b *testing.B, data DynamicMap) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf) // Create encoder once if possible
	decoder := json.NewDecoder(&buf) // Create decoder once if possible

	var targetMap DynamicMap // Declare target map outside loop
	for i := 0; i < b.N; i++ {
		buf.Reset()

		// Marshal
		err := encoder.Encode(data)
		if err != nil {
			b.Fatalf("JSON Marshal failed: %v", err)
		}

		// Unmarshal
		// Reset target map explicitly if needed, or re-declare inside
		// targetMap = make(DynamicMap) // Could reallocate here
		err = decoder.Decode(&targetMap)
		if err != nil {
			b.Fatalf("JSON Unmarshal failed: %v", err)
		}
		// Prevent compiler optimization by checking a value (optional but good practice)
		// if _, ok := targetMap["item_0"]; !ok && len(data) > 0 {
		//     b.Fatalf("Expected item_0 not found after unmarshal")
		// }
	}
	// Optional: Assign to a global volatile variable to ensure usage
	// _ = targetMap
}

func BenchmarkJSONDynamic_1(b *testing.B) {
	benchmarkJSONDynamic(b, dynamicTestData1)
}

func BenchmarkJSONDynamic_5(b *testing.B) {
	benchmarkJSONDynamic(b, dynamicTestData5)
}

func BenchmarkJSONDynamic_20(b *testing.B) {
	benchmarkJSONDynamic(b, dynamicTestData20)
}

// -- Gob Benchmarks --

func benchmarkGob(b *testing.B, data map[string]any) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf) // Create encoder once if possible
	decoder := gob.NewDecoder(&buf) // Create decoder once if possible
	var targetMap map[string]any    // Declare target map outside loop

	for i := 0; i < b.N; i++ {
		buf.Reset() // Reset buffer for each iteration

		// Encode (Marshal)
		err := encoder.Encode(data)
		if err != nil {
			b.Fatalf("Gob Encode failed: %v", err)
		}

		// Decode (Unmarshal)
		// targetMap = make(map[string]any) // Could reallocate here
		err = decoder.Decode(&targetMap)
		if err != nil {
			b.Fatalf("Gob Decode failed: %v", err)
		}
		// Prevent compiler optimization
		// if _, ok := targetMap["item_0"]; !ok && len(data) > 0 {
		//     b.Fatalf("Expected item_0 not found after decode")
		// }
	}
	// _ = targetMap
}

func BenchmarkGob_1(b *testing.B) {
	benchmarkGob(b, testData1)
}

func BenchmarkGob_5(b *testing.B) {
	benchmarkGob(b, testData5)
}

func BenchmarkGob_20(b *testing.B) {
	benchmarkGob(b, testData20)
}
