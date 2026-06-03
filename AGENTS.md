# litprompt

A build system for prompts and skills. It strips author-only comments and resolves imports (local and remote) to produce a single flattened markdown file.

## Build and test

Requires Go 1.24 (managed via mise).

```sh
mise run build                        # build binary to bin/litprompt
mise run test                         # all tests (go test ./...)
mise run test-v                       # integration tests with ginkgo verbose output
go test ./integration/... -count=1    # integration tests only
mise run lint                         # go vet
```

## Architecture

```
main.go                       CLI entrypoint (cobra). Defines build, check, and lock commands.
                              Also: insertHeader (--header flag), resolveInputFiles (recursive walk + --match),
                              runBuildFromConfig (config-driven build path; no-args discovers
                              litprompt.yaml in cwd, or --config selects a named file).
internal/build/build.go       Core build orchestrator. Reads a file, strips comments,
                              resolves imports recursively, detects circular imports.
                              Remote imports read from cache by content hash. After
                              flattening, calls SubstituteVars on the output.
internal/build/substitute.go  Variable substitution. Walks the flattened output line-by-line,
                              skipping fenced code blocks and inline code spans. Returns
                              the substituted content plus a sorted list of unresolved names.
internal/config/              Parses litprompt.yaml (Load discovers in a dir; LoadFile reads an
                              explicit path) and resolves each entry into concrete
                              (source, output) pairs. Handles file/dir/glob source shapes.
internal/parse/parse.go       Comment stripping and import finding (variable directives,
                              [{{...}}](#NAME), are handled by the substitute pass). No I/O.
internal/gitfetch/            Parses GitHub/GitLab/Bitbucket URLs, fetches files via git CLI
                              with HTTPS→SSH fallback. Used by the lock command.
internal/lockfile/            Parses and writes litprompt.lock, verifies SHA-256 content hashes.
internal/interlock/           Derives skill-tool interlock tokens (slug:id:version) and the
                              aggregate manifest from source frontmatter + built body. Pure, no I/O.
internal/varsfile/            Hand-rolled .env parser. Load merges multiple files (later wins).
integration/                  Ginkgo test suite. Auto-discovers fixture dirs from tests/.
tests/*/                      Fixture-based test cases (success or error per directory).
```

**Data flow:** CLI calls `build.Build(path, opts)` which reads the file, calls `parse.StripComments`, then walks each `@[label](target)` import line, recursively calling `buildFile` for local imports or `resolveRemoteImport` for URLs. Once the output is flattened, `Build` calls `SubstituteVars` to replace `[{{placeholder}}](#NAME)` directives using `opts.Vars` (loaded from `--vars` files via `varsfile.Load`); any unresolved name aborts the build. Remote imports read cached content from `~/.cache/litprompt/` by hash, verified against `litprompt.lock`. The `lock` command uses `gitfetch.FetchFile` to populate the cache.

## Directive syntax

Three directives:

- **Comments:** `<!-- @ ... -->` -- stripped entirely from output. Regular HTML comments pass through.
- **Imports:** `@[label](./path.md)` -- replaced with the imported file's content. Supports local relative paths and HTTPS URLs. Must be at the start of a line. YAML frontmatter is stripped from imported files (root frontmatter is preserved).
- **Variables:** `[{{placeholder}}](#NAME)` -- substituted at build time from `--vars file.env`. `NAME` must match `[A-Z_][A-Z0-9_]*`; the `{{placeholder}}` text is the default rendered in the raw source (it reads as a Markdown link to `#NAME`, so an agent running the un-built prompt sees a sensible value). Variables can appear anywhere on a line; directives inside fenced code blocks or inline code spans are preserved. Missing variables are a hard build error.

## Test strategy

Tests are fixture-based in `tests/`. Each test case is a directory containing:

```
tests/my-test-case/
  src/prompt.md          # input file (and any files it imports)
  src/litprompt.lock        # lockfile, if the test uses remote imports
  expected/prompt.md     # expected build output (success case)
  expected/error         # expected error substring (error case)
  mock/                  # mock remote content tree (optional)
```

The integration suite (`integration/build_test.go`) auto-discovers all directories under `tests/` that contain a `src/` subdirectory. It determines success vs error cases by checking for `expected/error`.

**Adding a test case:**

1. Create `tests/my-case/src/prompt.md` with the input.
2. Add `tests/my-case/expected/prompt.md` with expected output, or `expected/error` with an error substring.
3. For remote imports, add `src/litprompt.lock` and a `mock/` tree mirroring the URL path structure (e.g., `mock/github.com/owner/repo/ref/file.md`).
4. Run `mise run test` -- the new case is picked up automatically.

## Code conventions

- **Error wrapping:** Use `fmt.Errorf("context: %w", err)` consistently. Errors bubble up with context at each layer.
- **No interfaces for internal types:** Concrete structs throughout. The codebase is small enough that this is fine.
- **Regex for parsing:** Comment and import patterns are compiled as package-level `regexp.MustCompile` vars.
- **Logging:** `log/slog` with leveled output (debug/verbose/quiet flags). Logs go to stderr, build output to stdout.
- **Testing remote imports:** The `--mock-dir` flag / `Options.MockDir` field replaces network fetches with local file reads during tests. URL paths are mapped to filesystem paths by stripping the scheme and `/blob/` segments.
- **Lockfile parser:** Hand-rolled minimal YAML-like parser (regex line-by-line), not a full YAML library.

## Key design decisions

- **`@` as the directive character:** Chosen to be visually distinct in markdown while not conflicting with standard markdown syntax. Both comments (`<!-- @ -->`) and imports (`@[label](path)`) use it.
- **Lockfile discovery:** `litprompt.lock` is discovered from the current working directory (like `package.json` or `terraform.lock`). `Options.LockfilePath` can override this. Remote imports require a lockfile entry with SHA-256 content hashes. The build never fetches from the network -- `litprompt lock` is the only command that hits the network.
- **Directory mode:** `litprompt build <dir/>` recursively walks the directory. `--match` filters files by glob pattern (uses `doublestar` library for `**` support). Output mirrors the input directory structure.
- **Config file (`litprompt.yaml`):** `litprompt build` with no args reads the config from cwd. Each entry has a `source` and `output`; per-build `header` is supported. Source shapes (file / directory / glob) each have exactly one allowed output shape (path / directory / sibling bare-filename) — see README for the matrix. Glob source ⇒ sibling output is the rule that makes output paths predictable without needing a base-path field. Failed builds are reported but don't stop siblings; the overall command exits non-zero if any failed.
- **`--config <path>`:** Selects an explicit config file instead of discovering `litprompt.yaml` in cwd — the primitive for multi-environment pipelines (e.g. `litprompt.prod.yaml` vs `litprompt.staging.yaml`). `runBuildFromConfig` `chdir`s into the config file's directory so sources, outputs, and the lockfile all resolve relative to it (i.e. `--config sub/x.yaml` ≡ `cd sub && litprompt build` for that file). It's config-mode only, so combining it with a source argument is an error, and a missing named file is a hard error (unlike discovery, which silently finds nothing).
- **Minimal templating:** A single placeholder mechanism (`[{{value}}](#NAME)` substituted from `--vars` files in `.env` format) is the only build-time templating. Loops, conditionals, value interpolation, and richer control flow remain out of scope.
- **Variable / import distinction is structural:** A variable directive `[{{value}}](#NAME)` is unambiguously distinct from an import `@[label](target)` — it has no leading `@` and wraps its placeholder text in `{{ }}` — so no heuristic is needed to tell them apart. A variable also reads as an ordinary Markdown link (`[{{value}}](#NAME)`) in the raw source, so the un-built prompt still renders. `NAME` is required to be UPPER_SNAKE_CASE to match the `.env` convention; a `{{ }}`-wrapped directive whose name is not UPPER_SNAKE is reported as a likely typo.
- **Substitute last:** Variable substitution runs on the fully flattened output, after all imports resolve. This makes substituted values literal — they are not re-parsed for directive shape — which prevents a value containing `@[x](./y.md)` from being interpreted as an import. The trade-off is that error messages don't carry source-file line numbers.
- **Frontmatter handling:** YAML frontmatter is preserved in the root file but stripped from all imported files, so the final output has at most one frontmatter block.
- **Circular import detection:** Uses an ordered set (`importChain`) tracking the current call stack. Errors include the full cycle path for debugging.
