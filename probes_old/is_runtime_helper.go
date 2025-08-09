package main

import (
	"reflect"
)

// isTypeOf checks if value x has the same type as the type of typeInstance
func isTypeOf(x any, typeInstance any) bool {
	if x == nil && typeInstance == nil {
		return true
	}
	if x == nil || typeInstance == nil {
		return false
	}
	
	xType := reflect.TypeOf(x)
	instanceType := reflect.TypeOf(typeInstance)
	
	// Remove pointer indirection from instanceType since we pass (*T)(nil)
	if instanceType.Kind() == reflect.Ptr {
		instanceType = instanceType.Elem()
	}
	
	return xType == instanceType
}

func main() {
	// Test cases
	println("Testing isTypeOf:")
	
	// Basic type checks
	println("1 is int:", isTypeOf(1, (*int)(nil)))                 // Should be true
	println("1 is string:", isTypeOf(1, (*string)(nil)))           // Should be false
	println("\"hello\" is string:", isTypeOf("hello", (*string)(nil))) // Should be true
	
	// Slice tests  
	slice := []int{1, 2, 3}
	println("[]int is []int:", isTypeOf(slice, (*[]int)(nil)))     // Should be true
	println("[]int is []any:", isTypeOf(slice, (*[]any)(nil)))     // Should be false
}