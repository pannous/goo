package main

import "fmt"

func hash(s string) uint {
	b := []byte(s)
	return (uint(b[0])<<4 ^ uint(b[1]) + uint(len(b))) & 63
}

func main() {
	keywords := []string{
		"break", "case", "chan", "check", "const", "continue",
		"default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var",
	}
	
	alternatives := []string{"fn"}
	
	fmt.Println("Testing alternatives:")
	for _, alt := range alternatives {
		h := hash(alt)
		collision := false
		for _, kw := range keywords {
			if hash(kw) == h {
				fmt.Printf("COLLISION: %s and %s both hash to %d\n", alt, kw, h)
				collision = true
			}
		}
		if !collision {
			fmt.Printf("%s -> %d (NO COLLISION)\n", alt, h)
		}
	}
	
	hashMap := make(map[uint]string)
	fmt.Println("Keyword hashes:")
	for _, kw := range keywords {
		h := hash(kw)
		if existing, exists := hashMap[h]; exists {
			fmt.Printf("COLLISION: %s and %s both hash to %d\n", kw, existing, h)
		} else {
			hashMap[h] = kw
		}
		fmt.Printf("%s -> %d\n", kw, h)
	}
}