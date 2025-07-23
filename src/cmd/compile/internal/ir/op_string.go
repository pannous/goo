// Manual string representation for Op constants
// This replaces stringer-generated code to avoid corruption issues

package ir

import "strconv"

// OpStringNames provides string representations for Op constants
// Using a map for simplicity and robustness against enum changes
var OpStringNames = map[Op]string{
	OXXX:                "XXX",
	ONAME:               "NAME",
	ONONAME:             "NONAME",
	OTYPE:               "TYPE",
	OLITERAL:            "LITERAL",
	ONIL:                "NIL",
	OADD:                "ADD",
	OSUB:                "SUB",
	OOR:                 "OR",
	OXOR:                "XOR",
	OADDSTR:             "ADDSTR",
	OADDR:               "ADDR",
	OANDAND:             "ANDAND",
	OAPPEND:             "APPEND",
	OBYTES2STR:          "BYTES2STR",
	OBYTES2STRTMP:       "BYTES2STRTMP",
	ORUNES2STR:          "RUNES2STR",
	OSTR2BYTES:          "STR2BYTES",
	OSTR2BYTESTMP:       "STR2BYTESTMP",
	OSTR2RUNES:          "STR2RUNES",
	OSLICE2ARR:          "SLICE2ARR",
	OSLICE2ARRPTR:       "SLICE2ARRPTR",
	OAS:                 "AS",
	OAS2:                "AS2",
	OAS2DOTTYPE:         "AS2DOTTYPE",
	OAS2FUNC:            "AS2FUNC",
	OAS2MAPR:            "AS2MAPR",
	OAS2RECV:            "AS2RECV",
	OASOP:               "ASOP",
	OCALL:               "CALL",
	OCALLFUNC:           "CALLFUNC",
	OCALLMETH:           "CALLMETH",
	OCALLINTER:          "CALLINTER",
	OCAP:                "CAP",
	OCLEAR:              "CLEAR",
	OCLOSE:              "CLOSE",
	OCLOSURE:            "CLOSURE",
	OCOMPLIT:            "COMPLIT",
	OMAPLIT:             "MAPLIT",
	OSTRUCTLIT:          "STRUCTLIT",
	OARRAYLIT:           "ARRAYLIT",
	OSLICELIT:           "SLICELIT",
	OPTRLIT:             "PTRLIT",
	OCONV:               "CONV",
	OCONVIFACE:          "CONVIFACE",
	OCONVNOP:            "CONVNOP",
	OCOPY:               "COPY",
	ODCL:                "DCL",
	ODCLFUNC:            "DCLFUNC",
	ODELETE:             "DELETE",
	ODOT:                "DOT",
	ODOTPTR:             "DOTPTR",
	ODOTMETH:            "DOTMETH",
	ODOTINTER:           "DOTINTER",
	OXDOT:               "XDOT",
	ODOTTYPE:            "DOTTYPE",
	ODOTTYPE2:           "DOTTYPE2",
	OEQ:                 "EQ",
	ONE:                 "NE",
	OLT:                 "LT",
	OLE:                 "LE",
	OGE:                 "GE",
	OGT:                 "GT",
	ODEREF:              "DEREF",
	OINDEX:              "INDEX",
	OINDEXMAP:           "INDEXMAP",
	OKEY:                "KEY",
	OSTRUCTKEY:          "STRUCTKEY",
	OLEN:                "LEN",
	OMAKE:               "MAKE",
	OMAKECHAN:           "MAKECHAN",
	OMAKEMAP:            "MAKEMAP",
	OMAKESLICE:          "MAKESLICE",
	OMAKESLICECOPY:      "MAKESLICECOPY",
	OMUL:                "MUL",
	ODIV:                "DIV",
	OMOD:                "MOD",
	OLSH:                "LSH",
	ORSH:                "RSH",
	OAND:                "AND",
	OANDNOT:             "ANDNOT",
	ONEW:                "NEW",
	ONOT:                "NOT",
	OBITNOT:             "BITNOT",
	OPLUS:               "PLUS",
	ONEG:                "NEG",
	OOROR:               "OROR",
	OPANIC:              "PANIC",
	OPRINT:              "PRINT",
	OPRINTLN:            "PRINTLN",
	OPRINTF:             "PRINTF",
	OPAREN:              "PAREN",
	OSEND:               "SEND",
	OSLICE:              "SLICE",
	OSLICEARR:           "SLICEARR",
	OSLICESTR:           "SLICESTR",
	OSLICE3:             "SLICE3",
	OSLICE3ARR:          "SLICE3ARR",
	OSLICEHEADER:        "SLICEHEADER",
	OSTRINGHEADER:       "STRINGHEADER",
	ORECOVER:            "RECOVER",
	ORECOVERFP:          "RECOVERFP",
	ORECV:               "RECV",
	ORUNESTR:            "RUNESTR",
	OSELRECV2:           "SELRECV2",
	OMIN:                "MIN",
	OMAX:                "MAX",
	OREAL:               "REAL",
	OIMAG:               "IMAG",
	OCOMPLEX:            "COMPLEX",
	OUNSAFEADD:          "UNSAFEADD",
	OUNSAFESLICE:        "UNSAFESLICE",
	OUNSAFESLICEDATA:    "UNSAFESLICEDATA",
	OUNSAFESTRING:       "UNSAFESTRING",
	OUNSAFESTRINGDATA:   "UNSAFESTRINGDATA",
	OMETHEXPR:           "METHEXPR",
	OMETHVALUE:          "METHVALUE",
	OBLOCK:              "BLOCK",
	OBREAK:              "BREAK",
	OCASE:               "CASE",
	OCONTINUE:           "CONTINUE",
	ODEFER:              "DEFER",
	OFALL:               "FALL",
	OFOR:                "FOR",
	OGOTO:               "GOTO",
	OIF:                 "IF",
	OLABEL:              "LABEL",
	OGO:                 "GO",
	ORANGE:              "RANGE",
	ORETURN:             "RETURN",
	OSELECT:             "SELECT",
	OSWITCH:             "SWITCH",
	OTYPESW:             "TYPESW",
	OINLCALL:            "INLCALL",
	OMAKEFACE:           "MAKEFACE",
	OITAB:               "ITAB",
	OIDATA:              "IDATA",
	OSPTR:               "SPTR",
	OCFUNC:              "CFUNC",
	OCHECKNIL:           "CHECKNIL",
	ORESULT:             "RESULT",
	OINLMARK:            "INLMARK",
	OLINKSYMOFFSET:      "LINKSYMOFFSET",
	OJUMPTABLE:          "JUMPTABLE",
	OINTERFACESWITCH:    "INTERFACESWITCH",
	ODYNAMICDOTTYPE:     "DYNAMICDOTTYPE",
	ODYNAMICDOTTYPE2:    "DYNAMICDOTTYPE2",
	ODYNAMICTYPE:        "DYNAMICTYPE",
	OTAILCALL:           "TAILCALL",
	OGETG:               "GETG",
	OGETCALLERSP:        "GETCALLERSP",
	OTYPEOF:             "TYPEOF",
	OEND:                "END",
}

// Enum stability validation - ensure critical ops have expected values
// This prevents the bootstrap corruption issue we encountered
func init() {
	criticalOps := map[Op]int{
		OBREAK:    118, // Critical for NewBranchStmt
		OCASE:     119, // The problematic one in our bug
		OCONTINUE: 120, // Critical for NewBranchStmt
		OFALL:     122, // Critical for NewBranchStmt
		OGOTO:     124, // Critical for NewBranchStmt
	}
	
	for op, expectedValue := range criticalOps {
		if int(op) != expectedValue {
			panic("CRITICAL: Op enum corruption detected! " + 
				  OpStringNames[op] + " changed from " + strconv.Itoa(expectedValue) + 
				  " to " + strconv.Itoa(int(op)) + ". This will cause bootstrap issues.")
		}
	}
}

// String returns the string representation of the Op
func (op Op) String() string {
	if name, ok := OpStringNames[op]; ok {
		return name
	}
	return "Op(" + strconv.Itoa(int(op)) + ")"
}