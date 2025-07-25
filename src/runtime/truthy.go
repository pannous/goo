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
func truthy(i any) bool {
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
		switch typ.Kind() & abi.KindMask {
		case abi.Slice:
			slice := (*slice)(eface.data)
			return slice.len != 0
		case abi.Map:
			// Map is truthy if non-nil
			return eface.data != nil
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
