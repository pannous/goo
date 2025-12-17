// Copyright 2025 The Goo Authors. All rights reserved.

package transforms

import (
	"fmt"
	"os"
)

// trace outputs debug messages when GOO_TRACE environment variable is set
// This allows easy activation/deactivation of debug output during development
func trace(format string, args ...any) {
	if os.Getenv("GOO_TRACE") != "" {
		fmt.Printf("[TRACE] "+format+"\n", args...)
	}
}
