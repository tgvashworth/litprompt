package main

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"github.com/tgvashworth/litprompt/internal/build"
	"github.com/tgvashworth/litprompt/internal/gitfetch"
	"github.com/tgvashworth/litprompt/internal/lockfile"
	"github.com/tgvashworth/litprompt/internal/parse"
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
	header    string
)

func main() {
	root := &cobra.Command{
		Use:     "litprompt",
		Short:   "A markdown preprocessor for LLM prompts",
		Version: version,
		Long: `litprompt builds LLM prompts from markdown files with comments and imports.

Comments (<!-- @ ... -->) are stripped from the output.
Imports (@[label](./path.md)) inline content from other files.
Remote imports require a litprompt.lock with content hashes.`,
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
	root.AddCommand(lockCmd())

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
	cmd.Flags().StringVar(&header, "header", "", "add a generated-file comment: 'short' or 'full'")

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
			warnCount := 0
			for _, f := range files {
				slog.Info("checking", "file", f)
				_, err := build.Build(f, opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR %s: %s\n", f, err)
					errCount++
				} else {
					slog.Info("ok", "file", f)
				}

				// Warn about suspected imports (lines that look like imports but have trailing content).
				data, readErr := os.ReadFile(f)
				if readErr == nil {
					for _, s := range parse.FindSuspectedImports(string(data)) {
						fmt.Fprintf(os.Stderr, "WARN %s:%d: possible malformed import (trailing content): %s\n", f, s.Line+1, strings.TrimSpace(s.Content))
						warnCount++
					}
				}
			}

			if errCount > 0 {
				return fmt.Errorf("%d file(s) failed validation", errCount)
			}

			msg := fmt.Sprintf("ok: %d file(s) checked", len(files))
			if warnCount > 0 {
				msg += fmt.Sprintf(", %d warning(s)", warnCount)
			}
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			return nil
		},
	}

	cmd.Flags().StringVar(&matchGlob, "match", "", "glob pattern to filter files (e.g. '**/prompt.md')")

	return cmd
}

func lockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock <file.md|dir/>",
		Short: "Fetch remote imports and write litprompt.lock",
		Long: `Lock scans markdown files for remote imports, fetches each one
via git, computes content hashes, and writes litprompt.lock in the
current directory. Fetched content is cached in ~/.cache/litprompt/.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runLock,
	}
}

func runLock(cmd *cobra.Command, args []string) error {
	input := args[0]

	// Collect all files to scan.
	var filePaths []string
	if input == "-" {
		return fmt.Errorf("lock does not support stdin")
	}
	var err error
	filePaths, err = resolveInputFiles(input)
	if err != nil {
		return err
	}

	// Find all remote imports across all files.
	type remoteImport struct {
		url  string
		file string
	}
	var remotes []remoteImport
	seen := map[string]bool{}

	for _, f := range filePaths {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("reading %s: %w", f, err)
		}
		imports := parse.FindImports(string(data))
		for _, imp := range imports {
			if imp.IsRemote() && !seen[imp.Target] {
				seen[imp.Target] = true
				remotes = append(remotes, remoteImport{url: imp.Target, file: f})
			}
		}
	}

	if len(remotes) == 0 {
		fmt.Fprintf(os.Stderr, "no remote imports found\n")
		return nil
	}

	// Load existing lockfile (if any) to preserve entries.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	lockPath := filepath.Join(cwd, "litprompt.lock")
	lf, _ := lockfile.Load(lockPath)
	if lf == nil {
		lf = &lockfile.Lockfile{Imports: map[string]lockfile.Entry{}}
	}

	cacheDir := gitfetch.CacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	// Fetch each remote import.
	for _, r := range remotes {
		slog.Info("fetching", "url", r.url)

		cloneURL, ref, filePath, err := gitfetch.ParseGitURL(r.url)
		if err != nil {
			return fmt.Errorf("parsing URL %s: %w", r.url, err)
		}

		content, err := gitfetch.FetchFile(cloneURL, ref, filePath)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", r.url, err)
		}

		hash := lockfile.HashContent(content)
		lf.Imports[r.url] = lockfile.Entry{Hash: hash}

		// Cache by hash.
		hashHex := strings.TrimPrefix(hash, "sha256:")
		cachePath := filepath.Join(cacheDir, hashHex)
		if err := os.WriteFile(cachePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing cache: %w", err)
		}

		slog.Info("locked", "url", r.url, "hash", hash)
	}

	// Write lockfile.
	if err := lockfile.Save(lockPath, lf); err != nil {
		return fmt.Errorf("writing lockfile: %w", err)
	}

	fmt.Fprintf(os.Stderr, "locked %d remote import(s) → litprompt.lock\n", len(remotes))
	return nil
}

func buildOpts() build.Options {
	opts := build.Options{MockDir: mockDir}
	cwd, err := os.Getwd()
	if err == nil {
		opts.LockfilePath = filepath.Join(cwd, "litprompt.lock")
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
		if header != "" {
			result = insertHeader(result, header, "<stdin>")
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

		if header != "" {
			// Use path relative to cwd for the header.
			srcRel := f
			if cwd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(cwd, f); err == nil {
					srcRel = rel
				}
			}
			result = insertHeader(result, header, srcRel)
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

	// If the input was a single file, check if output looks like a directory.
	if !info.IsDir() {
		// Trailing slash or existing directory → write filename into that dir.
		if strings.HasSuffix(output, "/") || isDir(output) {
			return filepath.Join(output, filepath.Base(inputFile)), nil
		}
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

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

var frontmatterRe = regexp.MustCompile(`(?s)\A(---\n.*?\n---\n)`)

// insertHeader adds a generated-file HTML comment after any YAML frontmatter.
// mode is "short" or "full". srcPath is the source file path for the template.
func insertHeader(content string, mode string, srcPath string) string {
	var comment string
	switch mode {
	case "short":
		comment = fmt.Sprintf("<!-- litprompt %s -->", srcPath)
	case "full":
		comment = fmt.Sprintf("<!-- Generated by litprompt from %s. Do not edit. -->", srcPath)
	default:
		return content
	}

	if loc := frontmatterRe.FindStringIndex(content); loc != nil {
		// Insert after frontmatter.
		return content[:loc[1]] + "\n" + comment + "\n" + content[loc[1]:]
	}

	// No frontmatter — prepend.
	if content == "" {
		return comment + "\n"
	}
	return comment + "\n\n" + content
}
