// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the Check function, which drives type-checking.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"os"
	"sync/atomic"
)

// nopos indicates an unknown position
var nopos syntax.Pos

// debugging/development support
const debug = false // leave on during development

// position tracing for panics during type checking
const tracePos = false // TODO(markfreeman): check performance implications

// _aliasAny changes the behavior of [Scope.Lookup] for "any" in the
// [Universe] scope.
//
// This is necessary because while Alias creation is controlled by
// [Config.EnableAlias], the representation of "any" is a global. In
// [Scope.Lookup], we select this global representation based on the result of
// [aliasAny], but as a result need to guard against this behavior changing
// during the type checking pass. Therefore we implement the following rule:
// any number of goroutines can type check concurrently with the same
// EnableAlias value, but if any goroutine tries to type check concurrently
// with a different EnableAlias value, we panic.
//
// To achieve this, _aliasAny is a state machine:
//
//	0:        no type checking is occurring
//	negative: type checking is occurring without EnableAlias set
//	positive: type checking is occurring with EnableAlias set
var _aliasAny int32

func aliasAny() bool {
	return atomic.LoadInt32(&_aliasAny) >= 0 // default true
}

// exprInfo stores information about an untyped expression.
type exprInfo struct {
	isLhs bool // expression is lhs operand of a shift with delayed type-check
	mode  operandMode
	typ   *Basic
	val   constant.Value // constant value; or nil (if not a constant)
}

// An environment represents the environment within which an object is
// type-checked.
type environment struct {
	decl          *declInfo                 // package-level declaration whose init expression/function body is checked
	scope         *Scope                    // top-most scope for lookups
	version       goVersion                 // current accepted language version; changes across files
	iota          constant.Value            // value of iota in a constant declaration; nil otherwise
	errpos        syntax.Pos                // if valid, identifier position of a constant with inherited initializer
	inTParamList  bool                      // set if inside a type parameter list
	sig           *Signature                // function signature if inside a function; nil otherwise
	isPanic       map[*syntax.CallExpr]bool // set of panic call expressions (used for termination check)
	hasLabel      bool                      // set if a function makes use of labels (only ~1% of functions); unused outside functions
	hasCallOrRecv bool                      // set if an expression contains a function call or channel receive operation
}

// lookupScope looks up name in the current environment and if an object
// is found it returns the scope containing the object and the object.
// Otherwise it returns (nil, nil).
//
// Note that obj.Parent() may be different from the returned scope if the
// object was inserted into the scope and already had a parent at that
// time (see Scope.Insert). This can only happen for dot-imported objects
// whose parent is the scope of the package that exported them.
func (env *environment) lookupScope(name string) (*Scope, Object) {
	for s := env.scope; s != nil; s = s.parent {
		if obj := s.Lookup(name); obj != nil {
			return s, obj
		}
	}
	return nil, nil
}

// lookup is like lookupScope but it only returns the object (or nil).
func (env *environment) lookup(name string) Object {
	_, obj := env.lookupScope(name)
	return obj
}

// An importKey identifies an imported package by import path and source directory
// (directory containing the file containing the import). In practice, the directory
// may always be the same, or may not matter. Given an (import path, directory), an
// importer must always return the same package (but given two different import paths,
// an importer may still return the same package by mapping them to the same package
// paths).
type importKey struct {
	path, dir string
}

// A dotImportKey describes a dot-imported object in the given scope.
type dotImportKey struct {
	scope *Scope
	name  string
}

// An action describes a (delayed) action.
type action struct {
	version goVersion   // applicable language version
	f       func()      // action to be executed
	desc    *actionDesc // action description; may be nil, requires debug to be set
}

// If debug is set, describef sets a printf-formatted description for action a.
// Otherwise, it is a no-op.
func (a *action) describef(pos poser, format string, args ...interface{}) {
	if debug {
		a.desc = &actionDesc{pos, format, args}
	}
}

// An actionDesc provides information on an action.
// For debugging only.
type actionDesc struct {
	pos    poser
	format string
	args   []interface{}
}

// A Checker maintains the state of the type checker.
// It must be created with NewChecker.
type Checker struct {
	// package information
	// (initialized by NewChecker, valid for the life-time of checker)
	conf *Config
	ctxt *Context // context for de-duplicating instances
	pkg  *Package
	*Info
	nextID uint64                 // unique Id for type parameters (first valid Id is 1)
	objMap map[Object]*declInfo   // maps package-level objects and (non-interface) methods to declaration info
	impMap map[importKey]*Package // maps (import path, source directory) to (complete or fake) package
	// see TODO in validtype.go
	// valids  instanceLookup      // valid *Named (incl. instantiated) types per the validType check

	// pkgPathMap maps package names to the set of distinct import paths we've
	// seen for that name, anywhere in the import graph. It is used for
	// disambiguating package names in error messages.
	//
	// pkgPathMap is allocated lazily, so that we don't pay the price of building
	// it on the happy path. seenPkgMap tracks the packages that we've already
	// walked.
	pkgPathMap map[string]map[string]bool
	seenPkgMap map[*Package]bool

	// information collected during type-checking of a set of package files
	// (initialized by Files, valid only for the duration of check.Files;
	// maps and lists are allocated on demand)
	files         []*syntax.File             // list of package files
	versions      map[*syntax.PosBase]string // maps files to version strings (each file has an entry); shared with Info.FileVersions if present; may be unaltered Config.GoVersion
	imports       []*PkgName                 // list of imported packages
	dotImportMap  map[dotImportKey]*PkgName  // maps dot-imported objects to the package they were dot-imported through
	brokenAliases map[*TypeName]bool         // set of aliases with broken (not yet determined) types
	unionTypeSets map[*Union]*_TypeSet       // computed type sets for union types
	usedVars      map[*Var]bool              // set of used variables
	usedPkgNames  map[*PkgName]bool          // set of used package names
	mono          monoGraph                  // graph for detecting non-monomorphizable instantiation loops

	firstErr error                    // first error encountered
	methods  map[*TypeName][]*Func    // maps package scope type names to associated non-blank (non-interface) methods
	untyped  map[syntax.Expr]exprInfo // map of expressions without final type
	delayed  []action                 // stack of delayed action segments; segments are processed in FIFO order
	objPath  []Object                 // path of object dependencies during type inference (for cycle reporting)
	cleaners []cleaner                // list of types that may need a final cleanup at the end of type-checking

	// environment within which the current object is type-checked (valid only
	// for the duration of type-checking a specific object)
	environment

	// debugging
	posStack []syntax.Pos // stack of source positions seen; used for panic tracing
	indent   int          // indentation for tracing
}

// addDeclDep adds the dependency edge (check.decl -> to) if check.decl exists
func (checks *Checker) addDeclDep(to Object) {
	from := checks.decl
	if from == nil {
		return // not in a package-level init expression
	}
	if _, found := checks.objMap[to]; !found {
		return // to is not a package-level object
	}
	from.addDep(to)
}

// Note: The following three alias-related functions are only used
//       when Alias types are not enabled.

// brokenAlias records that alias doesn't have a determined type yet.
// It also sets alias.typ to Typ[Invalid].
// Not used if check.conf.EnableAlias is set.
func (checks *Checker) brokenAlias(alias *TypeName) {
	assert(!checks.conf.EnableAlias)
	if checks.brokenAliases == nil {
		checks.brokenAliases = make(map[*TypeName]bool)
	}
	checks.brokenAliases[alias] = true
	alias.typ = Typ[Invalid]
}

// validAlias records that alias has the valid type typ (possibly Typ[Invalid]).
func (checks *Checker) validAlias(alias *TypeName, typ Type) {
	assert(!checks.conf.EnableAlias)
	delete(checks.brokenAliases, alias)
	alias.typ = typ
}

// isBrokenAlias reports whether alias doesn't have a determined type yet.
func (checks *Checker) isBrokenAlias(alias *TypeName) bool {
	assert(!checks.conf.EnableAlias)
	return checks.brokenAliases[alias]
}

func (checks *Checker) rememberUntyped(e syntax.Expr, lhs bool, mode operandMode, typ *Basic, val constant.Value) {
	m := checks.untyped
	if m == nil {
		m = make(map[syntax.Expr]exprInfo)
		checks.untyped = m
	}
	m[e] = exprInfo{lhs, mode, typ, val}
}

// later pushes f on to the stack of actions that will be processed later;
// either at the end of the current statement, or in case of a local constant
// or variable declaration, before the constant or variable is in scope
// (so that f still sees the scope before any new declarations).
// later returns the pushed action so one can provide a description
// via action.describef for debugging, if desired.
func (checks *Checker) later(f func()) *action {
	i := len(checks.delayed)
	checks.delayed = append(checks.delayed, action{version: checks.version, f: f})
	return &checks.delayed[i]
}

// push pushes obj onto the object path and returns its index in the path.
func (checks *Checker) push(obj Object) int {
	checks.objPath = append(checks.objPath, obj)
	return len(checks.objPath) - 1
}

// pop pops and returns the topmost object from the object path.
func (checks *Checker) pop() Object {
	i := len(checks.objPath) - 1
	obj := checks.objPath[i]
	checks.objPath[i] = nil
	checks.objPath = checks.objPath[:i]
	return obj
}

type cleaner interface {
	cleanup()
}

// needsCleanup records objects/types that implement the cleanup method
// which will be called at the end of type-checking.
func (checks *Checker) needsCleanup(c cleaner) {
	checks.cleaners = append(checks.cleaners, c)
}

// NewChecker returns a new Checker instance for a given package.
// Package files may be added incrementally via checker.Files.
func NewChecker(conf *Config, pkg *Package, info *Info) *Checker {
	// make sure we have a configuration
	if conf == nil {
		conf = new(Config)
	}

	// make sure we have an info struct
	if info == nil {
		info = new(Info)
	}

	// Note: clients may call NewChecker with the Unsafe package, which is
	// globally shared and must not be mutated. Therefore NewChecker must not
	// mutate *pkg.
	//
	// (previously, pkg.goVersion was mutated here: go.dev/issue/61212)

	return &Checker{
		conf:         conf,
		ctxt:         conf.Context,
		pkg:          pkg,
		Info:         info,
		objMap:       make(map[Object]*declInfo),
		impMap:       make(map[importKey]*Package),
		usedVars:     make(map[*Var]bool),
		usedPkgNames: make(map[*PkgName]bool),
	}
}

// initFiles initializes the files-specific portion of checker.
// The provided files must all belong to the same package.
func (checks *Checker) initFiles(files []*syntax.File) {
	// start with a clean slate (check.Files may be called multiple times)
	// TODO(gri): what determines which fields are zeroed out here, vs at the end
	// of checkFiles?
	checks.files = nil
	checks.imports = nil
	checks.dotImportMap = nil

	checks.firstErr = nil
	checks.methods = nil
	checks.untyped = nil
	checks.delayed = nil
	checks.objPath = nil
	checks.cleaners = nil

	// We must initialize usedVars and usedPkgNames both here and in NewChecker,
	// because initFiles is not called in the CheckExpr or Eval codepaths, yet we
	// want to free this memory at the end of Files ('used' predicates are
	// only needed in the context of a given file).
	checks.usedVars = make(map[*Var]bool)
	checks.usedPkgNames = make(map[*PkgName]bool)

	// determine package name and collect valid files
	pkg := checks.pkg
	for _, file := range files {
		switch name := file.PkgName.Value; pkg.name {
		case "":
			if name != "_" {
				pkg.name = name
			} else {
				checks.error(file.PkgName, BlankPkgName, "invalid package name _")
			}
			fallthrough

		case name:
			checks.files = append(checks.files, file)

		default:
			checks.errorf(file, MismatchedPkgName, "package %s; expected package %s", name, pkg.name)
			// ignore this file
		}
	}

	// reuse Info.FileVersions if provided
	versions := checks.Info.FileVersions
	if versions == nil {
		versions = make(map[*syntax.PosBase]string)
	}
	checks.versions = versions

	pkgVersion := asGoVersion(checks.conf.GoVersion)
	if pkgVersion.isValid() && len(files) > 0 && pkgVersion.cmp(go_current) > 0 {
		checks.errorf(files[0], TooNew, "package requires newer Go version %v (application built with %v)",
			pkgVersion, go_current)
	}

	// determine Go version for each file
	for _, file := range checks.files {
		// use unaltered Config.GoVersion by default
		// (This version string may contain dot-release numbers as in go1.20.1,
		// unlike file versions which are Go language versions only, if valid.)
		v := checks.conf.GoVersion

		// If the file specifies a version, use max(fileVersion, go1.21).
		if fileVersion := asGoVersion(file.GoVersion); fileVersion.isValid() {
			// Go 1.21 introduced the feature of allowing //go:build lines
			// to sometimes set the Go version in a given file. Versions Go 1.21 and later
			// can be set backwards compatibly as that was the first version
			// files with go1.21 or later build tags could be built with.
			//
			// Set the version to max(fileVersion, go1.21): That will allow a
			// downgrade to a version before go1.22, where the for loop semantics
			// change was made, while being backwards compatible with versions of
			// go before the new //go:build semantics were introduced.
			v = string(versionMax(fileVersion, go1_21))

			// Report a specific error for each tagged file that's too new.
			// (Normally the build system will have filtered files by version,
			// but clients can present arbitrary files to the type checker.)
			if fileVersion.cmp(go_current) > 0 {
				// Use position of 'package [p]' for types/types2 consistency.
				// (Ideally we would use the //build tag itself.)
				checks.errorf(file.PkgName, TooNew, "file requires newer Go version %v", fileVersion)
			}
		}
		versions[file.Pos().FileBase()] = v // file.Pos().FileBase() may be nil for tests
	}
}

func versionMax(a, b goVersion) goVersion {
	if a.cmp(b) > 0 {
		return a
	}
	return b
}

// pushPos pushes pos onto the pos stack.
func (checks *Checker) pushPos(pos syntax.Pos) {
	checks.posStack = append(checks.posStack, pos)
}

// popPos pops from the pos stack.
func (checks *Checker) popPos() {
	checks.posStack = checks.posStack[:len(checks.posStack)-1]
}

// A bailout panic is used for early termination.
type bailout struct{}

func (checks *Checker) handleBailout(err *error) {
	switch p := recover().(type) {
	case nil, bailout:
		// normal return or early exit
		*err = checks.firstErr
	default:
		if len(checks.posStack) > 0 {
			doPrint := func(ps []syntax.Pos) {
				for i := len(ps) - 1; i >= 0; i-- {
					fmt.Fprintf(os.Stderr, "\t%v\n", ps[i])
				}
			}

			fmt.Fprintln(os.Stderr, "The following panic happened checking types near:")
			if len(checks.posStack) <= 10 {
				doPrint(checks.posStack)
			} else {
				// if it's long, truncate the middle; it's least likely to help
				doPrint(checks.posStack[len(checks.posStack)-5:])
				fmt.Fprintln(os.Stderr, "\t...")
				doPrint(checks.posStack[:5])
			}
		}

		// re-panic
		panic(p)
	}
}

// Files checks the provided files as part of the checker's package.
func (checks *Checker) Files(files []*syntax.File) (err error) {
	if checks.pkg == Unsafe {
		// Defensive handling for Unsafe, which cannot be type checked, and must
		// not be mutated. See https://go.dev/issue/61212 for an example of where
		// Unsafe is passed to NewChecker.
		return nil
	}

	// Avoid early returns here! Nearly all errors can be
	// localized to a piece of syntax and needn't prevent
	// type-checking of the rest of the package.

	defer checks.handleBailout(&err)
	checks.checkFiles(files)
	return
}

// checkFiles type-checks the specified files. Errors are reported as
// a side effect, not by returning early, to ensure that well-formed
// syntax is properly type annotated even in a package containing
// errors.
func (checks *Checker) checkFiles(files []*syntax.File) {
	// Ensure that EnableAlias is consistent among concurrent type checking
	// operations. See the documentation of [_aliasAny] for details.
	if checks.conf.EnableAlias {
		if atomic.AddInt32(&_aliasAny, 1) <= 0 {
			panic("EnableAlias set while !EnableAlias type checking is ongoing")
		}
		defer atomic.AddInt32(&_aliasAny, -1)
	} else {
		if atomic.AddInt32(&_aliasAny, -1) >= 0 {
			panic("!EnableAlias set while EnableAlias type checking is ongoing")
		}
		defer atomic.AddInt32(&_aliasAny, 1)
	}

	print := func(msg string) {
		if checks.conf.Trace {
			fmt.Println()
			fmt.Println(msg)
		}
	}

	print("== initFiles ==")
	checks.initFiles(files)

	print("== collectObjects ==")
	checks.collectObjects()

	print("== packageObjects ==")
	checks.packageObjects()

	print("== processDelayed ==")
	checks.processDelayed(0) // incl. all functions

	print("== cleanup ==")
	checks.cleanup()

	print("== initOrder ==")
	checks.initOrder()

	if !checks.conf.DisableUnusedImportCheck {
		print("== unusedImports ==")
		checks.unusedImports()
	}

	print("== recordUntyped ==")
	checks.recordUntyped()

	if checks.firstErr == nil {
		// TODO(mdempsky): Ensure monomorph is safe when errors exist.
		checks.monomorph()
	}

	checks.pkg.goVersion = checks.conf.GoVersion
	checks.pkg.complete = true

	// no longer needed - release memory
	checks.imports = nil
	checks.dotImportMap = nil
	checks.pkgPathMap = nil
	checks.seenPkgMap = nil
	checks.brokenAliases = nil
	checks.unionTypeSets = nil
	checks.usedVars = nil
	checks.usedPkgNames = nil
	checks.ctxt = nil

	// TODO(gri): shouldn't the cleanup above occur after the bailout?
	// TODO(gri) There's more memory we should release at this point.
}

// processDelayed processes all delayed actions pushed after top.
func (checks *Checker) processDelayed(top int) {
	// If each delayed action pushes a new action, the
	// stack will continue to grow during this loop.
	// However, it is only processing functions (which
	// are processed in a delayed fashion) that may
	// add more actions (such as nested functions), so
	// this is a sufficiently bounded process.
	savedVersion := checks.version
	for i := top; i < len(checks.delayed); i++ {
		a := &checks.delayed[i]
		if checks.conf.Trace {
			if a.desc != nil {
				checks.trace(a.desc.pos.Pos(), "-- "+a.desc.format, a.desc.args...)
			} else {
				checks.trace(nopos, "-- delayed %p", a.f)
			}
		}
		checks.version = a.version // reestablish the effective Go version captured earlier
		a.f()                      // may append to check.delayed
		if checks.conf.Trace {
			fmt.Println()
		}
	}
	assert(top <= len(checks.delayed)) // stack must not have shrunk
	checks.delayed = checks.delayed[:top]
	checks.version = savedVersion
}

// cleanup runs cleanup for all collected cleaners.
func (checks *Checker) cleanup() {
	// Don't use a range clause since Named.cleanup may add more cleaners.
	for i := 0; i < len(checks.cleaners); i++ {
		checks.cleaners[i].cleanup()
	}
	checks.cleaners = nil
}

// types2-specific support for recording type information in the syntax tree.
func (checks *Checker) recordTypeAndValueInSyntax(x syntax.Expr, mode operandMode, typ Type, val constant.Value) {
	if checks.StoreTypesInSyntax {
		tv := TypeAndValue{mode, typ, val}
		stv := syntax.TypeAndValue{Type: typ, Value: val}
		if tv.IsVoid() {
			stv.SetIsVoid()
		}
		if tv.IsType() {
			stv.SetIsType()
		}
		if tv.IsBuiltin() {
			stv.SetIsBuiltin()
		}
		if tv.IsValue() {
			stv.SetIsValue()
		}
		if tv.IsNil() {
			stv.SetIsNil()
		}
		if tv.Addressable() {
			stv.SetAddressable()
		}
		if tv.Assignable() {
			stv.SetAssignable()
		}
		if tv.HasOk() {
			stv.SetHasOk()
		}
		x.SetTypeInfo(stv)
	}
}

// types2-specific support for recording type information in the syntax tree.
func (checks *Checker) recordCommaOkTypesInSyntax(x syntax.Expr, t0, t1 Type) {
	if checks.StoreTypesInSyntax {
		// Note: this loop is duplicated because the type of tv is different.
		// Above it is types2.TypeAndValue, here it is syntax.TypeAndValue.
		for {
			tv := x.GetTypeInfo()
			assert(tv.Type != nil) // should have been recorded already
			pos := x.Pos()
			tv.Type = NewTuple(
				NewParam(pos, checks.pkg, "", t0),
				NewParam(pos, checks.pkg, "", t1),
			)
			x.SetTypeInfo(tv)
			p, _ := x.(*syntax.ParenExpr)
			if p == nil {
				break
			}
			x = p.X
		}
	}
}

// instantiatedIdent determines the identifier of the type instantiated in expr.
// Helper function for recordInstance in recording.go.
func instantiatedIdent(expr syntax.Expr) *syntax.Name {
	var selOrIdent syntax.Expr
	switch e := expr.(type) {
	case *syntax.IndexExpr:
		selOrIdent = e.X
	case *syntax.SelectorExpr, *syntax.Name:
		selOrIdent = e
	}
	switch x := selOrIdent.(type) {
	case *syntax.Name:
		return x
	case *syntax.SelectorExpr:
		return x.Sel
	}

	// extra debugging of go.dev/issue/63933
	panic(sprintf(nil, true, "instantiated ident not found; please report: %s", expr))
}
