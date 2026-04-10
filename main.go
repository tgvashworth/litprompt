package main

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"github.com/tgvashworth/litprompt/internal/build"
)

// version is set at build time via ldflags.
var version = "dev"

var (
	verbose   bool
	debug     bool
	quiet     bool
	mockDir   string
	outputTo  string
	matchGlob string
)

func main() {
	root := &cobra.Command{
		Use:     "litprompt",
		Short:   "A markdown preprocessor for LLM prompts",
		Version: version,
		Long: `litprompt builds LLM prompts from markdown files with comments and imports.

Comments (<!-- @ ... -->) are stripped from the output.
Imports (@[label](./path.md)) inline content from other files.
Remote imports require a prompt.lock with content hashes.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show files processed and imports resolved")
	root.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "show parsing details, hash comparisons")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress all output except errors")
	root.PersistentFlags().StringVar(&mockDir, "mock-dir", "", "use a directory for remote content (for testing)")

	root.AddCommand(buildCmd())
	root.AddCommand(checkCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging() {
	level := slog.LevelWarn
	if quiet {
		level = slog.LevelError
	} else if debug {
		level = slog.LevelDebug
	} else if verbose {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build <file.md|dir/>",
		Short: "Build one or more markdown files",
		Long: `Build processes markdown files, stripping comments
and resolving imports. Output goes to stdout by default.

Examples:
  litprompt build prompt.md            # build one file, print to stdout
  litprompt build prompt.md -o out.md  # build one file to a specific output
  litprompt build prompts/ -o out/     # build all .md files in directory`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runBuild,
	}

	cmd.Flags().StringVarP(&outputTo, "output", "o", "", "output file or directory")
	cmd.Flags().StringVar(&matchGlob, "match", "", "glob pattern to filter files (e.g. '**/prompt.md')")

	return cmd
}

func checkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <file.md|dir/>",
		Short: "Validate imports resolve, lockfile is current, no cycles",
		Long: `Check validates markdown files without producing output.
It verifies that all imports resolve, the lockfile is current for remote
imports, and there are no circular dependencies.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveInputFiles(args[0])
			if err != nil {
				return err
			}

			opts := buildOpts()
			errCount := 0
			for _, f := range files {
				slog.Info("checking", "file", f)
				_, err := build.Build(f, opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR %s: %s\n", f, err)
					errCount++
				} else {
					slog.Info("ok", "file", f)
				}
			}

			if errCount > 0 {
				return fmt.Errorf("%d file(s) failed validation", errCount)
			}

			fmt.Fprintf(os.Stderr, "ok: %d file(s) checked\n", len(files))
			return nil
		},
	}

	cmd.Flags().StringVar(&matchGlob, "match", "", "glob pattern to filter files (e.g. '**/prompt.md')")

	return cmd
}

func buildOpts() build.Options {
	opts := build.Options{MockDir: mockDir}
	cwd, err := os.Getwd()
	if err == nil {
		opts.LockfilePath = filepath.Join(cwd, "prompt.lock")
	}
	return opts
}

func runBuild(cmd *cobra.Command, args []string) error {
	input := args[0]
	opts := buildOpts()

	// Handle stdin
	if input == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		result, err := build.BuildString(string(data), cwd, opts)
		if err != nil {
			return err
		}
		fmt.Print(result)
		return nil
	}

	files, err := resolveInputFiles(input)
	if err != nil {
		return err
	}

	for _, f := range files {
		slog.Info("building", "file", f)

		result, err := build.Build(f, opts)
		if err != nil {
			return fmt.Errorf("building %s: %w", f, err)
		}

		if outputTo == "" {
			// stdout
			fmt.Print(result)
		} else {
			outPath, err := resolveOutputPath(f, input, outputTo)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}

			if err := os.WriteFile(outPath, []byte(result), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}

			slog.Info("wrote", "file", outPath)
		}
	}

	return nil
}

// resolveInputFiles returns a list of .md files to process.
// If input is a file, returns that file. If a directory, walks recursively.
// matchPattern, if non-empty, filters files by matching against their
// path relative to the input directory (supports ** via doublestar).
func resolveInputFiles(input string) ([]string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", input, err)
	}

	if !info.IsDir() {
		return []string{input}, nil
	}

	var files []string
	err = filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory %s: %w", input, err)
	}

	// Apply glob filter if set.
	if matchGlob != "" && info.IsDir() {
		var filtered []string
		for _, f := range files {
			rel, err := filepath.Rel(input, f)
			if err != nil {
				return nil, fmt.Errorf("computing relative path: %w", err)
			}
			matched, err := doublestar.PathMatch(matchGlob, rel)
			if err != nil {
				return nil, fmt.Errorf("invalid match pattern %q: %w", matchGlob, err)
			}
			if matched {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		if matchGlob != "" {
			return nil, fmt.Errorf("no .md files matching %q found in %s", matchGlob, input)
		}
		return nil, fmt.Errorf("no .md files found in %s", input)
	}

	return files, nil
}

// resolveOutputPath figures out where to write the output for a given input file.
func resolveOutputPath(inputFile, inputArg, output string) (string, error) {
	info, err := os.Stat(inputArg)
	if err != nil {
		return "", err
	}

	// If the input was a single file, output is used as-is (file path).
	if !info.IsDir() {
		return output, nil
	}

	// If the input was a directory, output is a directory.
	// Map input file name into the output directory.
	rel, err := filepath.Rel(inputArg, inputFile)
	if err != nil {
		return "", err
	}

	return filepath.Join(output, rel), nil
}
