// Copyright 2025 The Goo Authors. All rights reserved.

package syntax

import (
	"io"
	"os"
	"strings"
)

// ConditionalPreprocessor handles #if/#end conditional compilation directives
type ConditionalPreprocessor struct {
	conditions map[string]bool // Build conditions (e.g., "DEBUG" -> true)
	original   io.Reader       // Original source
}

// NewConditionalPreprocessor creates a preprocessor with build conditions
func NewConditionalPreprocessor(src io.Reader, buildTags []string) *ConditionalPreprocessor {
	conditions := make(map[string]bool)
	
	// Add build tags from command line/environment
	for _, tag := range buildTags {
		conditions[tag] = true
	}
	
	// Add common environment-based conditions
	if isDebugBuild() {
		conditions["DEBUG"] = true
	}
	
	// Debug: Print what conditions are active (commented out for production)
	// fmt.Printf("PREPROCESSOR DEBUG: Active conditions: %v\n", conditions)
	
	return &ConditionalPreprocessor{
		conditions: conditions,
		original:   src,
	}
}

// Read implements io.Reader, preprocessing conditional directives
func (cp *ConditionalPreprocessor) Read(p []byte) (n int, err error) {
	// For now, read all content and preprocess it
	// In a production implementation, this could be streaming
	content, err := io.ReadAll(cp.original)
	if err != nil {
		return 0, err
	}
	
	processed := cp.processDirectives(string(content))
	
	// Return the processed content
	if len(processed) <= len(p) {
		copy(p, []byte(processed))
		return len(processed), io.EOF
	} else {
		copy(p, []byte(processed)[:len(p)])
		return len(p), nil
	}
}

// processDirectives handles the actual conditional compilation
func (cp *ConditionalPreprocessor) processDirectives(source string) string {
	lines := strings.Split(source, "\n")
	var result []string
	var stack []bool // Stack of condition states
	
	includeCurrentBlock := true
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmed, "#if ") {
			// Parse condition
			condition := strings.TrimSpace(trimmed[4:])
			conditionMet := cp.evaluateCondition(condition)
			
			// Push current state and update
			stack = append(stack, includeCurrentBlock)
			includeCurrentBlock = includeCurrentBlock && conditionMet
			
			// Skip the directive line itself
			continue
		}
		
		if trimmed == "#end" {
			// Pop from stack
			if len(stack) > 0 {
				includeCurrentBlock = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			
			// Skip the directive line itself  
			continue
		}
		
		// Include line if we're in an active block
		if includeCurrentBlock {
			result = append(result, line)
		} else {
			// Replace excluded lines with blank lines to preserve line numbers
			result = append(result, "")
		}
	}
	
	return strings.Join(result, "\n")
}

// evaluateCondition checks if a build condition is met
func (cp *ConditionalPreprocessor) evaluateCondition(condition string) bool {
	// Handle simple conditions for now
	condition = strings.TrimSpace(condition)
	
	// Check if condition is set
	return cp.conditions[condition]
}

// isDebugBuild detects if this is a debug build
func isDebugBuild() bool {
	// Check specifically for our DEBUG environment variable
	debug := os.Getenv("DEBUG")
	return debug != "" && debug != "0" && debug != "false"
}