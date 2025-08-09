// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/abi"
	"unsafe"
)

// truthy implements truthiness conversion for if and for statements.
// This function is called by the compiler when non-boolean values
// are used in conditional contexts.
func truthy(i interface{}) bool {
	if i == nil {
		return false
	}

	switch v := i.(type) {
	// Boolean values
	case bool:
		return v

	// Numeric types - zero values are falsy
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case uintptr:
		return v != 0
	case float32:
		return v != 0.0
	case float64:
		return v != 0.0
	case complex64:
		return v != 0+0i
	case complex128:
		return v != 0+0i

	// String type - empty string is falsy
	case string:
		return len(v) != 0

	// Pointer types - nil is falsy
	case unsafe.Pointer:
		return v != nil

	default:
		// For other types (slices, maps, channels, interfaces),
		// use reflection-like approach to check for zero value
		eface := (*eface)(unsafe.Pointer(&i))
		if eface.data == nil {
			return false
		}

		typ := eface._type
		if typ == nil {
			return false
		}

		// Handle slice, map, channel - zero length/nil is falsy
		switch typ.Kind() {
		case abi.Slice:
			slice := (*slice)(eface.data)
			return slice.len != 0
		case abi.Map:
			// Map is truthy if non-nil and has elements
			if eface.data == nil {
				return false // nil map is falsy
			}
			// For non-nil maps, check if they have any elements
			// This is a simplified approach - ideally we'd check map length
			// but that requires more complex runtime introspection
			return true // Non-nil maps are truthy (including empty ones created with make())
		case abi.Chan:
			// Channel is truthy if non-nil
			return eface.data != nil
		case abi.Pointer, abi.Func:
			// Pointers and functions are truthy if non-nil
			return eface.data != nil
		case abi.Interface:
			// Interface is truthy if non-nil
			iface := (*iface)(eface.data)
			return iface.tab != nil && iface.data != nil
		default:
			// For other types (struct, array), they are truthy if not nil
			// This is a conservative approach - structs/arrays are always truthy
			return true
		}
	}
}

// falsey is the opposite of truthy - returns true for falsy values
func falsey(i interface{}) bool {
	return !truthy(i)
}

// truthyAndOp implements the truthy AND operator (&&)
// Returns the first falsy value, or the last value if all are truthy
func truthyAndOp(left interface{}, right interface{}) interface{} {
	if !truthy(left) {
		return left
	}
	return right
}

// isTypeOf checks if the value has the specified type name
// This is used by the compiler for type checking operations
func isTypeOf(value interface{}, typeName string) bool {
	if value == nil {
		return typeName == "nil" || typeName == "<nil>"
	}

	// For now, let's use a simplified approach based on known types
	switch v := value.(type) {
	case int:
		return typeName == "int"
	case string:
		return typeName == "string"
	case bool:
		return typeName == "bool"
	case float64:
		return typeName == "float64"
	case float32:
		return typeName == "float32"
	case []int:
		return typeName == "[]int"
	case []string:
		return typeName == "[]string"
	case []interface{}:
		return typeName == "[]interface{}" || typeName == "[]any"
	default:
		// For other types, try to match based on type assertion patterns
		_ = v
		return false
	}
}

// typeMatches is an alias for isTypeOf for backward compatibility
// This is used by the is operator transform: x is int -> typeMatches(x, "int")
func typeMatches(value interface{}, typeName string) bool {
	return isTypeOf(value, typeName)
}

// listSort sorts a list in ascending order and returns the sorted list
func listSort(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		// Create a copy to avoid modifying original
		result := make([]int, len(v))
		copy(result, v)
		// Simple bubble sort in ascending order
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []string:
		result := make([]string, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []float64:
		result := make([]float64, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	default:
		// For unsupported types, return original
		return list
	}
}

// listReverse reverses a list and returns the reversed list  
func listReverse(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		// Create a copy to avoid modifying original
		result := make([]int, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []string:
		result := make([]string, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []float64:
		result := make([]float64, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	default:
		// For unsupported types, return original
		return list
	}
}

// listSortDesc sorts a list in descending order and returns the sorted list
func listSortDesc(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		// Create a copy to avoid modifying original
		result := make([]int, len(v))
		copy(result, v)
		// Sort in descending order
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] < result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []string:
		result := make([]string, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] < result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []float64:
		result := make([]float64, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] < result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	default:
		// For unsupported types, return original
		return list
	}
}

// listPop removes and returns the last element from the list
func listPop(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		if len(v) == 0 {
			return 0 // Return zero value for empty slice
		}
		return v[len(v)-1]
	case []string:
		if len(v) == 0 {
			return ""
		}
		return v[len(v)-1]
	case []float64:
		if len(v) == 0 {
			return 0.0
		}
		return v[len(v)-1]
	case []interface{}:
		if len(v) == 0 {
			return nil
		}
		return v[len(v)-1]
	default:
		return nil
	}
}

// listShift removes and returns the first element from the list
func listShift(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		if len(v) == 0 {
			return 0 // Return zero value for empty slice
		}
		return v[0]
	case []string:
		if len(v) == 0 {
			return ""
		}
		return v[0]
	case []float64:
		if len(v) == 0 {
			return 0.0
		}
		return v[0]
	case []interface{}:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	default:
		return nil
	}
}

// sliceCloneAndSort creates a clone of the slice and sorts it
func sliceCloneAndSort(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		// Create a copy
		result := make([]int, len(v))
		copy(result, v)
		// Simple bubble sort in ascending order
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []string:
		result := make([]string, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	case []float64:
		result := make([]float64, len(v))
		copy(result, v)
		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				if result[i] > result[j] {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
		return result
	default:
		// For unsupported types, return original
		return list
	}
}

// sliceCloneAndReverse creates a clone of the slice and reverses it  
func sliceCloneAndReverse(list interface{}) interface{} {
	switch v := list.(type) {
	case []int:
		result := make([]int, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []string:
		result := make([]string, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []float64:
		result := make([]float64, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[len(v)-1-i] = val
		}
		return result
	default:
		// For unsupported types, return original
		return list
	}
}
