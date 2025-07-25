package q1

func Deref(typ any) any {
	if typ, ok := typ.(*int); ok {
		return *typ
	}
	return typ
}
