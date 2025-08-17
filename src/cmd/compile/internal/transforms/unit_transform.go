package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"strings"
)

func init() {
	RegisterTransformer(&UnitTransformer{})
}

type UnitTransformer struct{}

func (t *UnitTransformer) Name() string {
	return "unit_transform"
}

func (t *UnitTransformer) Priority() int {
	return 50 // Run after basic transformers but before type checking
}

func (t *UnitTransformer) Transform(file *syntax.File, ctx *TransformContext) bool {
	if !t.shouldTransform(file) {
		return false
	}

	// First pass: scan for unit assignments like m := units.M
	unitVars := t.findUnitVariableAssignments(file)
	
	// Second pass: check if units are actually needed
	hasUnits := t.hasUnitExpressions(file) || len(unitVars) > 0
	
	
	if hasUnits {
		// Add units import when units are actually used
		t.addUnitsImport(file)
	}

	// Third pass: transform expressions with knowledge of unit variables
	changed := false
	for i, decl := range file.DeclList {
		newDecl := t.transformDecl(decl, unitVars)
		if newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}
	
	return changed
}

func (t *UnitTransformer) transformDecl(decl syntax.Decl, unitVars map[string]bool) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body, unitVars); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
				return &newDecl
			}
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := t.transformExpr(d.Values, unitVars); newValues != d.Values {
				newDecl := *d
				newDecl.Values = newValues
				return &newDecl
			}
		}
	}
	return decl
}

func (t *UnitTransformer) transformStmt(stmt syntax.Stmt, unitVars map[string]bool) syntax.Stmt {
	if stmt == nil {
		return stmt
	}
	
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, sub := range s.List {
			newSub := t.transformStmt(sub, unitVars)
			newList[i] = newSub
			if newSub != sub {
				changed = true
			}
		}
		if changed {
			newStmt := *s
			newStmt.List = newList
			return &newStmt
		}
	case *syntax.ExprStmt:
		if newExpr := t.transformExpr(s.X, unitVars); newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.AssignStmt:
		lhsChanged := false
		rhsChanged := false
		newLhs := t.transformExpr(s.Lhs, unitVars)
		newRhs := t.transformExpr(s.Rhs, unitVars)
		if newLhs != s.Lhs {
			lhsChanged = true
		}
		if newRhs != s.Rhs {
			rhsChanged = true
		}
		if lhsChanged || rhsChanged {
			newStmt := *s
			newStmt.Lhs = newLhs
			newStmt.Rhs = newRhs
			return &newStmt
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			if newResults := t.transformExpr(s.Results, unitVars); newResults != s.Results {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
	case *syntax.CheckStmt:
		// Transform expressions inside check statements: check 500*ms + 5*s == 5500*ms
		if s.Cond != nil {
			if newCond := t.transformExpr(s.Cond, unitVars); newCond != s.Cond {
				newStmt := *s
				newStmt.Cond = newCond
				return &newStmt
			}
		}
	}
	return stmt
}

func (t *UnitTransformer) transformExpr(expr syntax.Expr, unitVars map[string]bool) syntax.Expr {
	if expr == nil {
		return expr
	}
	
	switch n := expr.(type) {
	case *syntax.UnitLitExpr:
		return t.transformUnitLit(n)
	case *syntax.Operation:
		if n.Op == syntax.Mul {
			if t.isNumericLiteral(n.X) && t.isUnitConstantWithContext(n.Y, unitVars) {
				return t.createScalarMultiplication(n.Y, n.X, n.Pos())
			} else if t.isUnitConstantWithContext(n.X, unitVars) && t.isNumericLiteral(n.Y) {
				return t.createScalarMultiplication(n.X, n.Y, n.Pos())
			}
		}
		
		// Handle equality comparison between units specially
		if n.Op == syntax.Eql {
			// Transform operands first
			newX := t.transformExpr(n.X, unitVars)
			newY := t.transformExpr(n.Y, unitVars)
			
			// Check if this is a unit equality comparison
			if t.isUnitExpressionWithContext(newX, unitVars) && t.isUnitExpressionWithContext(newY, unitVars) {
				// Transform: unit1 == unit2 -> unit1.Equal(unit2)
				methodCall := &syntax.SelectorExpr{
					X:   newX,
					Sel: &syntax.Name{Value: "Equal"},
				}
				methodCall.SetPos(n.Pos())
				methodCall.Sel.SetPos(n.Pos())
				
				call := &syntax.CallExpr{
					Fun:     methodCall,
					ArgList: []syntax.Expr{newY},
				}
				call.SetPos(n.Pos())
				return call
			}
		}
		
		// For comparison and other operations, transform operands recursively first
		newX := t.transformExpr(n.X, unitVars)
		newY := t.transformExpr(n.Y, unitVars)
		
		// Create new operation node with transformed operands
		var currentNode *syntax.Operation
		if newX != n.X || newY != n.Y {
			currentNode = &syntax.Operation{
				Op: n.Op,
				X:  newX,
				Y:  newY,
			}
			currentNode.SetPos(n.Pos())
		} else {
			currentNode = n
		}
		
		// Now try to transform the operation itself (e.g., unit addition/subtraction)
		return t.transformUnitBinaryExprWithContext(currentNode, unitVars)
	case *syntax.CallExpr:
		// Transform function arguments
		newFun := t.transformExpr(n.Fun, unitVars)
		changed := newFun != n.Fun
		var newArgList []syntax.Expr
		if n.ArgList != nil {
			newArgList = make([]syntax.Expr, len(n.ArgList))
			for i, arg := range n.ArgList {
				newArg := t.transformExpr(arg, unitVars)
				newArgList[i] = newArg
				if newArg != arg {
					changed = true
				}
			}
		}
		if changed {
			newCall := *n
			newCall.Fun = newFun
			newCall.ArgList = newArgList
			return &newCall
		}
	case *syntax.SelectorExpr:
		if newX := t.transformExpr(n.X, unitVars); newX != n.X {
			newSel := *n
			newSel.X = newX
			return &newSel
		}
	case *syntax.IndexExpr:
		xChanged := false
		indexChanged := false
		newX := t.transformExpr(n.X, unitVars)
		newIndex := t.transformExpr(n.Index, unitVars)
		if newX != n.X {
			xChanged = true
		}
		if newIndex != n.Index {
			indexChanged = true
		}
		if xChanged || indexChanged {
			newIndexExpr := *n
			newIndexExpr.X = newX
			newIndexExpr.Index = newIndex
			return &newIndexExpr
		}
	case *syntax.ParenExpr:
		if newX := t.transformExpr(n.X, unitVars); newX != n.X {
			newParen := *n
			newParen.X = newX
			return &newParen
		}
	}
	return expr
}

func (t *UnitTransformer) transformUnitLit(node *syntax.UnitLitExpr) syntax.Expr {
	pos := node.Pos()
	
	// Transform: 500ms -> units.Ms.Multiply(500.0)
	// Use qualified references to avoid injecting local constants
	
	// Map unit suffix to qualified units package reference
	unitMapping := map[string]string{
		"m": "M", "km": "Km", "cm": "Cm", "mm": "Mm",
		"s": "S", "ms": "Ms", "h": "H", "min": "Min",
		"Hz": "Hz_unit", "kHz": "KHz_unit", "MHz": "MHz", "GHz": "GHz",
	}
	
	unitsConstant, ok := unitMapping[node.Unit]
	if !ok {
		panic("Unknown unit: " + node.Unit)
	}
	
	// Create units.M reference
	unitsRef := &syntax.SelectorExpr{
		X:   &syntax.Name{Value: "units"},
		Sel: &syntax.Name{Value: unitsConstant},
	}
	unitsRef.SetPos(pos)
	unitsRef.X.SetPos(pos)
	unitsRef.Sel.SetPos(pos)
	
	// Create units.Ms.Multiply(value) call
	methodCall := &syntax.SelectorExpr{
		X:   unitsRef,
		Sel: &syntax.Name{Value: "Multiply"},
	}
	methodCall.SetPos(pos)
	methodCall.Sel.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun: methodCall,
		ArgList: []syntax.Expr{
			&syntax.BasicLit{
				Value: node.Value,
				Kind:  syntax.FloatLit,
			},
		},
	}
	call.SetPos(pos)
	
	return call
}

func (t *UnitTransformer) transformUnitBinaryExpr(node *syntax.Operation) syntax.Expr {
	pos := node.Pos()
	
	// Handle number * unit or number · unit multiplication specially  
	if node.Op == syntax.Mul || node.Op == syntax.MiddleDot {
		if t.isNumericLiteral(node.X) && t.isUnitConstant(node.Y) {
			// Transform: 2 * km or 2·km -> km.Mul(NewUnit(2.0, "1"))
			return t.createScalarMultiplication(node.Y, node.X, pos)
		}
		if t.isUnitConstant(node.X) && t.isNumericLiteral(node.Y) {
			// Transform: km * 2 or km·2 -> km.Mul(NewUnit(2.0, "1"))  
			return t.createScalarMultiplication(node.X, node.Y, pos)
		}
	}
	
	if !t.hasUnitOperands(node) {
		return node
	}
	
	var methodName string
	switch node.Op {
	case syntax.Add:
		methodName = "Add"  // Exported method name for Go
	case syntax.Sub:
		methodName = "Sub"
	case syntax.Mul:
		methodName = "Multiply"
	case syntax.Div:
		methodName = "Divide"
	default:
		return node // unsupported operation
	}
	
	// Transform: a + b -> a.Add(b)
	methodCall := &syntax.SelectorExpr{
		X: node.X,
		Sel: &syntax.Name{Value: methodName},
	}
	methodCall.SetPos(pos)
	methodCall.Sel.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun: methodCall,
		ArgList: []syntax.Expr{node.Y},
	}
	call.SetPos(pos)
	
	return call
}

func (t *UnitTransformer) hasUnitOperands(node *syntax.Operation) bool {
	return t.isUnitExpression(node.X) || t.isUnitExpression(node.Y)
}

func (t *UnitTransformer) isUnitExpression(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case *syntax.UnitLitExpr:
		return true
	case *syntax.CallExpr:
		// Check if it's a method call on a unit (has Add, Sub, etc. methods)
		if sel, ok := e.Fun.(*syntax.SelectorExpr); ok {
			methodName := sel.Sel.Value
			return methodName == "Add" || methodName == "Sub" || 
				   methodName == "Multiply" || methodName == "Divide" ||
				   methodName == "ToString" || methodName == "withValue" ||
				   methodName == "add" || methodName == "sub" ||
				   methodName == "mul" || methodName == "div" || methodName == "toString"
		}
	}
	return false
}

func (t *UnitTransformer) addUnitsImport(file *syntax.File) {
	// Check if units is already imported
	for _, decl := range file.DeclList {
		if imp, ok := decl.(*syntax.ImportDecl); ok {
			if imp.Path != nil && imp.Path.Value == `"units"` {
				return // already imported
			}
		}
	}
	
	// Add units import
	importDecl := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: `"units"`,
			Kind:  syntax.StringLit,
		},
	}
	
	// Insert at the beginning of declarations  
	file.DeclList = append([]syntax.Decl{importDecl}, file.DeclList...)
}

func (t *UnitTransformer) addUnitConstants(file *syntax.File) {
	// Add local constants that reference the units package
	// Since Go doesn't allow lowercase exports, we inject them as local constants
	
	// Create variable declarations: var m = units.M, etc.
	unitMappings := map[string]string{
		"m":   "units.M",
		"km":  "units.Km", 
		"cm":  "units.Cm",
		"mm":  "units.Mm",
		"s":   "units.S",
		"ms":  "units.Ms",
		"h":   "units.H",
		"min": "units.Min",
		"Hz":  "units.Hz_unit",
		"kHz": "units.KHz_unit", 
		"MHz": "units.MHz",
		"GHz": "units.GHz",
	}
	
	// Create individual var declarations
	var newDecls []syntax.Decl
	for localName, unitsRef := range unitMappings {
		// Create: var m = units.M
		varDecl := &syntax.VarDecl{
			NameList: []*syntax.Name{{Value: localName}},
			Values:   t.createUnitsReference(unitsRef),
		}
		newDecls = append(newDecls, varDecl)
	}
	
	// Insert the declarations at the beginning of the file after imports
	insertIndex := 0
	for i, decl := range file.DeclList {
		if _, isImport := decl.(*syntax.ImportDecl); !isImport {
			insertIndex = i
			break
		}
	}
	
	// Insert at the correct position
	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+len(newDecls))
	newDeclList = append(newDeclList, file.DeclList[:insertIndex]...)
	newDeclList = append(newDeclList, newDecls...)
	newDeclList = append(newDeclList, file.DeclList[insertIndex:]...)
	file.DeclList = newDeclList
}

func (t *UnitTransformer) createUnitsReference(unitsRef string) syntax.Expr {
	// Parse "units.M" into SelectorExpr
	parts := strings.Split(unitsRef, ".")
	if len(parts) != 2 {
		panic("Invalid units reference: " + unitsRef)
	}
	
	return &syntax.SelectorExpr{
		X:   &syntax.Name{Value: parts[0]}, // "units"
		Sel: &syntax.Name{Value: parts[1]}, // "M"
	}
}

func (t *UnitTransformer) isNumericLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.IntLit || lit.Kind == syntax.FloatLit
	}
	return false
}

func (t *UnitTransformer) isUnitConstant(expr syntax.Expr) bool {
	// Check for simple unit constants: m, km, s, etc.
	if name, ok := expr.(*syntax.Name); ok {
		units := []string{"m", "km", "cm", "mm", "s", "ms", "h", "min", "Hz", "kHz", "MHz", "GHz"}
		for _, unit := range units {
			if name.Value == unit {
				return true
			}
		}
	}
	
	// Check for qualified unit constants: units.M, units.Km, etc.
	if sel, ok := expr.(*syntax.SelectorExpr); ok {
		if pkg, ok := sel.X.(*syntax.Name); ok && pkg.Value == "units" {
			units := []string{"M", "Km", "Cm", "Mm", "S", "Ms", "H", "Min", "Hz", "KHz", "MHz", "GHz"}
			for _, unit := range units {
				if sel.Sel.Value == unit {
					return true
				}
			}
		}
	}
	
	return false
}

func (t *UnitTransformer) createScalarMultiplication(unitExpr, numberExpr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Transform: 2 * units.M -> units.M.Multiply(2.0)
	// Uses the Go method where units have Multiply() methods
	
	// Create unit.Multiply(number)
	methodCall := &syntax.SelectorExpr{
		X:   unitExpr,
		Sel: &syntax.Name{Value: "Multiply"},
	}
	methodCall.SetPos(pos)
	methodCall.Sel.SetPos(pos)
	
	result := &syntax.CallExpr{
		Fun:     methodCall,
		ArgList: []syntax.Expr{numberExpr}, // use the number directly
	}
	result.SetPos(pos)
	
	return result
}

func (t *UnitTransformer) shouldTransform(file *syntax.File) bool {
	// Only transform .goo files
	return true // This will be controlled by the transformer pipeline
}

func (t *UnitTransformer) hasUnitExpressions(file *syntax.File) bool {
	hasUnits := false
	
	// Check for unit constants usage or unit literals in the file
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			if t.isUnitConstant(expr) {
				hasUnits = true
			}
			if _, isUnitLit := expr.(*syntax.UnitLitExpr); isUnitLit {
				hasUnits = true
			}
			return expr
		},
	}
	walker.WalkFile(file)
	
	return hasUnits
}

func (t *UnitTransformer) findUnitVariableAssignments(file *syntax.File) map[string]bool {
	unitVars := make(map[string]bool)
	
	// Look for variable assignments like: m := units.M
	for _, decl := range file.DeclList {
		// Check function-level assignments
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			t.scanBlockForUnitAssignments(funcDecl.Body, unitVars)
		}
		
		// Check package-level variable declarations
		if varDecl, ok := decl.(*syntax.VarDecl); ok {
			// Check for: ms := units.Ms  
			if len(varDecl.NameList) == 1 && varDecl.Values != nil {
				varName := varDecl.NameList[0].Value
				if t.isUnitConstant(varDecl.Values) {
					unitVars[varName] = true
				}
			}
		}
	}
	
	return unitVars
}

func (t *UnitTransformer) scanBlockForUnitAssignments(stmt syntax.Stmt, unitVars map[string]bool) {
	if block, ok := stmt.(*syntax.BlockStmt); ok {
		for _, s := range block.List {
			t.scanStmtForUnitAssignments(s, unitVars)
		}
	}
}

func (t *UnitTransformer) scanStmtForUnitAssignments(stmt syntax.Stmt, unitVars map[string]bool) {
	switch s := stmt.(type) {
	case *syntax.DeclStmt:
		for _, decl := range s.DeclList {
			if varDecl, ok := decl.(*syntax.VarDecl); ok {
				// Check for: m := units.M
				if len(varDecl.NameList) == 1 && varDecl.Values != nil {
					varName := varDecl.NameList[0].Value
					if t.isUnitConstant(varDecl.Values) {
						unitVars[varName] = true
					}
				}
			}
		}
	case *syntax.AssignStmt:
		// Check for: m := units.M  
		if s.Op == syntax.Def {
			if name, ok := s.Lhs.(*syntax.Name); ok {
				if t.isUnitConstant(s.Rhs) {
					unitVars[name.Value] = true
				}
			}
		}
	case *syntax.BlockStmt:
		t.scanBlockForUnitAssignments(s, unitVars)
	}
}


func (t *UnitTransformer) transformUnitBinaryExprWithContext(node *syntax.Operation, unitVars map[string]bool) syntax.Expr {
	pos := node.Pos()
	
	// Handle number * unit or number · unit multiplication specially  
	if node.Op == syntax.Mul || node.Op == syntax.MiddleDot {
		if t.isNumericLiteral(node.X) && t.isUnitConstantWithContext(node.Y, unitVars) {
			// Transform: 2 * m or 2·m -> m.Multiply(2.0)
			return t.createScalarMultiplication(node.Y, node.X, pos)
		}
		if t.isUnitConstantWithContext(node.X, unitVars) && t.isNumericLiteral(node.Y) {
			// Transform: m * 2 or m·2 -> m.Multiply(2.0)  
			return t.createScalarMultiplication(node.X, node.Y, pos)
		}
	}
	
	if !t.hasUnitOperandsWithContext(node, unitVars) {
		return node
	}
	
	var methodName string
	switch node.Op {
	case syntax.Add:
		methodName = "Add"  // Exported method name for Go
	case syntax.Sub:
		methodName = "Sub"
	case syntax.Mul:
		methodName = "Multiply"
	case syntax.Div:
		methodName = "Divide"
	default:
		return node // unsupported operation
	}
	
	// Transform: a + b -> a.Add(b)
	methodCall := &syntax.SelectorExpr{
		X: node.X,
		Sel: &syntax.Name{Value: methodName},
	}
	methodCall.SetPos(pos)
	methodCall.Sel.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun: methodCall,
		ArgList: []syntax.Expr{node.Y},
	}
	call.SetPos(pos)
	
	return call
}

func (t *UnitTransformer) isUnitConstantWithContext(expr syntax.Expr, unitVars map[string]bool) bool {
	// Check if it's a regular unit constant
	if t.isUnitConstant(expr) {
		return true
	}
	
	// Check if it's a variable that was assigned from a unit constant
	if name, ok := expr.(*syntax.Name); ok {
		return unitVars[name.Value]
	}
	
	return false
}

func (t *UnitTransformer) hasUnitOperandsWithContext(node *syntax.Operation, unitVars map[string]bool) bool {
	return t.isUnitExpressionWithContext(node.X, unitVars) || t.isUnitExpressionWithContext(node.Y, unitVars)
}

func (t *UnitTransformer) isUnitExpressionWithContext(expr syntax.Expr, unitVars map[string]bool) bool {
	switch e := expr.(type) {
	case *syntax.UnitLitExpr:
		return true
	case *syntax.CallExpr:
		// Check if it's a method call on a unit (has Add, Sub, etc. methods)
		if sel, ok := e.Fun.(*syntax.SelectorExpr); ok {
			methodName := sel.Sel.Value
			return methodName == "Add" || methodName == "Sub" || 
				   methodName == "Multiply" || methodName == "Divide" ||
				   methodName == "ToString" || methodName == "withValue" ||
				   methodName == "add" || methodName == "sub" ||
				   methodName == "mul" || methodName == "div" || methodName == "toString"
		}
	case *syntax.Name:
		// Check if it's a unit variable
		return unitVars[e.Value]
	}
	return false
}