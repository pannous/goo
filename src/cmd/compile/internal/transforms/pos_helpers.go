package transforms

import "cmd/compile/internal/syntax"

// generatedNodePos returns a reasonable position to use for newly synthesized
// nodes so that they share the same PosBase as existing source elements.
func generatedNodePos(file *syntax.File) syntax.Pos {
	if file != nil {
		ensureFileNodeBases(file)
		if file.PkgName != nil {
			if pos := file.PkgName.Pos(); pos.IsKnown() {
				return pos
			}
		}
		for _, decl := range file.DeclList {
			if decl == nil {
				continue
			}
			if pos := decl.Pos(); pos.IsKnown() {
				return pos
			}
		}
		if len(file.TopLevelStmts) > 0 {
			if pos := file.TopLevelStmts[0].Pos(); pos.IsKnown() {
				return pos
			}
		}
		if file.Path != nil {
			return syntax.MakePos(file.Path, 1, 1)
		}
		base := syntax.NewFileBase("generated.goo")
		file.Path = base
		return syntax.MakePos(base, 1, 1)
	}

	base := syntax.NewFileBase("generated.goo")
	return syntax.MakePos(base, 1, 1)
}

// ensureImportPos assigns a stable position to the generated import so that
// later compilation stages can clone it without hitting missing PosBase panics.
func ensureImportPos(file *syntax.File, imp *syntax.ImportDecl) {
	if imp == nil {
		return
	}
	ensureFileNodeBases(file)
	generated := generatedNodePos(file)
	current := imp.Pos()
	if !current.IsKnown() || current.Base() == nil {
		current = generated
		imp.SetPos(current)
	}
	if path := imp.Path; path != nil {
		pathPos := path.Pos()
		if !pathPos.IsKnown() || pathPos.Base() == nil {
			path.SetPos(current)
		}
	}
	if local := imp.LocalPkgName; local != nil {
		localPos := local.Pos()
		if !localPos.IsKnown() || localPos.Base() == nil {
			local.SetPos(current)
		}
	}
}

type posBaseFixer struct {
	base *syntax.PosBase
}

func (f *posBaseFixer) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	pos := node.Pos()
	line := pos.Line()
	if line == 0 {
		line = 1
	}
	col := pos.Col()
	if col == 0 {
		col = 1
	}
	if pos.Base() == nil {
		node.SetPos(syntax.MakePos(f.base, line, col))
	} else if line != pos.Line() || col != pos.Col() {
		node.SetPos(syntax.MakePos(pos.Base(), line, col))
	}
	return f
}

func ensureFileNodeBases(file *syntax.File) {
	if file == nil {
		return
	}
	base := file.Path
	if base == nil {
		filename := "generated.goo"
		if file.PkgName != nil && file.PkgName.Value != "" {
			filename = file.PkgName.Value + ".goo"
		}
		base = syntax.NewFileBase(filename)
		file.Path = base
	}
	fixer := &posBaseFixer{base: base}
	syntax.Walk(file, fixer)
	for _, stmt := range file.TopLevelStmts {
		if stmt != nil {
			syntax.Walk(stmt, fixer)
		}
	}
}
