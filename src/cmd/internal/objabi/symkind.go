// Derived from Inferno utils/6l/l.h and related files.
// https://bitbucket.org/inferno-os/inferno-os/src/master/utils/6l/l.h
//
//	Copyright © 1994-1999 Lucent Technologies Inc.  All rights reserved.
//	Portions Copyright © 1995-1997 C H Forsyth (forsyth@terzarima.net)
//	Portions Copyright © 1997-1999 Vita Nuova Limited
//	Portions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com)
//	Portions Copyright © 2004,2006 Bruce Ellis
//	Portions Copyright © 2005-2007 C H Forsyth (forsyth@terzarima.net)
//	Revisions Copyright © 2000-2007 Lucent Technologies Inc. and others
//	Portions Copyright © 2009 The Go Authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package objabi

import "strconv"

// A SymKind describes the kind of memory represented by a symbol.
type SymKind uint8

// Defined SymKind values.
// These are used to index into cmd/link/internal/sym/AbiSymKindToSymKind
//
// TODO(rsc): Give idiomatic Go names.
//
const (
	// An otherwise invalid zero value for the type
	Sxxx SymKind = iota
	// Executable instructions
	STEXT
	STEXTFIPS
	// Read only static data
	SRODATA
	SRODATAFIPS
	// Static data that does not contain any pointers
	SNOPTRDATA
	SNOPTRDATAFIPS
	// Static data
	SDATA
	SDATAFIPS
	// Statically data that is initially all 0s
	SBSS
	// Statically data that is initially all 0s and does not contain pointers
	SNOPTRBSS
	// Thread-local data that is initially all 0s
	STLSBSS
	// Debugging data
	SDWARFCUINFO
	SDWARFCONST
	SDWARFFCN
	SDWARFABSFCN
	SDWARFTYPE
	SDWARFVAR
	SDWARFRANGE
	SDWARFLOC
	SDWARFLINES
	SDWARFADDR
	// Coverage instrumentation counter for libfuzzer.
	SLIBFUZZER_8BIT_COUNTER
	// Coverage instrumentation counter, aux variable for cmd/cover
	SCOVERAGE_COUNTER
	SCOVERAGE_AUXVAR

	SSEHUNWINDINFO
	// Update cmd/link/internal/sym/AbiSymKindToSymKind for new SymKind values.
)

// SymKindNames provides string representations for SymKind constants, replacing stringer-generated symkind_string.go
var SymKindNames = [...]string{
	Sxxx:                    "Sxxx",
	STEXT:                   "STEXT",
	STEXTFIPS:               "STEXTFIPS",
	SRODATA:                 "SRODATA",
	SRODATAFIPS:             "SRODATAFIPS",
	SNOPTRDATA:              "SNOPTRDATA",
	SNOPTRDATAFIPS:          "SNOPTRDATAFIPS",
	SDATA:                   "SDATA",
	SDATAFIPS:               "SDATAFIPS",
	SBSS:                    "SBSS",
	SNOPTRBSS:               "SNOPTRBSS",
	STLSBSS:                 "STLSBSS",
	SDWARFCUINFO:            "SDWARFCUINFO",
	SDWARFCONST:             "SDWARFCONST",
	SDWARFFCN:               "SDWARFFCN",
	SDWARFABSFCN:            "SDWARFABSFCN",
	SDWARFTYPE:              "SDWARFTYPE",
	SDWARFVAR:               "SDWARFVAR",
	SDWARFRANGE:             "SDWARFRANGE",
	SDWARFLOC:               "SDWARFLOC",
	SDWARFLINES:             "SDWARFLINES",
	SDWARFADDR:              "SDWARFADDR",
	SLIBFUZZER_8BIT_COUNTER: "SLIBFUZZER_8BIT_COUNTER",
	SCOVERAGE_COUNTER:       "SCOVERAGE_COUNTER",
	SCOVERAGE_AUXVAR:        "SCOVERAGE_AUXVAR",
	SSEHUNWINDINFO:          "SSEHUNWINDINFO",
}

// String returns the string representation of the SymKind.
// This replaces the stringer-generated String() method.
func (s SymKind) String() string {
	if int(s) < len(SymKindNames) && SymKindNames[s] != "" {
		return SymKindNames[s]
	}
	return "SymKind(" + strconv.FormatInt(int64(s), 10) + ")"
}

// IsText reports whether t is one of the text kinds.
func (t SymKind) IsText() bool {
	return t == STEXT || t == STEXTFIPS
}

// IsDATA reports whether t is one of the DATA kinds (SDATA or SDATAFIPS,
// excluding NOPTRDATA, RODATA, BSS, and so on).
func (t SymKind) IsDATA() bool {
	return t == SDATA || t == SDATAFIPS
}

// IsFIPS reports whether t is one fo the FIPS kinds.
func (t SymKind) IsFIPS() bool {
	return t == STEXTFIPS || t == SRODATAFIPS || t == SNOPTRDATAFIPS || t == SDATAFIPS
}
