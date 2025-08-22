package eval

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cmd/go/internal/base"
)

var CmdEval = &base.Command{
	UsageLine: "go eval [expression]",
	Short:     "evaluate Go expressions and print the result",
	Long: `
Eval evaluates Go expressions and prints the result.

The eval command creates a temporary .goo file with the expression
wrapped in a put() statement and runs it.

Examples:
    go eval "2**2"                    # Prints: 4
    go eval "3 * 4 + 5"              # Prints: 17  
    go eval "\"Hello \" + \"World\"" # Prints: Hello World
    go eval "len([1,2,3,4,5])"       # Prints: 5

The expression is automatically wrapped in put() for output.
`,
}

func init() {
	CmdEval.Run = runEval
}

func runEval(ctx context.Context, cmd *base.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "go eval: missing expression\n")
		fmt.Fprintf(os.Stderr, "usage: go eval [expression]\n")
		os.Exit(1)
	}
	
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "go eval: too many arguments\n")
		fmt.Fprintf(os.Stderr, "usage: go eval [expression]\n") 
		os.Exit(1)
	}
	
	code := args[0]
	
	// Create temporary .goo file content
	// Check if it's a statement (starts with keywords like if, for, func, etc.) or an expression
	gooCode := generateGooCode(code)
	
	// Create temporary directory
	tempDir, err := ioutil.TempDir("", "go-eval-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "go eval: failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)
	
	// Write temporary .goo file
	tempFile := filepath.Join(tempDir, "eval.goo")
	err = ioutil.WriteFile(tempFile, []byte(gooCode), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go eval: failed to write temp file: %v\n", err)
		os.Exit(1)
	}
	
	// Run the temporary file
	err = runTempFile(tempFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go eval: %v\n", err)
		os.Exit(1)
	}
}

// runTempFile executes the temporary .goo file
func runTempFile(filename string) error {
	// Get the path to the current go binary
	goPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find go binary: %v", err)
	}
	
	// Execute: go run tempfile.goo
	cmd := exec.Command(goPath, "run", filename)
	
	// Set up environment to ensure transformers are enabled and GOROOT is set
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOO_USE_TRANSFORMERS=1")
	
	// Ensure GOROOT is set correctly
	// For our custom go installation, GOROOT should be the parent of the bin directory
	binDir := filepath.Dir(goPath)
	goRoot := filepath.Dir(binDir)
	cmd.Env = append(cmd.Env, "GOROOT="+goRoot)
	fmt.Fprintf(os.Stderr, "DEBUG: Setting GOROOT=%s for eval\n", goRoot)
	
	// Connect stdout/stderr to current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// generateGooCode determines if the input is a statement or expression and generates appropriate .goo code
func generateGooCode(code string) string {
	// Trim leading whitespace to check the first meaningful characters
	trimmed := strings.TrimLeft(code, " \t\n")
	
	// Check if it starts with statement keywords
	if isStatement(trimmed) {
		// For statements, use the code directly
		return trimmed
	}
	
	// For expressions, wrap in put() 
	return fmt.Sprintf("put(%s)", code)
}

// isStatement checks if the code starts with statement keywords
func isStatement(code string) bool {
	// Common statement keywords that indicate this is not an expression
	stmtKeywords := []string{
		"if ", "if(", "if{",
		"for ", "for(", "for{", 
		"while ", "while(", "while{",
		"func ", "func(",
		"def ", "def(",
		"switch ", "switch(", "switch{",
		"select ", "select{",
		"go ", 
		"defer ",
		"return ", "return;", "return\n",
		"break", "continue",
		"var ", "const ",
		"type ",
		"import ",
		"package ",
	}
	
	for _, keyword := range stmtKeywords {
		if strings.HasPrefix(code, keyword) {
			return true
		}
	}
	
	// Check for assignment statements (contains = but not == or != or <= or >= etc.)
	if strings.Contains(code, "=") && !strings.Contains(code, "==") && 
	   !strings.Contains(code, "!=") && !strings.Contains(code, "<=") && 
	   !strings.Contains(code, ">=") {
		return true
	}
	
	// Check for function calls that are statements (end with ; or no return value expected)
	// This is trickier to detect, so we'll be conservative for now
	
	return false
}

