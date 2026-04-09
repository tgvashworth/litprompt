package build

import (
	"fmt"
)

// Options configures the build process.
type Options struct {
	// MockDir, if set, is used to resolve remote imports from a local
	// directory instead of fetching from the network.
	MockDir string
}

// Build processes a literate prompting markdown file, stripping comments,
// resolving imports, and returning the final output.
func Build(inputPath string, opts Options) (string, error) {
	return "", fmt.Errorf("not implemented")
}
