package gobson

import (
	"encoding/json"
	"reflect"
	"testing"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

func TestDynamicMap_RoundTrip_Comprehensive(t *testing.T) {
	ResetRegistry() // Ensure clean state for this test
	Register("person", Person{})
	Register("address", Address{})

	originalData := DynamicMap{
		"user1":    Person{Name: "Alice", Age: 30},                              // Registered struct value
		"user2":    &Person{Name: "Bob", Age: 40},                               // Registered struct pointer
		"loc1":     Address{Street: "123 Main St", City: "Anytown"},             // Registered struct value
		"loc2":     &Address{Street: "456 Side St", City: "Otherville"},         // Registered struct pointer
		"id":       "xyz-123",                                                   // Basic type: string
		"count":    float64(101),                                                // Basic type: number (becomes float64)
		"enabled":  true,                                                        // Basic type: bool
		"flags":    []any{"A", float64(2), true},                                // Slice of any
		"nested":   map[string]any{"value": float64(99), "label": "nested_map"}, // Nested map
		"nullable": nil,                                                         // Null value
	}

	// 1. Marshal
	jsonData, err := json.MarshalIndent(originalData, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	// fmt.Println("Marshalled JSON:\n", string(jsonData)) // For debugging

	// 2. Unmarshal
	var unmarshaledData DynamicMap
	err = json.Unmarshal(jsonData, &unmarshaledData)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v\nJSON was:\n%s", err, string(jsonData))
	}

	// 3. Compare
	// Use reflect.DeepEqual for comparison. It handles pointers vs values,
	// slices, maps, and basic types correctly. It also handles the
	// number type difference (int vs float64) for non-registered types well.
	if !reflect.DeepEqual(originalData, unmarshaledData) {
		t.Errorf("Round trip failed. Data mismatch.")
		// Provide detailed diff (requires external library like go-spew or custom diff func)
		// Example using simple fmt Printf for illustration:
		for k, vOrig := range originalData {
			vNew, ok := unmarshaledData[k]
			if !ok {
				t.Errorf("Key '%s' missing after unmarshal.", k)
				continue
			}
			if !reflect.DeepEqual(vOrig, vNew) {
				t.Errorf("Mismatch for key '%s':\nOriginal: (%T) %+v\nNew:      (%T) %+v",
					k, vOrig, vOrig, vNew, vNew)
			}
		}
		for k := range unmarshaledData {
			if _, ok := originalData[k]; !ok {
				t.Errorf("Extra key '%s' found after unmarshal.", k)
			}
		}
	}

	// 4. Verify types explicitly for registered items
	if _, ok := unmarshaledData["user1"].(Person); !ok {
		t.Errorf("Expected 'user1' to be Person, got %T", unmarshaledData["user1"])
	}
	// Note: Pointers become values after unmarshal in this implementation
	if _, ok := unmarshaledData["user2"].(Person); !ok {
		t.Errorf("Expected 'user2' (originally *Person) to be Person, got %T", unmarshaledData["user2"])
	}
	if _, ok := unmarshaledData["loc1"].(Address); !ok {
		t.Errorf("Expected 'loc1' to be Address, got %T", unmarshaledData["loc1"])
	}
	if _, ok := unmarshaledData["loc2"].(Address); !ok {
		t.Errorf("Expected 'loc2' (originally *Address) to be Address, got %T", unmarshaledData["loc2"])
	}
	// Check a basic type
	if v, ok := unmarshaledData["count"].(float64); !ok || v != 101 {
		t.Errorf("Expected 'count' to be float64(101), got %T(%v)", unmarshaledData["count"], unmarshaledData["count"])
	}
	// Check nil
	if unmarshaledData["nullable"] != nil {
		t.Errorf("Expected 'nullable' to be nil, got %T(%v)", unmarshaledData["nullable"], unmarshaledData["nullable"])
	}
}

func TestDynamicMap_Empty(t *testing.T) {
	ResetRegistry()
	originalData := DynamicMap{}

	// Marshal
	jsonData, err := json.Marshal(originalData)
	if err != nil {
		t.Fatalf("Marshal failed for empty map: %v", err)
	}
	expectedJSON := "{}"
	if string(jsonData) != expectedJSON {
		t.Errorf("Expected JSON '%s' for empty map, got '%s'", expectedJSON, string(jsonData))
	}

	// Unmarshal
	var unmarshaledData DynamicMap
	err = json.Unmarshal(jsonData, &unmarshaledData)
	if err != nil {
		t.Fatalf("Unmarshal failed for empty map JSON: %v", err)
	}
	if unmarshaledData == nil {
		t.Errorf("Unmarshaled map should not be nil, should be empty map")
	}
	if len(unmarshaledData) != 0 {
		t.Errorf("Unmarshaled map should be empty, got size %d", len(unmarshaledData))
	}
}

func TestDynamicMap_NullJSON(t *testing.T) {
	ResetRegistry()
	jsonData := []byte("null")

	var unmarshaledData DynamicMap // Target is non-nil map variable, but holds nil value
	err := json.Unmarshal(jsonData, &unmarshaledData)
	if err != nil {
		t.Fatalf("Unmarshal failed for null JSON: %v", err)
	}
	if unmarshaledData != nil {
		t.Errorf("Unmarshaling 'null' JSON should result in a nil map, got: %#v", unmarshaledData)
	}
}

func TestDynamicMap_CheckJSONStructure(t *testing.T) {
	ResetRegistry()
	Register("person", Person{})

	t.Run("BasicTypesOnly", func(t *testing.T) {
		data := DynamicMap{
			"name":  "Test",
			"value": 123.45,
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		// Check structure - should NOT have __type__ wrappers
		var check map[string]any
		if err := json.Unmarshal(jsonData, &check); err != nil {
			t.Fatalf("Failed to unmarshal result for checking: %v", err)
		}
		if _, ok := check["name"].(string); !ok {
			t.Errorf("Expected 'name' to be string, got %T", check["name"])
		}
		if _, ok := check["value"].(float64); !ok {
			t.Errorf("Expected 'value' to be float64, got %T", check["value"])
		}
	})

	t.Run("RegisteredTypesOnly", func(t *testing.T) {
		data := DynamicMap{
			"p1": Person{Name: "Reg", Age: 1},
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		// Check structure - MUST have __type__ wrappers
		var check map[string]map[string]any // Expecting nested map structure
		if err := json.Unmarshal(jsonData, &check); err != nil {
			t.Fatalf("Failed to unmarshal result for checking structure: %v\nJSON: %s", err, string(jsonData))
		}

		p1Wrapper, ok := check["p1"]
		if !ok {
			t.Fatalf("Key 'p1' missing in marshaled structure check")
		}
		typeFieldVal, typeOk := p1Wrapper[typeField].(string)
		valueFieldVal, valueOk := p1Wrapper[valueField].(map[string]any) // Value should be map

		if !typeOk || typeFieldVal != "person" {
			t.Errorf("Expected '%s' field to be 'person', got: %v (%T)", typeField, p1Wrapper[typeField], p1Wrapper[typeField])
		}
		if !valueOk {
			t.Errorf("Expected '%s' field to be map[string]any, got: %v (%T)", valueField, p1Wrapper[valueField], p1Wrapper[valueField])
		} else {
			// Check inner value structure
			if name, nameOk := valueFieldVal["name"].(string); !nameOk || name != "Reg" {
				t.Errorf("Incorrect name in __value__")
			}
		}
	})
}

func TestUnmarshal_UnknownType(t *testing.T) {
	ResetRegistry() // Ensure 'widget' is not registered

	// JSON containing a type not present in the registry
	jsonData := []byte(`{
		"item1": {
			"__type__": "widget",
			"__value__": { "id": 99, "color": "red" }
		}
	}`)

	var unmarshaledData DynamicMap
	err := json.Unmarshal(jsonData, &unmarshaledData)
	if err != nil {
		// We might expect an error OR fallback handling depending on implementation
		// Current implementation logs warning and treats __value__ generically
		t.Logf("Unmarshal handled unknown type (expected warning in logs): %v", err) // Allow no error
	}

	item1, ok := unmarshaledData["item1"]
	if !ok {
		t.Fatalf("Key 'item1' missing after unmarshal")
	}

	// Check that the resulting type is the generic fallback (map[string]any)
	// because "widget" was not registered.
	if _, ok := item1.(map[string]any); !ok {
		t.Errorf("Expected 'item1' for unknown type to be map[string]any, got %T", item1)
	} else {
		// Optionally check the contents of the fallback map
		fallbackMap := item1.(map[string]any)
		if id, idOk := fallbackMap["id"].(float64); !idOk || id != 99 {
			t.Errorf("Incorrect ID in fallback map")
		}
		if color, colorOk := fallbackMap["color"].(string); !colorOk || color != "red" {
			t.Errorf("Incorrect color in fallback map")
		}
	}
}

func TestUnmarshal_InvalidJSON(t *testing.T) {
	ResetRegistry()
	Register("person", Person{})

	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{"Malformed JSON", `{ "key": "value }`, true},
		{"Wrong type in wrapper", `{ "person1": {"__type__": 123, "__value__": {}} }`, true},
		{"Value doesn't match struct", `{ "person1": {"__type__": "person", "__value__": {"name": "Joe", "age": "invalid_age"}} }`, true},
		{"Missing __value__", `{ "person1": {"__type__": "person"} }`, true}, // Or handle as error / fallback
		{"Not an object", `[1, 2, 3]`, true},                                 // Cannot unmarshal array into map
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var unmarshaledData DynamicMap
			err := json.Unmarshal([]byte(tc.jsonData), &unmarshaledData)
			if tc.expectError && err == nil {
				t.Errorf("Expected an error during unmarshal but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error during unmarshal but got: %v", err)
			}
		})
	}
}

// You can add more tests for edge cases like:
// - Unmarshaling into an existing, non-nil map.
// - Types containing nested registered types.
// - Concurrent marshal/unmarshal calls (would require separate test structure).
