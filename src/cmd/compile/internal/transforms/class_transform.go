// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// ClassTransform handles automatic transformation of class syntax to struct syntax.
// It transforms expressions like:
// class Person { name string; age int } --> type Person struct { name string; age int }
// class methods and constructors are also transformed appropriately.
type ClassTransform struct{}

func (t *ClassTransform) Name() string {
	return "class_transform"
}

func (t *ClassTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	// Walk through all declarations in the file
	for i, decl := range file.DeclList {
		if typeDecl, ok := decl.(*syntax.TypeDecl); ok {
			if t.isClassDeclaration(typeDecl) {
				if transformed := t.transformClassDeclaration(typeDecl, ctx); transformed != nil {
					file.DeclList[i] = transformed

					// Add class methods to the file
					methods := syntax.GetClassMethods(typeDecl.Name.Value)
					if len(methods) > 0 {
						// Add methods to the file (no transformation needed with self.field syntax)
						for _, method := range methods {
							file.DeclList = append(file.DeclList, method)
						}
					}

					changed = true
				}
			}
		}
	}

	return changed
}

// isClassDeclaration checks if this is a class declaration
func (t *ClassTransform) isClassDeclaration(decl *syntax.TypeDecl) bool {
	// Check if this type declaration was generated from class syntax
	// The parser marks class-generated types with the Alias flag
	return decl.Alias
}

// transformClassDeclaration transforms a class declaration to struct
func (t *ClassTransform) transformClassDeclaration(decl *syntax.TypeDecl, ctx *TransformContext) *syntax.TypeDecl {
	pos := decl.Pos()

	// The parser already created a StructType, we just need to clear the Alias flag
	// to make it a regular type declaration
	newDecl := &syntax.TypeDecl{
		Name: decl.Name,
		Type: decl.Type, // Use the struct type already created by parser
	}
	newDecl.SetPos(pos)
	// Clear the class marker flag
	newDecl.Alias = false

	return newDecl
}

// transformMethodBody transforms method body to use receiver for field access
func (t *ClassTransform) transformMethodBody(method *syntax.FuncDecl, className string) {
	// With the compromise syntax using explicit self.field references,
	// no AST transformation is needed - the user provides the correct syntax
	// The method body already uses self.x, self.y etc.
}

// transformClassMethod transforms class methods to regular methods
func (t *ClassTransform) transformClassMethod(method *syntax.FuncDecl, className string, ctx *TransformContext) *syntax.FuncDecl {
	pos := method.Pos()

	// Create receiver parameter for the method
	receiverType := &syntax.Name{Value: className}
	receiverType.SetPos(pos)

	receiver := &syntax.Field{
		Name: &syntax.Name{Value: "self"}, // Use "self" as receiver name
		Type: receiverType,
	}
	receiver.SetPos(pos)

	// Create new function declaration with receiver
	newMethod := &syntax.FuncDecl{
		Recv: receiver,
		Name: method.Name,
		Type: method.Type,
		Body: method.Body,
	}
	newMethod.SetPos(pos)

	return newMethod
}

// createConstructor creates a constructor function for the class
func (t *ClassTransform) createConstructor(className string, fields []*syntax.Field, pos syntax.Pos) *syntax.FuncDecl {
	// Create function name (e.g., "NewPerson")
	funcName := &syntax.Name{Value: "New" + className}
	funcName.SetPos(pos)

	// Create return type
	returnType := &syntax.Name{Value: className}
	returnType.SetPos(pos)

	// Create function type
	funcType := &syntax.FuncType{
		ParamList: fields, // Constructor parameters match struct fields
		ResultList: []*syntax.Field{
			{
				Type: returnType,
			},
		},
	}
	funcType.SetPos(pos)

	// Create constructor body (placeholder)
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{
			// TODO: Generate actual constructor body
			// return ClassName{field1: param1, field2: param2}
		},
	}
	body.SetPos(pos)

	// Create constructor function
	constructor := &syntax.FuncDecl{
		Name: funcName,
		Type: funcType,
		Body: body,
	}
	constructor.SetPos(pos)

	return constructor
}

// createGetter creates a getter method for a field
func (t *ClassTransform) createGetter(className, fieldName string, fieldType syntax.Expr, pos syntax.Pos) *syntax.FuncDecl {
	// Create method name (e.g., "GetName")
	methodName := &syntax.Name{Value: "Get" + capitalize(fieldName)}
	methodName.SetPos(pos)

	// Create receiver
	receiverType := &syntax.Name{Value: className}
	receiverType.SetPos(pos)

	receiver := &syntax.Field{
		Name: &syntax.Name{Value: "self"},
		Type: receiverType,
	}
	receiver.SetPos(pos)

	// Create method type
	methodType := &syntax.FuncType{
		ResultList: []*syntax.Field{
			{
				Type: fieldType,
			},
		},
	}
	methodType.SetPos(pos)

	// Create method body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{
			// return self.fieldName
			&syntax.ReturnStmt{
				Results: &syntax.SelectorExpr{
					X:   &syntax.Name{Value: "self"},
					Sel: &syntax.Name{Value: fieldName},
				},
			},
		},
	}
	body.SetPos(pos)

	// Create getter method
	getter := &syntax.FuncDecl{
		Recv: receiver,
		Name: methodName,
		Type: methodType,
		Body: body,
	}
	getter.SetPos(pos)

	return getter
}

// createSetter creates a setter method for a field
func (t *ClassTransform) createSetter(className, fieldName string, fieldType syntax.Expr, pos syntax.Pos) *syntax.FuncDecl {
	// Create method name (e.g., "SetName")
	methodName := &syntax.Name{Value: "Set" + capitalize(fieldName)}
	methodName.SetPos(pos)

	// Create receiver
	receiverType := &syntax.Name{Value: "*" + className} // Pointer receiver for setter
	receiverType.SetPos(pos)

	receiver := &syntax.Field{
		Name: &syntax.Name{Value: "self"},
		Type: receiverType,
	}
	receiver.SetPos(pos)

	// Create parameter
	param := &syntax.Field{
		Name: &syntax.Name{Value: fieldName},
		Type: fieldType,
	}
	param.SetPos(pos)

	// Create method type
	methodType := &syntax.FuncType{
		ParamList: []*syntax.Field{param},
	}
	methodType.SetPos(pos)

	// Create method body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{
			// self.fieldName = fieldName
			&syntax.AssignStmt{
				Op: 0, // 0 means no operation (simple assignment)
				Lhs: &syntax.SelectorExpr{
					X:   &syntax.Name{Value: "self"},
					Sel: &syntax.Name{Value: fieldName},
				},
				Rhs: &syntax.Name{Value: fieldName},
			},
		},
	}
	body.SetPos(pos)

	// Create setter method
	setter := &syntax.FuncDecl{
		Recv: receiver,
		Name: methodName,
		Type: methodType,
		Body: body,
	}
	setter.SetPos(pos)

	return setter
}

// Helper function to capitalize first letter
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

func init() {
	RegisterTransformer(&ClassTransform{})
}
