# Module systems research for literate prompting

This document surveys import/module systems across languages and tools, extracting lessons for designing an import system for a markdown-based literate prompting preprocessor.

## Survey of existing systems

### Go modules

Go modules use URL-like import paths (`github.com/org/repo/pkg`) resolved via a `go.mod` file that pins minimum versions. The module proxy (`proxy.golang.org`) caches and serves modules, while the checksum database (`sum.golang.org`) provides a tamper-proof transparency log of content hashes. Version resolution uses Minimum Version Selection (MVS): the build always uses the *minimum* version satisfying all constraints, making builds reproducible without a lock file (the `go.sum` file records expected hashes, not resolved versions).

**What went well:** Import paths are self-describing and globally unique. The proxy provides caching, availability (modules survive repo deletion), and privacy (the proxy fetches on your behalf). The checksum database prevents supply-chain attacks. MVS is simple and deterministic.

**What didn't:** The `v2+` major version path suffix (`/v2`, `/v3`) is awkward. The proxy is a centralized dependency. Early Go had no module system at all (`GOPATH`), and the migration was painful.

### Terraform modules

Terraform modules can be sourced from local paths (`./modules/vpc`), git repos (`git::https://github.com/org/repo.git//path?ref=v1.0`), S3, GCS, or the Terraform Registry. The registry provides a discovery protocol and semantic versioning. Git sources support `ref=` for branch, tag, or commit pinning. The double-slash (`//`) separates the repo URL from a subdirectory within it.

**What went well:** The git source syntax is extremely flexible -- any git-accessible content can be a module. The registry adds discoverability. Local paths are trivial for in-project reuse.

**What didn't:** No integrity checking for git sources (you trust the ref). The `//` subdirectory syntax is non-obvious. No lock file until Terraform 0.14 added `.terraform.lock.hcl` (and even then it only locks provider versions, not module sources). Transitive module versioning can be confusing.

### Deno

Deno originally used bare URL imports (`import { x } from "https://deno.land/std@0.200.0/fs/mod.ts"`), with versions embedded in the URL. Lock files (`deno.lock`) record content hashes for integrity. Import maps (`deno.json`) alias bare specifiers to URLs, enabling cleaner import statements. Deno caches downloaded modules in a global directory; once cached, no network access is needed.

**What went well:** URL imports are conceptually simple and need no registry. The version-in-URL pattern is explicit. Lock files with hashes provide reproducibility and integrity. Caching means offline-capable after first fetch.

**What didn't:** URL imports are verbose and hard to update across a codebase. The ecosystem eventually needed `deno.land/x` as a de facto registry. Deno has since moved toward `npm:` specifiers and `package.json` compatibility, partly acknowledging that pure URL imports had friction. Transitive dependency URLs can diverge across libraries.

### ES modules (JavaScript/Node)

ES Modules use bare specifiers (`import x from "lodash"`) resolved through `node_modules` and `package.json`, or URL specifiers in browsers. Import maps (a browser standard, also used by Deno) remap specifiers to URLs. `package.json` declares version ranges; `package-lock.json` or `yarn.lock` pins exact resolved versions. Registries (npm, JSR) are the primary distribution mechanism.

**What went well:** Import maps are a flexible, declarative remapping layer. The npm registry is a massive ecosystem with good tooling. Lock files are well-understood.

**What didn't:** `node_modules` is infamously heavy. Version range resolution is complex (semver ranges, peer dependencies). The CJS/ESM dual-module split caused years of pain. Supply-chain security is an ongoing concern despite `npm audit`.

### Python (pip)

Python uses `import` statements resolved from `sys.path`. Distribution uses pip with `requirements.txt` (pinned versions) or `pyproject.toml`. Lock files are not built-in; tools like `pip-compile`, `poetry.lock`, or `uv.lock` fill the gap. PyPI is the central registry. There is no native URL or git import in the language itself, though pip can install from git URLs (`pip install git+https://github.com/org/repo@tag`).

**What went well:** The registry ecosystem is vast. `requirements.txt` with pinned versions is simple to understand.

**What didn't:** No built-in lock file. Dependency resolution was historically slow and unreliable (improved by newer tools like `uv`). The distinction between import names and package names causes confusion. Virtual environments add complexity.

### Nix (flakes)

Nix flakes use a `flake.nix` file with inputs specifying git repos, GitHub repos, or other flakes by URL (`github:owner/repo/ref`). A `flake.lock` file pins inputs to exact revisions and content hashes (NAR hashes). `fetchFromGitHub` and similar builtins fetch content by owner/repo/rev with an expected hash. Everything is content-addressed and reproducible.

**What went well:** Maximal reproducibility. Content-addressable storage means identical inputs always produce identical outputs. The lock file pins *everything*, including transitive dependencies. No builds can be influenced by mutable state.

**What didn't:** Steep learning curve. The Nix language is unusual. Hash mismatches produce opaque errors. Flakes are still technically experimental. The upfront cost of computing and specifying hashes is real.

### Dhall

Dhall supports URL imports with mandatory integrity checks: `https://example.com/package.dhall sha256:abc123...`. The SHA-256 hash after the URL is computed over the *semantically normalized* expression, not the raw bytes. This means formatting changes don't break the hash. Imports are transitively resolved and cached by hash. There is no registry; URLs are the distribution mechanism.

**What went well:** Semantic hashing is brilliant -- it pins meaning, not formatting. Integrity is mandatory, not optional. Caching by content hash is maximally efficient. The system is simple and needs no lock file because every import carries its own integrity check.

**What didn't:** Computing the hash upfront requires tooling (`dhall freeze`). URL imports without a registry make discoverability hard. No versioning beyond what the URL encodes. If a URL goes down and you don't have the cache, you're stuck (no proxy/mirror system).

### Jsonnet

Jsonnet uses `import "path/to/file.jsonnet"` with resolution relative to a configurable set of import paths (`-J` flag). There is no native URL import, versioning, or integrity checking. The `jsonnet-bundler` (jb) tool adds dependency management with a `jsonnetfile.json` and `jsonnetfile.lock.json`, fetching from git repos.

**What went well:** Simple mental model -- it's just file paths. The bundler adds what's needed for cross-repo use.

**What didn't:** Without the bundler, there's no story for external dependencies. No integrity checking. The bundler is a community tool, not part of the language.

### MDX / remark (markdown ecosystem)

MDX allows JSX and JavaScript `import` statements in markdown files, resolved through the JavaScript module system (so `node_modules`, bundlers, etc.). Remark plugins can transform markdown but don't have a native import/include mechanism. There is no standard way in CommonMark or GFM to include content from another file. Various tools have invented their own: `!include` directives, custom code fence processors, or remark plugins like `remark-include`.

**What went well:** MDX's approach of embedding in an existing module system is pragmatic -- you get npm's entire ecosystem.

**What didn't:** It ties markdown to JavaScript. There is no cross-language standard for markdown includes. The fragmentation of custom solutions means nothing is portable.

### Typst

Typst (a modern document typesetting system) uses `#import "file.typ"` for local imports and has a package system using `#import "@preview/package:version"`. Packages are hosted on a central registry (Typst Universe). Versions follow semver. Packages are namespaced under `@preview` (or `@local` for local packages). There is no git-based import; the registry is the distribution mechanism.

**What went well:** Clean, simple syntax. The namespace system (`@preview`, `@local`) is elegant. Semver versioning is built in from day one. Good developer experience for a young ecosystem.

**What didn't:** Registry-only distribution is limiting for private or experimental packages. No git-based fallback. The system is young and the registry is small.

## Synthesis: an import system for literate prompting

### Recommended design

A literate prompting preprocessor should support three import forms, in order of complexity:

**1. Local imports (relative paths).** For within-project reuse. Simple, no versioning needed, resolved relative to the importing file.

```markdown
@import ./shared/tone.md
@import ../common/safety.md
```

**2. Git-based remote imports.** Using a URL + path + ref syntax inspired by Go modules and Terraform. The ref can be a semver tag, branch, or commit SHA.

```markdown
@import github.com/org/prompts/tone.md@v1.2.0
@import github.com/org/prompts/safety.md@abc123f
```

This is the right primary mechanism because: prompts are text files that live naturally in git repos; git refs give you immutable pinning (commit SHAs) and human-friendly versioning (tags); no registry infrastructure is needed to start; and it works for both public and private repos (via git authentication).

**3. A lock file for reproducibility.** A `prompt.lock` (or similar) file that records the resolved commit SHA and a content hash for every remote import, even when the import specifies a mutable ref like a branch or semver tag. This follows the Deno/Nix model: the import specifies *intent*, the lock file specifies *exact resolution*.

```yaml
# prompt.lock
imports:
  github.com/org/prompts/tone.md@v1.2.0:
    resolved: abc123f...
    hash: sha256:def456...
```

### Key design decisions

**Content hashing over semantic hashing.** Dhall's semantic hashing is elegant but requires a normalization step specific to the language. For markdown/prompt content, raw content hashing (SHA-256 of the file bytes) is sufficient and simpler. The lock file records these hashes.

**Transitive imports should be supported but flattened.** If an imported file itself contains imports, those should be resolved transitively. The lock file should record all transitive dependencies. Circular imports should be detected and rejected.

**No registry (yet).** A registry adds discoverability but also infrastructure burden. Git-based imports are sufficient for an early system. A registry could be layered on later as a resolution shorthand (e.g., `@import @org/tone@v1` resolving to a git URL via a registry lookup), similar to how Deno layered `deno.land/x` over URL imports.

**Security: treat imported prompts as untrusted by default.** External prompts can contain prompt injection. The system should: (a) clearly mark imported content boundaries in the resolved output, (b) support an allow-list of trusted sources, and (c) potentially support a review/audit step before accepting a new external import (similar to `go mod tidy` showing what changed). The lock file hash provides tamper detection for previously-reviewed imports.

**Caching.** Downloaded content should be cached locally by content hash (like Nix and Dhall). After the first fetch, resolution should work offline from cache. A global cache (`~/.cache/literate-prompting/`) avoids redundant downloads across projects.

**Developer experience priorities.** Clear error messages when an import cannot be resolved (unreachable repo, bad ref, hash mismatch). A `resolve` or `freeze` command that fetches all imports and updates the lock file. A `--verify` flag that checks all lock file hashes without re-fetching. Support for `--offline` mode that only uses the cache.

### What to learn from each system

| System | Key lesson for literate prompting |
|---|---|
| Go | URL-as-identity is powerful; a proxy/mirror adds resilience |
| Terraform | `git::url//subpath?ref=tag` proves the pattern works; lack of integrity checking is a gap |
| Deno | URL imports are simple to start but need aliasing (import maps) at scale |
| Dhall | Mandatory integrity checks are worth the friction; content-addressed caching is ideal |
| Nix | Pin everything in the lock file; content-addressability enables reproducibility |
| Jsonnet | A minimal system (just file paths) can be extended later with tooling |
| MDX | Tying to an existing module system is pragmatic but limits portability |
| Typst | Clean syntax and namespaces matter for developer experience |
| Python/JS | Registries enable ecosystems but add complexity; lock files are essential |
