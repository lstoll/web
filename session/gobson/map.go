package gobson

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sync"
)

// --- Type Registry ---

var (
	registry      = make(map[string]reflect.Type)
	registryMutex sync.RWMutex
	// Optional reverse lookup for marshaling efficiency
	reverseRegistry = make(map[reflect.Type]string)
)

// Register associates a type with a unique name for dynamic marshaling.
// It registers both the value type and the pointer type.
func Register(name string, value any) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	valType := reflect.TypeOf(value)
	if valType.Kind() == reflect.Ptr {
		valType = valType.Elem() // Get the underlying type if pointer
	}

	// Register the value type
	if _, exists := registry[name]; exists {
		log.Printf("Warning: Type name '%s' already registered. Overwriting.", name)
	}
	registry[name] = valType
	reverseRegistry[valType] = name

	// Also register the pointer type
	ptrType := reflect.PointerTo(valType)
	reverseRegistry[ptrType] = name // Marshaling often deals with pointers

	log.Printf("Registered type '%s' for %v and %v", name, valType, ptrType)
}

// ResetRegistry is needed for test isolation between test functions/runs
func ResetRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	registry = make(map[string]reflect.Type)
	reverseRegistry = make(map[reflect.Type]string)
}

// getType returns the reflect.Type associated with a registered name.
func getType(name string) (reflect.Type, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	typ, ok := registry[name]
	return typ, ok
}

// getTypeName returns the registered name for a given reflect.Type.
func getTypeName(typ reflect.Type) (string, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	name, ok := reverseRegistry[typ]
	return name, ok
}

// --- Wrapper for Typed Values ---

const (
	typeField  = "__type__" // Choose field names unlikely to clash
	valueField = "__value__"
)

// typedValue is the structure we use in JSON to wrap registered types.
type typedValue struct {
	Type  string          `json:"__type__"`  // Registered type name
	Value json.RawMessage `json:"__value__"` // Raw JSON of the actual value
}

// --- Custom Map Type for Marshaling/Unmarshaling ---

// DynamicMap wraps map[string]any to provide custom JSON handling.
type DynamicMap map[string]any

// MarshalJSON implements the json.Marshaler interface for DynamicMap.
func (dm DynamicMap) MarshalJSON() ([]byte, error) {
	// Create a temporary map to marshal standard or wrapped values
	tempMap := make(map[string]any, len(dm))

	for key, val := range dm {
		if val == nil {
			tempMap[key] = nil
			continue
		}

		valType := reflect.TypeOf(val)
		typeName, registered := getTypeName(valType)

		if registered {
			// Marshal the actual value first
			valBytes, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal registered type %s for key '%s': %w", typeName, key, err)
			}
			// Wrap it in our typedValue structure
			tempMap[key] = typedValue{
				Type:  typeName,
				Value: json.RawMessage(valBytes),
			}
		} else {
			// Not a registered type, marshal as is
			tempMap[key] = val
		}
	}

	// Marshal the temporary map which now contains either original values
	// or wrapped typedValue structs.
	return json.Marshal(tempMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for DynamicMap.
func (dm *DynamicMap) UnmarshalJSON(data []byte) error {
	// 1. Unmarshal into a map of raw messages first
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("failed to unmarshal into raw map: %w", err)
	}

	// Initialize the actual map if it's nil
	if *dm == nil {
		*dm = make(DynamicMap)
	}

	// 2. Process each raw message
	for key, rawValue := range rawMap {
		// 3. Try to unmarshal as our special typedValue wrapper
		var wrapper typedValue
		err := json.Unmarshal(rawValue, &wrapper)

		if err == nil && wrapper.Type != "" && wrapper.Value != nil {
			// It looks like our wrapper!
			registeredType, typeFound := getType(wrapper.Type)
			if !typeFound {
				// If type is specified but not found, treat as error or generic map?
				// Option 1: Error out
				// return fmt.Errorf("unregistered type '%s' found for key '%s'", wrapper.Type, key)
				// Option 2: Unmarshal __value__ as map[string]any (or just any)
				log.Printf("Warning: Unregistered type '%s' found for key '%s'. Unmarshaling __value__ generically.", wrapper.Type, key)
				var genericValue any
				if err := json.Unmarshal(wrapper.Value, &genericValue); err != nil {
					return fmt.Errorf("failed to unmarshal value for unregistered type '%s' key '%s': %w", wrapper.Type, key, err)
				}
				(*dm)[key] = genericValue // Store the inner value directly
				continue                  // Skip to next key
				// Option 3: Could unmarshal the wrapper itself if needed {Type: "name", Value: map[...]}
				// (*dm)[key] = wrapper // Store the wrapper struct itself (less useful usually)
				// continue
			}

			// Create a new instance of the registered type (must be a pointer for Unmarshal)
			newValue := reflect.New(registeredType).Interface()

			// Unmarshal the inner Value into the new instance
			if err := json.Unmarshal(wrapper.Value, newValue); err != nil {
				return fmt.Errorf("failed to unmarshal value for type '%s' key '%s': %w", wrapper.Type, key, err)
			}

			// Store the concrete typed value (dereference the pointer)
			(*dm)[key] = reflect.ValueOf(newValue).Elem().Interface()

		} else {
			// 4. Not our wrapper, unmarshal as standard 'any'
			var genericValue any
			if err := json.Unmarshal(rawValue, &genericValue); err != nil {
				return fmt.Errorf("failed to unmarshal generic value for key '%s': %w", key, err)
			}
			(*dm)[key] = genericValue
		}
	}

	return nil
}
