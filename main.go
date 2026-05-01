package main

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"github.com/tgvashworth/litprompt/internal/build"
	"github.com/tgvashworth/litprompt/internal/config"
	"github.com/tgvashworth/litprompt/internal/gitfetch"
	"github.com/tgvashworth/litprompt/internal/interlock"
	"github.com/tgvashworth/litprompt/internal/lockfile"
	"github.com/tgvashworth/litprompt/internal/parse"
	"github.com/tgvashworth/litprompt/internal/varsfile"
)

// version is set at build time via ldflags.
var version = "dev"

var (
	verbose    bool
	debug      bool
	quiet      bool
	mockDir    string
	outputTo   string
	matchGlob  string
	header     string
	configPath string

	interlockMode     string
	interlockParam    string
	interlockManifest string

	varsFiles []string
)

func main() {
	root := &cobra.Command{
		Use:     "litprompt",
		Short:   "A build system for prompts and skills",
		Version: version,
		Long: `litprompt builds LLM prompts from markdown files with comments, imports, and variables.

Comments (<!-- @ ... -->) are stripped from the output.
Imports (@[label](./path.md)) inline content from other files.
Variables ([{{default}}](#NAME)) are substituted from --vars files at build time.
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
		Use:   "build [file.md|dir/]",
		Short: "Build one or more markdown files",
		Long: `Build processes markdown files, stripping comments
and resolving imports. Output goes to stdout by default.

With no argument, reads litprompt.yaml (or .yml) from the current directory
and builds every entry in it. CLI flags (-o, --header, --match) are ignored
in that mode — per-build settings come from the config. --config <path> selects
an explicit config file (e.g. for separate prod/staging builds) instead of
discovering one in the current directory.

Examples:
  litprompt build                              # build everything in litprompt.yaml
  litprompt build --config litprompt.prod.yaml # build everything in a named config
  litprompt build prompt.md                    # build one file, print to stdout
  litprompt build prompt.md -o out.md          # build one file to a specific output
  litprompt build prompts/ -o out/             # build all .md files in directory`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runBuild,
	}

	cmd.Flags().StringVarP(&outputTo, "output", "o", "", "output file or directory")
	cmd.Flags().StringVar(&matchGlob, "match", "", "glob pattern to filter files (e.g. '**/prompt.md')")
	cmd.Flags().StringVar(&header, "header", "", "add a generated-file comment: 'short' or 'full'")
	cmd.Flags().StringVar(&interlockMode, "interlock", "", "stamp an interlock line: 'analytics' or 'enforce'")
	cmd.Flags().StringVar(&interlockParam, "interlock-param", "", "tool-parameter name in the interlock line (default \"interlock_tokens\")")
	cmd.Flags().StringVar(&interlockManifest, "interlock-manifest", "", "path to write the interlock manifest (default \"interlocks.json\")")
	cmd.Flags().StringVar(&configPath, "config", "", "path to a litprompt.yaml config (default: discover in cwd); cannot be combined with a source argument")
	cmd.Flags().StringSliceVar(&varsFiles, "vars", nil, ".env-format file of variable values (repeatable; later files override earlier on key collision)")

	return cmd
}

func checkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <file.md|dir/>",
		Short: "Validate imports resolve, lockfile is current, no cycles",
		Long: `Check validates markdown files without producing output.
It verifies that all imports resolve, the lockfile is current for remote
imports, all variable directives resolve against --vars (if supplied),
and there are no circular dependencies.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveInputFiles(args[0])
			if err != nil {
				return err
			}

			opts, err := buildOpts()
			if err != nil {
				return err
			}
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
	cmd.Flags().StringSliceVar(&varsFiles, "vars", nil, ".env-format file of variable values (repeatable; later files override earlier on key collision)")

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

func buildOpts() (build.Options, error) {
	opts := build.Options{MockDir: mockDir}
	cwd, err := os.Getwd()
	if err == nil {
		opts.LockfilePath = filepath.Join(cwd, "litprompt.lock")
	}
	if len(varsFiles) > 0 {
		vars, err := varsfile.Load(varsFiles)
		if err != nil {
			return opts, err
		}
		opts.Vars = vars
	}
	return opts, nil
}

func runBuild(cmd *cobra.Command, args []string) (err error) {
	opts, err := buildOpts()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return runBuildFromConfig(opts)
	}

	if configPath != "" {
		return fmt.Errorf("--config applies only to config-driven builds; remove the source argument or the --config flag")
	}

	input := args[0]

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

	il, err := interlockOptsFromFlags()
	if err != nil {
		return err
	}

	// Write the manifest for whatever built successfully, even if a later file
	// fails — mirroring the config path, which records successful builds before
	// returning. A manifest-write failure surfaces only if the build itself did
	// not already fail.
	defer func() {
		if len(il.manifest) > 0 {
			if werr := writeManifest(il.settings.Manifest, il.manifest); werr != nil && err == nil {
				err = werr
			}
		}
	}()

	for _, f := range files {
		var outPath string
		if outputTo != "" {
			outPath, err = resolveOutputPath(f, input, outputTo)
			if err != nil {
				return err
			}
		}
		if err := buildOne(f, outPath, header, il, opts); err != nil {
			return err
		}
	}

	return nil
}

// interlockOptsFromFlags builds the interlock options for the args/stdin path
// from the --interlock* flags, validating the mode and seeding a manifest
// accumulator when interlock is active.
func interlockOptsFromFlags() (interlockOpts, error) {
	mode, err := normalizeInterlockMode(interlockMode)
	if err != nil {
		return interlockOpts{}, err
	}
	il := interlockOpts{
		mode: mode,
		settings: config.InterlockConfig{
			Param:    orDefault(interlockParam, config.DefaultInterlockParam),
			Manifest: orDefault(interlockManifest, config.DefaultInterlockManifest),
		},
	}
	if mode != interlock.ModeOff {
		il.manifest = map[string]interlock.ManifestEntry{}
	}
	return il, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// normalizeInterlockMode validates a mode string and maps "" to "off".
func normalizeInterlockMode(mode string) (string, error) {
	switch mode {
	case "", interlock.ModeOff:
		return interlock.ModeOff, nil
	case interlock.ModeAnalytics, interlock.ModeEnforce:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid interlock %q: must be \"off\", \"analytics\", or \"enforce\"", mode)
	}
}

// runBuildFromConfig runs every build declared in a litprompt.yaml config,
// continuing on errors and returning a non-nil error if any failed. When
// configPath is set, that explicit file is used and sources, outputs, and the
// lockfile resolve relative to its directory; otherwise the config is
// discovered in the current working directory.
func runBuildFromConfig(opts build.Options) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	var cfg *config.Config
	baseDir := cwd

	if configPath != "" {
		// Resolve the config to an absolute path before any chdir, then run as
		// if invoked from its directory: sources, outputs, and the lockfile are
		// all cwd-relative downstream, so `--config sub/x.yaml` behaves exactly
		// like `cd sub && litprompt build` pointed at that file.
		abs, aerr := filepath.Abs(configPath)
		if aerr != nil {
			return fmt.Errorf("resolving config path: %w", aerr)
		}
		cfg, err = config.LoadFile(abs)
		if err != nil {
			// Report the path the user typed, not the resolved absolute path.
			return fmt.Errorf("loading config %s: %w", configPath, err)
		}
		// chdir into the config's directory so downstream cwd-relative resolution
		// matches it. `cwd` is left untouched as the original directory, so the
		// deferred restore returns the process to where it started; `baseDir`
		// (not cwd) drives source/output/lockfile resolution below.
		if dir := filepath.Dir(abs); dir != cwd {
			if cerr := os.Chdir(dir); cerr != nil {
				return fmt.Errorf("entering config directory %s: %w", dir, cerr)
			}
			defer func() { _ = os.Chdir(cwd) }()
			baseDir = dir
		}
		opts.LockfilePath = filepath.Join(baseDir, "litprompt.lock")
	} else {
		cfg, err = config.Load(cwd)
		if err != nil {
			return err
		}
		if cfg == nil {
			return fmt.Errorf("no source given and no litprompt.yaml in %s", cwd)
		}
	}

	items, err := cfg.Resolve(baseDir)
	if err != nil {
		return err
	}

	settings := cfg.InterlockSettings()
	manifest := map[string]interlock.ManifestEntry{}

	errCount := 0
	for _, r := range items {
		il := interlockOpts{mode: r.Interlock, settings: settings, manifest: manifest}
		if err := buildOne(r.Source, r.Output, r.Header, il, opts); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR %s: %s\n", r.Source, err)
			errCount++
		}
	}

	// Write the single aggregate manifest after the loop. Failed builds never
	// recorded an entry, so it reflects only what actually built.
	if len(manifest) > 0 {
		if err := writeManifest(settings.Manifest, manifest); err != nil {
			return err
		}
	}

	if errCount > 0 {
		return fmt.Errorf("%d of %d build(s) failed", errCount, len(items))
	}
	if !quiet {
		fmt.Fprintf(os.Stderr, "ok: built %d file(s)\n", len(items))
	}
	return nil
}

// interlockOpts carries the resolved interlock configuration into buildOne.
type interlockOpts struct {
	mode     string                             // "off" | "analytics" | "enforce"
	settings config.InterlockConfig             // param/manifest/message, defaults applied
	manifest map[string]interlock.ManifestEntry // accumulator keyed by output path; may be nil
}

// writeManifest renders and writes the aggregate interlock manifest.
func writeManifest(path string, entries map[string]interlock.ManifestEntry) error {
	data, err := interlock.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshaling interlock manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing interlock manifest %s: %w", path, err)
	}
	return nil
}

// buildOne builds a single source file, optionally writing to outPath (empty
// → stdout). When interlock is active it stamps an interlock line and records a
// manifest entry; the optional header comment is added after that. The final
// order after any frontmatter is: header → interlock → body.
func buildOne(srcPath, outPath, headerMode string, il interlockOpts, opts build.Options) error {
	slog.Info("building", "file", srcPath)

	result, err := build.Build(srcPath, opts)
	if err != nil {
		return fmt.Errorf("building %s: %w", srcPath, err)
	}

	interlockLine, entry, err := interlockFor(srcPath, result, il)
	if err != nil {
		return err
	}
	// Each insertion goes immediately after the frontmatter, so the line added
	// last ends up on top. Insert the interlock line first, then the header, to
	// land on the order frontmatter → header → interlock → body. (Swapping these
	// two calls would invert that order, not preserve it.)
	if interlockLine != "" {
		result = insertAfterFrontmatter(result, interlockLine)
	}
	if comment := headerComment(headerMode, relPath(srcPath)); comment != "" {
		result = insertAfterFrontmatter(result, comment)
	}

	if outPath == "" {
		fmt.Print(result)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(result), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	// Record the manifest entry only after the output is durably written, keyed
	// by output path, so a failed build never contributes an entry.
	if entry != nil && il.manifest != nil {
		il.manifest[outPath] = *entry
	}
	slog.Info("wrote", "file", outPath)
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

// frontmatterRe matches a leading YAML frontmatter block. The trailing newline
// after the closing --- is optional so a file whose frontmatter ends at EOF is
// still detected (matching internal/build and internal/interlock).
var frontmatterRe = regexp.MustCompile(`(?s)\A(---\n.*?\n---\n?)`)

// interlockFor derives the interlock line and manifest entry for a build when
// interlock is active. The version hash is computed from the built body with
// frontmatter stripped, before any line is inserted, so it never hashes itself
// and is stable regardless of header/interlock. A missing frontmatter identity
// is a hard error. The caller records the returned entry only after a
// successful write. Returns ("", nil, nil) when interlock is off.
func interlockFor(srcPath, result string, il interlockOpts) (string, *interlock.ManifestEntry, error) {
	if il.mode == "" || il.mode == interlock.ModeOff {
		return "", nil, nil
	}

	src, err := os.ReadFile(srcPath)
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", srcPath, err)
	}
	id, err := interlock.DeriveIdentity(string(src))
	if err != nil {
		return "", nil, fmt.Errorf("cannot derive interlock identity for %s: %w", srcPath, err)
	}

	version := interlock.Version(build.StripFrontmatter(result))
	entry := &interlock.ManifestEntry{
		Slug:           id.Slug,
		ID:             id.ID,
		Version:        version,
		Name:           id.Name,
		IdentitySource: id.IdentitySource,
	}
	return interlock.Line(id.Token(version), il.settings.Param, il.mode, il.settings.Message), entry, nil
}

// relPath returns srcPath relative to the working directory when possible.
func relPath(srcPath string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, srcPath); err == nil {
			return rel
		}
	}
	return srcPath
}

// headerComment returns the generated-file HTML comment for mode, or "" when
// no header is requested. mode is "short" or "full".
func headerComment(mode, srcPath string) string {
	switch mode {
	case "short":
		return fmt.Sprintf("<!-- litprompt %s -->", srcPath)
	case "full":
		return fmt.Sprintf("<!-- Generated by litprompt from %s. Do not edit. -->", srcPath)
	}
	return ""
}

// insertAfterFrontmatter inserts line after any YAML frontmatter block, or
// prepends it when the content has none.
func insertAfterFrontmatter(content, line string) string {
	if loc := frontmatterRe.FindStringIndex(content); loc != nil {
		return content[:loc[1]] + "\n" + line + "\n" + content[loc[1]:]
	}
	if content == "" {
		return line + "\n"
	}
	return line + "\n\n" + content
}

// insertHeader adds a generated-file HTML comment after any YAML frontmatter.
func insertHeader(content, mode, srcPath string) string {
	if comment := headerComment(mode, srcPath); comment != "" {
		return insertAfterFrontmatter(content, comment)
	}
	return content
}
