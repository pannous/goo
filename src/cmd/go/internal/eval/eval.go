package eval

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

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
	
	expression := args[0]
	
	// Create temporary .goo file content
	gooCode := fmt.Sprintf("put(%s)", expression)
	
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

