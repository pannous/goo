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

package sym

import (
	"cmd/internal/objabi"
	"strconv"
)

// A SymKind describes the kind of memory represented by a symbol.
type SymKind uint8

// Defined SymKind values.
//
// TODO(rsc): Give idiomatic Go names.
//
const (
	// An otherwise invalid zero value for the type.
	Sxxx SymKind = iota
	// The text segment, containing executable instructions.
	STEXT          // General executable code.
	STEXTFIPSSTART // Start of FIPS text section.
	STEXTFIPS      // Instructions hashed for FIPS checks.
	STEXTFIPSEND   // End of FIPS text section.
	STEXTEND       // End of text section.
	SELFRXSECT     // Executable PLT; PPC64 .glink.
	SMACHOPLT      // Mach-O PLT.

	// Read-only, non-executable, segment.
	STYPE            // Type descriptors.
	SSTRING          // Used only for XCOFF runtime.rodata symbol?
	SGOSTRING        // Go string constants.
	SGOFUNC          // Function descriptors and funcdata symbols.
	SGCBITS          // GC bit masks and programs.
	SRODATA          // General read-only data.
	SRODATAFIPSSTART // Start of FIPS read-only data.
	SRODATAFIPS      // FIPS read-only data.
	SRODATAFIPSEND   // End of FIPS read-only data.
	SRODATAEND       // End of read-only data.
	SFUNCTAB         // Appears to be unused, except for runtime.etypes.
	SPCLNTAB         // Pclntab data.
	SELFROSECT       // ELF read-only data: relocs, dynamic linking info.

	// Read-only, non-executable, dynamically relocatable segment.
	//
	// Types STYPE-SFUNCTAB above are written to the .rodata section by default.
	// When linking a shared object, some conceptually "read only" types need to
	// be written to by relocations and putting them in a section called
	// ".rodata" interacts poorly with the system linkers. The GNU linkers
	// support this situation by arranging for sections of the name
	// ".data.rel.ro.XXX" to be mprotected read only by the dynamic linker after
	// relocations have applied, so when the Go linker is creating a shared
	// object it checks all objects of the above types and bumps any object that
	// has a relocation to it to the corresponding type below, which are then
	// written to sections with appropriate magic names.
	STYPERELRO
	SSTRINGRELRO
	SGOSTRINGRELRO
	SGOFUNCRELRO
	SGCBITSRELRO
	SRODATARELRO
	SFUNCTABRELRO

	SELFRELROSECT   // ELF-specific read-only relocatable: PLT, etc.
	SMACHORELROSECT // Mach-O specific read-only relocatable.

	STYPELINK // Type links.
	SITABLINK // Itab links.

	// Allocated writable segment.
	SFirstWritable
	SBUILDINFO          // debug/buildinfo data (why is this writable?).
	SFIPSINFO           // go:fipsinfo aka crypto/internal/fips140/check.Linkinfo (why is this writable)?
	SELFSECT            // .got.plt, .plt, .dynamic where appropriate.
	SMACHO              // Used only for .llvmasm?
	SWINDOWS            // Windows dynamic symbols.
	SMODULEDATA         // Linker generated moduledata struct.
	SELFGOT             // Writable ELF GOT section.
	SMACHOGOT           // Mach-O GOT.
	SNOPTRDATA          // Data with no heap pointers.
	SNOPTRDATAFIPSSTART // Start of FIPS non-pointer writable data.
	SNOPTRDATAFIPS      // FIPS non-pointer writable data.
	SNOPTRDATAFIPSEND   // End of FIPS non-pointer writable data.
	SNOPTRDATAEND       // End of data with no heap pointers.
	SINITARR            // ELF .init_array section.
	SDATA               // Data that may have heap pointers.
	SDATAFIPSSTART      // Start of FIPS writable data.
	SDATAFIPS           // FIPS writable data.
	SDATAFIPSEND        // End of FIPS writable data.
	SDATAEND            // End of data that may have heap pointers.
	SXCOFFTOC           // AIX TOC entries.

	// Allocated zero-initialized segment.
	SBSS                    // Zeroed data that may have heap pointers.
	SNOPTRBSS               // Zeroed data with no heap pointers.
	SLIBFUZZER_8BIT_COUNTER // Fuzzer counters.
	SCOVERAGE_COUNTER       // Coverage counters.
	SCOVERAGE_AUXVAR        // Compiler generated coverage symbols.
	STLSBSS                 // Thread-local zeroed data.

	// Unallocated segment.
	SFirstUnallocated
	SXREF             // Reference from non-Go object file.
	SMACHOSYMSTR      // Mach-O string table.
	SMACHOSYMTAB      // Mach-O symbol table.
	SMACHOINDIRECTPLT // Mach-O indirect PLT.
	SMACHOINDIRECTGOT // Mach-O indirect GOT.
	SDYNIMPORT        // Reference to symbol defined in shared library.
	SHOSTOBJ          // Symbol defined in non-Go object file.
	SUNDEFEXT         // Undefined symbol for resolution by external linker.

	// Unallocated DWARF debugging segment.
	SDWARFSECT
	// DWARF symbol types created by compiler or linker.
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

	// SEH symbol types. These are probably allocated at run time.
	SSEHUNWINDINFO // Compiler generated Windows SEH info.
	SSEHSECT       // Windows SEH data.
)

// AbiSymKindToSymKind maps values read from object files (which are
// of type cmd/internal/objabi.SymKind) to values of type SymKind.
var AbiSymKindToSymKind = [...]SymKind{
	objabi.Sxxx:                    Sxxx,
	objabi.STEXT:                   STEXT,
	objabi.STEXTFIPS:               STEXTFIPS,
	objabi.SRODATA:                 SRODATA,
	objabi.SRODATAFIPS:             SRODATAFIPS,
	objabi.SNOPTRDATA:              SNOPTRDATA,
	objabi.SNOPTRDATAFIPS:          SNOPTRDATAFIPS,
	objabi.SDATA:                   SDATA,
	objabi.SDATAFIPS:               SDATAFIPS,
	objabi.SBSS:                    SBSS,
	objabi.SNOPTRBSS:               SNOPTRBSS,
	objabi.STLSBSS:                 STLSBSS,
	objabi.SDWARFCUINFO:            SDWARFCUINFO,
	objabi.SDWARFCONST:             SDWARFCONST,
	objabi.SDWARFFCN:               SDWARFFCN,
	objabi.SDWARFABSFCN:            SDWARFABSFCN,
	objabi.SDWARFTYPE:              SDWARFTYPE,
	objabi.SDWARFVAR:               SDWARFVAR,
	objabi.SDWARFRANGE:             SDWARFRANGE,
	objabi.SDWARFLOC:               SDWARFLOC,
	objabi.SDWARFLINES:             SDWARFLINES,
	objabi.SDWARFADDR:              SDWARFADDR,
	objabi.SLIBFUZZER_8BIT_COUNTER: SLIBFUZZER_8BIT_COUNTER,
	objabi.SCOVERAGE_COUNTER:       SCOVERAGE_COUNTER,
	objabi.SCOVERAGE_AUXVAR:        SCOVERAGE_AUXVAR,
	objabi.SSEHUNWINDINFO:          SSEHUNWINDINFO,
}

// ReadOnly are the symbol kinds that form read-only sections. In some
// cases, if they will require relocations, they are transformed into
// rel-ro sections using relROMap.
var ReadOnly = []SymKind{
	STYPE,
	SSTRING,
	SGOSTRING,
	SGOFUNC,
	SGCBITS,
	SRODATA,
	SRODATAFIPSSTART,
	SRODATAFIPS,
	SRODATAFIPSEND,
	SRODATAEND,
	SFUNCTAB,
}

// RelROMap describes the transformation of read-only symbols to rel-ro
// symbols.
var RelROMap = map[SymKind]SymKind{
	STYPE:     STYPERELRO,
	SSTRING:   SSTRINGRELRO,
	SGOSTRING: SGOSTRINGRELRO,
	SGOFUNC:   SGOFUNCRELRO,
	SGCBITS:   SGCBITSRELRO,
	SRODATA:   SRODATARELRO,
	SFUNCTAB:  SFUNCTABRELRO,
}

// IsText returns true if t is a text type.
func (t SymKind) IsText() bool {
	return STEXT <= t && t <= STEXTEND
}

// IsData returns true if t is any kind of data type.
func (t SymKind) IsData() bool {
	return SNOPTRDATA <= t && t <= SNOPTRBSS
}

// IsDATA returns true if t is one of the SDATA types.
func (t SymKind) IsDATA() bool {
	return SDATA <= t && t <= SDATAEND
}

// IsRODATA returns true if t is one of the SRODATA types.
func (t SymKind) IsRODATA() bool {
	return SRODATA <= t && t <= SRODATAEND
}

// IsNOPTRDATA returns true if t is one of the SNOPTRDATA types.
func (t SymKind) IsNOPTRDATA() bool {
	return SNOPTRDATA <= t && t <= SNOPTRDATAEND
}

func (t SymKind) IsDWARF() bool {
	return SDWARFSECT <= t && t <= SDWARFADDR
}

// String returns the string representation of SymKind
func (s SymKind) String() string {
	switch s {
	case Sxxx:
		return "Sxxx"
	case STEXT:
		return "STEXT"
	case STEXTFIPSSTART:
		return "STEXTFIPSSTART"
	case STEXTFIPS:
		return "STEXTFIPS"
	case STEXTFIPSEND:
		return "STEXTFIPSEND"
	case STEXTEND:
		return "STEXTEND"
	case SELFRXSECT:
		return "SELFRXSECT"
	case SMACHOPLT:
		return "SMACHOPLT"
	case STYPE:
		return "STYPE"
	case SSTRING:
		return "SSTRING"
	case SGOSTRING:
		return "SGOSTRING"
	case SGOFUNC:
		return "SGOFUNC"
	case SGCBITS:
		return "SGCBITS"
	case SRODATA:
		return "SRODATA"
	case SRODATAFIPSSTART:
		return "SRODATAFIPSSTART"
	case SRODATAFIPS:
		return "SRODATAFIPS"
	case SRODATAFIPSEND:
		return "SRODATAFIPSEND"
	case SRODATAEND:
		return "SRODATAEND"
	case SFUNCTAB:
		return "SFUNCTAB"
	case SELFROSECT:
		return "SELFROSECT"
	case STYPERELRO:
		return "STYPERELRO"
	case SSTRINGRELRO:
		return "SSTRINGRELRO"
	case SGOSTRINGRELRO:
		return "SGOSTRINGRELRO"
	case SGOFUNCRELRO:
		return "SGOFUNCRELRO"
	case SGCBITSRELRO:
		return "SGCBITSRELRO"
	case SRODATARELRO:
		return "SRODATARELRO"
	case SFUNCTABRELRO:
		return "SFUNCTABRELRO"
	case SELFRELROSECT:
		return "SELFRELROSECT"
	case SMACHORELROSECT:
		return "SMACHORELROSECT"
	case STYPELINK:
		return "STYPELINK"
	case SITABLINK:
		return "SITABLINK"
	case SSYMTAB:
		return "SSYMTAB"
	case SPCLNTAB:
		return "SPCLNTAB"
	case SFirstWritable:
		return "SFirstWritable"
	case SBUILDINFO:
		return "SBUILDINFO"
	case SFIPSINFO:
		return "SFIPSINFO"
	case SELFSECT:
		return "SELFSECT"
	case SMACHO:
		return "SMACHO"
	case SMACHOGOT:
		return "SMACHOGOT"
	case SWINDOWS:
		return "SWINDOWS"
	case SELFGOT:
		return "SELFGOT"
	case SNOPTRDATA:
		return "SNOPTRDATA"
	case SNOPTRDATAFIPSSTART:
		return "SNOPTRDATAFIPSSTART"
	case SNOPTRDATAFIPS:
		return "SNOPTRDATAFIPS"
	case SNOPTRDATAFIPSEND:
		return "SNOPTRDATAFIPSEND"
	case SNOPTRDATAEND:
		return "SNOPTRDATAEND"
	case SINITARR:
		return "SINITARR"
	case SDATA:
		return "SDATA"
	case SDATAFIPSSTART:
		return "SDATAFIPSSTART"
	case SDATAFIPS:
		return "SDATAFIPS"
	case SDATAFIPSEND:
		return "SDATAFIPSEND"
	case SDATAEND:
		return "SDATAEND"
	case SXCOFFTOC:
		return "SXCOFFTOC"
	case SBSS:
		return "SBSS"
	case SNOPTRBSS:
		return "SNOPTRBSS"
	case SLIBFUZZER_8BIT_COUNTER:
		return "SLIBFUZZER_8BIT_COUNTER"
	case SCOVERAGE_COUNTER:
		return "SCOVERAGE_COUNTER"
	case SCOVERAGE_AUXVAR:
		return "SCOVERAGE_AUXVAR"
	case STLSBSS:
		return "STLSBSS"
	case SXREF:
		return "SXREF"
	case SMACHOSYMSTR:
		return "SMACHOSYMSTR"
	case SMACHOSYMTAB:
		return "SMACHOSYMTAB"
	case SMACHOINDIRECTPLT:
		return "SMACHOINDIRECTPLT"
	case SMACHOINDIRECTGOT:
		return "SMACHOINDIRECTGOT"
	case SFILEPATH:
		return "SFILEPATH"
	case SDYNIMPORT:
		return "SDYNIMPORT"
	case SHOSTOBJ:
		return "SHOSTOBJ"
	case SUNDEFEXT:
		return "SUNDEFEXT"
	case SDWARFSECT:
		return "SDWARFSECT"
	case SDWARFCUINFO:
		return "SDWARFCUINFO"
	case SDWARFCONST:
		return "SDWARFCONST"
	case SDWARFFCN:
		return "SDWARFFCN"
	case SDWARFABSFCN:
		return "SDWARFABSFCN"
	case SDWARFTYPE:
		return "SDWARFTYPE"
	case SDWARFVAR:
		return "SDWARFVAR"
	case SDWARFRANGE:
		return "SDWARFRANGE"
	case SDWARFLOC:
		return "SDWARFLOC"
	case SDWARFLINES:
		return "SDWARFLINES"
	case SDWARFADDR:
		return "SDWARFADDR"
	case SSEHUNWINDINFO:
		return "SSEHUNWINDINFO"
	case SSEHSECT:
		return "SSEHSECT"
	default:
		return "SymKind(" + strconv.Itoa(int(s)) + ")"
	}
}
