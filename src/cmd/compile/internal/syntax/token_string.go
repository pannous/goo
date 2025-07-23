// Manual string representation for token constants
// This replaces stringer-generated code to avoid corruption issues

package syntax

import "strconv"

// String returns the string representation of the token
func (tok token) String() string {
	if int(tok) < len(TokenNames) && TokenNames[tok] != "" {
		return TokenNames[tok]
	}
	return "token(" + strconv.Itoa(int(tok)) + ")"
}