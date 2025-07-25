// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	. "internal/types/errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// A declInfo describes a package-level const, type, var, or func declaration.
type declInfo struct {
	file      *Scope        // scope of file containing this declaration
	version   goVersion     // Go version of file containing this declaration
	lhs       []*Var        // lhs of n:1 variable declarations, or nil
	vtyp      ast.Expr      // type, or nil (for const and var declarations only)
	init      ast.Expr      // init/orig expression, or nil (for const and var declarations only)
	inherited bool          // if set, the init expression is inherited from a previous constant declaration
	tdecl     *ast.TypeSpec // type declaration, or nil
	fdecl     *ast.FuncDecl // func declaration, or nil

	// The deps field tracks initialization expression dependencies.
	deps map[Object]bool // lazily initialized
}

// hasInitializer reports whether the declared object has an initialization
// expression or function body.
func (d *declInfo) hasInitializer() bool {
	return d.init != nil || d.fdecl != nil && d.fdecl.Body != nil
}

// addDep adds obj to the set of objects d's init expression depends on.
func (d *declInfo) addDep(obj Object) {
	m := d.deps
	if m == nil {
		m = make(map[Object]bool)
		d.deps = m
	}
	m[obj] = true
}

// arityMatch checks that the lhs and rhs of a const or var decl
// have the appropriate number of names and init exprs. For const
// decls, init is the value spec providing the init exprs; for
// var decls, init is nil (the init exprs are in s in this case).
func (checks *Checker) arityMatch(s, init *ast.ValueSpec) {
	l := len(s.Names)
	r := len(s.Values)
	if init != nil {
		r = len(init.Values)
	}

	const code = WrongAssignCount
	switch {
	case init == nil && r == 0:
		// var decl w/o init expr
		if s.Type == nil {
			checks.error(s, code, "missing type or init expr")
		}
	case l < r:
		if l < len(s.Values) {
			// init exprs from s
			n := s.Values[l]
			checks.errorf(n, code, "extra init expr %s", n)
			// TODO(gri) avoid declared and not used error here
		} else {
			// init exprs "inherited"
			checks.errorf(s, code, "extra init expr at %s", checks.fset.Position(init.Pos()))
			// TODO(gri) avoid declared and not used error here
		}
	case l > r && (init != nil || r != 1):
		n := s.Names[r]
		checks.errorf(n, code, "missing init expr for %s", n)
	}
}

func validatedImportPath(path string) (string, error) {
	s, err := strconv.Unquote(path)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("empty string")
	}
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`\uFFFD"
	for _, r := range s {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
			return s, fmt.Errorf("invalid character %#U", r)
		}
	}
	return s, nil
}

// declarePkgObj declares obj in the package scope, records its ident -> obj mapping,
// and updates check.objMap. The object must not be a function or method.
func (checks *Checker) declarePkgObj(ident *ast.Ident, obj Object, d *declInfo) {
	assert(ident.Name == obj.Name())

	// spec: "A package-scope or file-scope identifier with name init
	// may only be declared to be a function with this (func()) signature."
	if ident.Name == "init" {
		checks.error(ident, InvalidInitDecl, "cannot declare init - must be func")
		return
	}

	// spec: "The main package must have package name main and declare
	// a function main that takes no arguments and returns no value."
	if ident.Name == "main" && checks.pkg.name == "main" {
		checks.error(ident, InvalidMainDecl, "cannot declare main - must be func")
		return
	}

	checks.declare(checks.pkg.scope, ident, obj, nopos)
	checks.objMap[obj] = d
	obj.setOrder(uint32(len(checks.objMap)))
}

// filename returns a filename suitable for debugging output.
func (checks *Checker) filename(fileNo int) string {
	file := checks.files[fileNo]
	if pos := file.Pos(); pos.IsValid() {
		return checks.fset.File(pos).Name()
	}
	return fmt.Sprintf("file[%d]", fileNo)
}

func (checks *Checker) importPackage(at positioner, path, dir string) *Package {
	// If we already have a package for the given (path, dir)
	// pair, use it instead of doing a full import.
	// Checker.impMap only caches packages that are marked Complete
	// or fake (dummy packages for failed imports). Incomplete but
	// non-fake packages do require an import to complete them.
	key := importKey{path, dir}
	imp := checks.impMap[key]
	if imp != nil {
		return imp
	}

	// no package yet => import it
	if path == "C" && (checks.conf.FakeImportC || checks.conf.go115UsesCgo) {
		if checks.conf.FakeImportC && checks.conf.go115UsesCgo {
			checks.error(at, BadImportPath, "cannot use FakeImportC and go115UsesCgo together")
		}
		imp = NewPackage("C", "C")
		imp.fake = true // package scope is not populated
		imp.cgo = checks.conf.go115UsesCgo
	} else {
		// ordinary import
		var err error
		if importer := checks.conf.Importer; importer == nil {
			err = fmt.Errorf("Config.Importer not installed")
		} else if importerFrom, ok := importer.(ImporterFrom); ok {
			imp, err = importerFrom.ImportFrom(path, dir, 0)
			if imp == nil && err == nil {
				err = fmt.Errorf("Config.Importer.ImportFrom(%s, %s, 0) returned nil but no error", path, dir)
			}
		} else {
			imp, err = importer.Import(path)
			if imp == nil && err == nil {
				err = fmt.Errorf("Config.Importer.Import(%s) returned nil but no error", path)
			}
		}
		// make sure we have a valid package name
		// (errors here can only happen through manipulation of packages after creation)
		if err == nil && imp != nil && (imp.name == "_" || imp.name == "") {
			err = fmt.Errorf("invalid package name: %q", imp.name)
			imp = nil // create fake package below
		}
		if err != nil {
			checks.errorf(at, BrokenImport, "could not import %s (%s)", path, err)
			if imp == nil {
				// create a new fake package
				// come up with a sensible package name (heuristic)
				name := path
				if i := len(name); i > 0 && name[i-1] == '/' {
					name = name[:i-1]
				}
				if i := strings.LastIndex(name, "/"); i >= 0 {
					name = name[i+1:]
				}
				imp = NewPackage(path, name)
			}
			// continue to use the package as best as we can
			imp.fake = true // avoid follow-up lookup failures
		}
	}

	// package should be complete or marked fake, but be cautious
	if imp.complete || imp.fake {
		checks.impMap[key] = imp
		// Once we've formatted an error message, keep the pkgPathMap
		// up-to-date on subsequent imports. It is used for package
		// qualification in error messages.
		if checks.pkgPathMap != nil {
			checks.markImports(imp)
		}
		return imp
	}

	// something went wrong (importer may have returned incomplete package without error)
	return nil
}

// collectObjects collects all file and package objects and inserts them
// into their respective scopes. It also performs imports and associates
// methods with receiver base type names.
func (checks *Checker) collectObjects() {
	pkg := checks.pkg

	// pkgImports is the set of packages already imported by any package file seen
	// so far. Used to avoid duplicate entries in pkg.imports. Allocate and populate
	// it (pkg.imports may not be empty if we are checking test files incrementally).
	// Note that pkgImports is keyed by package (and thus package path), not by an
	// importKey value. Two different importKey values may map to the same package
	// which is why we cannot use the check.impMap here.
	var pkgImports = make(map[*Package]bool)
	for _, imp := range pkg.imports {
		pkgImports[imp] = true
	}

	type methodInfo struct {
		obj  *Func      // method
		ptr  bool       // true if pointer receiver
		recv *ast.Ident // receiver type name
	}
	var methods []methodInfo // collected methods with valid receivers and non-blank _ names

	fileScopes := make([]*Scope, len(checks.files)) // fileScopes[i] corresponds to check.files[i]
	for fileNo, file := range checks.files {
		checks.version = asGoVersion(checks.versions[file])

		// The package identifier denotes the current package,
		// but there is no corresponding package object.
		checks.recordDef(file.Name, nil)

		// Use the actual source file extent rather than *ast.File extent since the
		// latter doesn't include comments which appear at the start or end of the file.
		// Be conservative and use the *ast.File extent if we don't have a *token.File.
		pos, end := file.Pos(), file.End()
		if f := checks.fset.File(file.Pos()); f != nil {
			pos, end = token.Pos(f.Base()), token.Pos(f.Base()+f.Size())
		}
		fileScope := NewScope(pkg.scope, pos, end, checks.filename(fileNo))
		fileScopes[fileNo] = fileScope
		checks.recordScope(file, fileScope)

		// determine file directory, necessary to resolve imports
		// FileName may be "" (typically for tests) in which case
		// we get "." as the directory which is what we would want.
		fileDir := dir(checks.fset.Position(file.Name.Pos()).Filename)

		checks.walkDecls(file.Decls, func(d decl) {
			switch d := d.(type) {
			case importDecl:
				// import package
				if d.spec.Path.Value == "" {
					return // error reported by parser
				}
				path, err := validatedImportPath(d.spec.Path.Value)
				if err != nil {
					checks.errorf(d.spec.Path, BadImportPath, "invalid import path (%s)", err)
					return
				}

				imp := checks.importPackage(d.spec.Path, path, fileDir)
				if imp == nil {
					return
				}

				// local name overrides imported package name
				name := imp.name
				if d.spec.Name != nil {
					name = d.spec.Name.Name
					if path == "C" {
						// match 1.17 cmd/compile (not prescribed by spec)
						checks.error(d.spec.Name, ImportCRenamed, `cannot rename import "C"`)
						return
					}
				}

				if name == "init" {
					checks.error(d.spec, InvalidInitDecl, "cannot import package as init - init must be a func")
					return
				}

				// add package to list of explicit imports
				// (this functionality is provided as a convenience
				// for clients; it is not needed for type-checking)
				if !pkgImports[imp] {
					pkgImports[imp] = true
					pkg.imports = append(pkg.imports, imp)
				}

				pkgName := NewPkgName(d.spec.Pos(), pkg, name, imp)
				if d.spec.Name != nil {
					// in a dot-import, the dot represents the package
					checks.recordDef(d.spec.Name, pkgName)
				} else {
					checks.recordImplicit(d.spec, pkgName)
				}

				if imp.fake {
					// match 1.17 cmd/compile (not prescribed by spec)
					checks.usedPkgNames[pkgName] = true
				}

				// add import to file scope
				checks.imports = append(checks.imports, pkgName)
				if name == "." {
					// dot-import
					if checks.dotImportMap == nil {
						checks.dotImportMap = make(map[dotImportKey]*PkgName)
					}
					// merge imported scope with file scope
					for name, obj := range imp.scope.elems {
						// Note: Avoid eager resolve(name, obj) here, so we only
						// resolve dot-imported objects as needed.

						// A package scope may contain non-exported objects,
						// do not import them!
						if token.IsExported(name) {
							// declare dot-imported object
							// (Do not use check.declare because it modifies the object
							// via Object.setScopePos, which leads to a race condition;
							// the object may be imported into more than one file scope
							// concurrently. See go.dev/issue/32154.)
							if alt := fileScope.Lookup(name); alt != nil {
								err := checks.newError(DuplicateDecl)
								err.addf(d.spec.Name, "%s redeclared in this block", alt.Name())
								err.addAltDecl(alt)
								err.report()
							} else {
								fileScope.insert(name, obj)
								checks.dotImportMap[dotImportKey{fileScope, name}] = pkgName
							}
						}
					}
				} else {
					// declare imported package object in file scope
					// (no need to provide s.Name since we called check.recordDef earlier)
					checks.declare(fileScope, nil, pkgName, nopos)
				}
			case constDecl:
				// declare all constants
				for i, name := range d.spec.Names {
					obj := NewConst(name.Pos(), pkg, name.Name, nil, constant.MakeInt64(int64(d.iota)))

					var init ast.Expr
					if i < len(d.init) {
						init = d.init[i]
					}

					d := &declInfo{file: fileScope, version: checks.version, vtyp: d.typ, init: init, inherited: d.inherited}
					checks.declarePkgObj(name, obj, d)
				}

			case varDecl:
				lhs := make([]*Var, len(d.spec.Names))
				// If there's exactly one rhs initializer, use
				// the same declInfo d1 for all lhs variables
				// so that each lhs variable depends on the same
				// rhs initializer (n:1 var declaration).
				var d1 *declInfo
				if len(d.spec.Values) == 1 {
					// The lhs elements are only set up after the for loop below,
					// but that's ok because declareVar only collects the declInfo
					// for a later phase.
					d1 = &declInfo{file: fileScope, version: checks.version, lhs: lhs, vtyp: d.spec.Type, init: d.spec.Values[0]}
				}

				// declare all variables
				for i, name := range d.spec.Names {
					obj := newVar(PackageVar, name.Pos(), pkg, name.Name, nil)
					lhs[i] = obj

					di := d1
					if di == nil {
						// individual assignments
						var init ast.Expr
						if i < len(d.spec.Values) {
							init = d.spec.Values[i]
						}
						di = &declInfo{file: fileScope, version: checks.version, vtyp: d.spec.Type, init: init}
					}

					checks.declarePkgObj(name, obj, di)
				}
			case typeDecl:
				obj := NewTypeName(d.spec.Name.Pos(), pkg, d.spec.Name.Name, nil)
				checks.declarePkgObj(d.spec.Name, obj, &declInfo{file: fileScope, version: checks.version, tdecl: d.spec})
			case funcDecl:
				name := d.decl.Name.Name
				obj := NewFunc(d.decl.Name.Pos(), pkg, name, nil) // signature set later
				hasTParamError := false                           // avoid duplicate type parameter errors
				if d.decl.Recv.NumFields() == 0 {
					// regular function
					if d.decl.Recv != nil {
						checks.error(d.decl.Recv, BadRecv, "method has no receiver")
						// treat as function
					}
					if name == "init" || (name == "main" && checks.pkg.name == "main") {
						code := InvalidInitDecl
						if name == "main" {
							code = InvalidMainDecl
						}
						if d.decl.Type.TypeParams.NumFields() != 0 {
							checks.softErrorf(d.decl.Type.TypeParams.List[0], code, "func %s must have no type parameters", name)
							hasTParamError = true
						}
						if t := d.decl.Type; t.Params.NumFields() != 0 || t.Results != nil {
							// TODO(rFindley) Should this be a hard error?
							checks.softErrorf(d.decl.Name, code, "func %s must have no arguments and no return values", name)
						}
					}
					if name == "init" {
						// don't declare init functions in the package scope - they are invisible
						obj.parent = pkg.scope
						checks.recordDef(d.decl.Name, obj)
						if d.decl.Body == nil {
							checks.softErrorf(obj, MissingInitBody, "func init must have a body")
						}
					} else {
						checks.declare(pkg.scope, d.decl.Name, obj, nopos)
					}
				} else {
					// method

					// TODO(rFindley) earlier versions of this code checked that methods
					//                have no type parameters, but this is checked later
					//                when type checking the function type. Confirm that
					//                we don't need to check tparams here.

					ptr, base, _ := checks.unpackRecv(d.decl.Recv.List[0].Type, false)
					// (Methods with invalid receiver cannot be associated to a type, and
					// methods with blank _ names are never found; no need to collect any
					// of them. They will still be type-checked with all the other functions.)
					if recv, _ := base.(*ast.Ident); recv != nil && name != "_" {
						methods = append(methods, methodInfo{obj, ptr, recv})
					}
					checks.recordDef(d.decl.Name, obj)
				}
				_ = d.decl.Type.TypeParams.NumFields() != 0 && !hasTParamError && checks.verifyVersionf(d.decl.Type.TypeParams.List[0], go1_18, "type parameter")
				info := &declInfo{file: fileScope, version: checks.version, fdecl: d.decl}
				// Methods are not package-level objects but we still track them in the
				// object map so that we can handle them like regular functions (if the
				// receiver is invalid); also we need their fdecl info when associating
				// them with their receiver base type, below.
				checks.objMap[obj] = info
				obj.setOrder(uint32(len(checks.objMap)))
			}
		})
	}

	// verify that objects in package and file scopes have different names
	for _, scope := range fileScopes {
		for name, obj := range scope.elems {
			if alt := pkg.scope.Lookup(name); alt != nil {
				obj = resolve(name, obj)
				err := checks.newError(DuplicateDecl)
				if pkg, ok := obj.(*PkgName); ok {
					err.addf(alt, "%s already declared through import of %s", alt.Name(), pkg.Imported())
					err.addAltDecl(pkg)
				} else {
					err.addf(alt, "%s already declared through dot-import of %s", alt.Name(), obj.Pkg())
					// TODO(gri) dot-imported objects don't have a position; addAltDecl won't print anything
					err.addAltDecl(obj)
				}
				err.report()
			}
		}
	}

	// Now that we have all package scope objects and all methods,
	// associate methods with receiver base type name where possible.
	// Ignore methods that have an invalid receiver. They will be
	// type-checked later, with regular functions.
	if methods == nil {
		return
	}

	checks.methods = make(map[*TypeName][]*Func)
	for i := range methods {
		m := &methods[i]
		// Determine the receiver base type and associate m with it.
		ptr, base := checks.resolveBaseTypeName(m.ptr, m.recv)
		if base != nil {
			m.obj.hasPtrRecv_ = ptr
			checks.methods[base] = append(checks.methods[base], m.obj)
		}
	}
}

// unpackRecv unpacks a receiver type expression and returns its components: ptr indicates
// whether rtyp is a pointer receiver, base is the receiver base type expression stripped
// of its type parameters (if any), and tparams are its type parameter names, if any. The
// type parameters are only unpacked if unpackParams is set. For instance, given the rtyp
//
//	*T[A, _]
//
// ptr is true, base is T, and tparams is [A, _] (assuming unpackParams is set).
// Note that base may not be a *ast.Ident for erroneous programs.
func (checks *Checker) unpackRecv(rtyp ast.Expr, unpackParams bool) (ptr bool, base ast.Expr, tparams []*ast.Ident) {
	// unpack receiver type
	base = ast.Unparen(rtyp)
	if t, _ := base.(*ast.StarExpr); t != nil {
		ptr = true
		base = ast.Unparen(t.X)
	}

	// unpack type parameters, if any
	switch base.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		ix := unpackIndexedExpr(base)
		base = ix.x
		if unpackParams {
			for _, arg := range ix.indices {
				var par *ast.Ident
				switch arg := arg.(type) {
				case *ast.Ident:
					par = arg
				case *ast.BadExpr:
					// ignore - error already reported by parser
				case nil:
					checks.error(ix.orig, InvalidSyntaxTree, "parameterized receiver contains nil parameters")
				default:
					checks.errorf(arg, BadDecl, "receiver type parameter %s must be an identifier", arg)
				}
				if par == nil {
					par = &ast.Ident{NamePos: arg.Pos(), Name: "_"}
				}
				tparams = append(tparams, par)
			}
		}
	}

	return
}

// resolveBaseTypeName returns the non-alias base type name for the given name, and whether
// there was a pointer indirection to get to it. The base type name must be declared
// in package scope, and there can be at most one pointer indirection. Traversals
// through generic alias types are not permitted. If no such type name exists, the
// returned base is nil.
func (checks *Checker) resolveBaseTypeName(ptr bool, name *ast.Ident) (ptr_ bool, base *TypeName) {
	// Algorithm: Starting from name, which is expected to denote a type,
	// we follow that type through non-generic alias declarations until
	// we reach a non-alias type name.
	var seen map[*TypeName]bool
	for name != nil {
		// name must denote an object found in the current package scope
		// (note that dot-imported objects are not in the package scope!)
		obj := checks.pkg.scope.Lookup(name.Name)
		if obj == nil {
			break
		}

		// the object must be a type name...
		tname, _ := obj.(*TypeName)
		if tname == nil {
			break
		}

		// ... which we have not seen before
		if seen[tname] {
			break
		}

		// we're done if tdecl describes a defined type (not an alias)
		tdecl := checks.objMap[tname].tdecl // must exist for objects in package scope
		if !tdecl.Assign.IsValid() {
			return ptr, tname
		}

		// an alias must not be generic
		// (importantly, we must not collect such methods - was https://go.dev/issue/70417)
		if tdecl.TypeParams != nil {
			break
		}

		// otherwise, remember this type name and continue resolving
		if seen == nil {
			seen = make(map[*TypeName]bool)
		}
		seen[tname] = true

		// The go/parser keeps parentheses; strip them, if any.
		typ := ast.Unparen(tdecl.Type)

		// dereference a pointer type
		if pexpr, _ := typ.(*ast.StarExpr); pexpr != nil {
			// if we've already seen a pointer, we're done
			if ptr {
				break
			}
			ptr = true
			typ = ast.Unparen(pexpr.X) // continue with pointer base type
		}

		// After dereferencing, typ must be a locally defined type name.
		// Referring to other packages (qualified identifiers) or going
		// through instantiated types (index expressions) is not permitted,
		// so we can ignore those.
		name, _ = typ.(*ast.Ident)
		if name == nil {
			break
		}
	}

	// no base type found
	return false, nil
}

// packageObjects typechecks all package objects, but not function bodies.
func (checks *Checker) packageObjects() {
	// process package objects in source order for reproducible results
	objList := make([]Object, len(checks.objMap))
	i := 0
	for obj := range checks.objMap {
		objList[i] = obj
		i++
	}
	slices.SortFunc(objList, func(a, b Object) int {
		return cmp.Compare(a.order(), b.order())
	})

	// add new methods to already type-checked types (from a prior Checker.Files call)
	for _, obj := range objList {
		if obj, _ := obj.(*TypeName); obj != nil && obj.typ != nil {
			checks.collectMethods(obj)
		}
	}

	if false && checks.conf._EnableAlias {
		// With Alias nodes we can process declarations in any order.
		//
		// TODO(adonovan): unfortunately, Alias nodes
		// (GODEBUG=gotypesalias=1) don't entirely resolve
		// problems with cycles. For example, in
		// GOROOT/test/typeparam/issue50259.go,
		//
		// 	type T[_ any] struct{}
		// 	type A T[B]
		// 	type B = T[A]
		//
		// TypeName A has Type Named during checking, but by
		// the time the unified export data is written out,
		// its Type is Invalid.
		//
		// Investigate and reenable this branch.
		for _, obj := range objList {
			checks.objDecl(obj, nil)
		}
	} else {
		// Without Alias nodes, we process non-alias type declarations first, followed by
		// alias declarations, and then everything else. This appears to avoid most situations
		// where the type of an alias is needed before it is available.
		// There may still be cases where this is not good enough (see also go.dev/issue/25838).
		// In those cases Checker.ident will report an error ("invalid use of type alias").
		var aliasList []*TypeName
		var othersList []Object // everything that's not a type
		// phase 1: non-alias type declarations
		for _, obj := range objList {
			if tname, _ := obj.(*TypeName); tname != nil {
				if checks.objMap[tname].tdecl.Assign.IsValid() {
					aliasList = append(aliasList, tname)
				} else {
					checks.objDecl(obj, nil)
				}
			} else {
				othersList = append(othersList, obj)
			}
		}
		// phase 2: alias type declarations
		for _, obj := range aliasList {
			checks.objDecl(obj, nil)
		}
		// phase 3: all other declarations
		for _, obj := range othersList {
			checks.objDecl(obj, nil)
		}
	}

	// At this point we may have a non-empty check.methods map; this means that not all
	// entries were deleted at the end of typeDecl because the respective receiver base
	// types were not found. In that case, an error was reported when declaring those
	// methods. We can now safely discard this map.
	checks.methods = nil
}

// unusedImports checks for unused imports.
func (checks *Checker) unusedImports() {
	// If function bodies are not checked, packages' uses are likely missing - don't check.
	if checks.conf.IgnoreFuncBodies {
		return
	}

	// spec: "It is illegal (...) to directly import a package without referring to
	// any of its exported identifiers. To import a package solely for its side-effects
	// (initialization), use the blank identifier as explicit package name."

	for _, obj := range checks.imports {
		if obj.name != "_" && !checks.usedPkgNames[obj] {
			checks.errorUnusedPkg(obj)
		}
	}
}

func (checks *Checker) errorUnusedPkg(obj *PkgName) {
	// If the package was imported with a name other than the final
	// import path element, show it explicitly in the error message.
	// Note that this handles both renamed imports and imports of
	// packages containing unconventional package declarations.
	// Note that this uses / always, even on Windows, because Go import
	// paths always use forward slashes.
	path := obj.imported.path
	elem := path
	if i := strings.LastIndex(elem, "/"); i >= 0 {
		elem = elem[i+1:]
	}
	if obj.name == "" || obj.name == "." || obj.name == elem {
		checks.softErrorf(obj, UnusedImport, "%q imported and not used", path)
	} else {
		checks.softErrorf(obj, UnusedImport, "%q imported as %s and not used", path, obj.name)
	}
}

// dir makes a good-faith attempt to return the directory
// portion of path. If path is empty, the result is ".".
// (Per the go/build package dependency tests, we cannot import
// path/filepath and simply use filepath.Dir.)
func dir(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i > 0 {
		return path[:i]
	}
	// i <= 0
	return "."
}
