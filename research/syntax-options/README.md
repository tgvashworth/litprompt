# Syntax options for literate prompting

Research into extending markdown for literate prompt authoring, focusing on two features: **comments** (author annotations stripped during build) and **imports** (including content from other files).

## Comment syntax options

| Option | Syntax | Valid markdown? | Inline support? | Multi-line? | Editor support | Familiarity |
|--------|--------|----------------|----------------|-------------|----------------|-------------|
| C1: Double-percent | `%% comment %%` | No | Yes | Yes | Needs custom grammar | Low (LaTeX-adjacent) |
| C2: Line-prefix | `// comment` | No | No | Repeated prefix | Good (many editors have Ctrl+/) | High (C/JS/Go) |
| C3: Fenced block | `` ```comment `` | Yes | No | Yes | Excellent (code block handling) | Medium |
| C4: HTML comments | `<!-- comment -->` | Yes | Yes | Yes | Excellent | High (HTML) |
| C5: Jinja-style | `{# comment #}` | No | Yes | Yes | Good (Jinja/Nunjucks modes) | Medium (template authors) |

### Recommendation notes

- **C4 (HTML comments)** is the safest choice if you want zero tooling friction, but requires a build step to avoid leaking comments into output. Consider a variant like `<!--! comment -->` to distinguish strippable comments from intentional HTML comments.
- **C1 (double-percent)** is the most ergonomic custom syntax -- light, supports inline, and unlikely to conflict.
- **C3 (fenced block)** is interesting because it is already valid markdown, but the verbosity makes it impractical for short notes.
- **C5 (Jinja-style)** is appealing if you want to add template logic later (variables, conditionals).

## Import syntax options

| Option | Syntax | Valid markdown? | Inline support? | Configurable? | Ecosystem precedent |
|--------|--------|----------------|----------------|---------------|-------------------|
| I1: @import | `@import "./file.md"` | No | No | Limited | CSS/Less/Sass |
| I2: Mustache | `{{import "./file.md"}}` | No | Yes | Via extensions | Handlebars/Hugo |
| I3: Link syntax | `![include](./file.md)` | Yes | Yes | No | None (hack) |
| I4: Colon-directive | `::import{src="./file.md"}` | No* | No | Yes (attributes) | remark-directive, MyST, Pandoc |
| I5: Frontmatter | YAML `imports:` block | Yes | N/A | Yes (YAML) | Static site generators |

*Proposed CommonMark extension, not yet standardized.

### Recommendation notes

- **I1 (@import)** is the simplest and most readable. Good default unless you need configuration.
- **I4 (colon-directive)** is the most future-proof, aligning with CommonMark's generic directives proposal. Best choice if you want rich attributes (heading offset, section selection, conditional includes).
- **I3 (link syntax)** is clever but too hacky -- the broken-image preview is a poor authoring experience.
- **I5 (frontmatter)** is clean but the disconnect between declaration and insertion point is a significant usability problem.

## Combined recommendations

| Priority | Comments | Imports | Tradeoff |
|----------|----------|---------|----------|
| Maximum tool compatibility | C4 (HTML stripped) | I3 (link syntax) | Both valid markdown, but I3's UX is poor |
| Clean custom syntax | C1 (double-percent) | I1 (@import) | Minimal, readable, easy to implement |
| Future extensibility | C5 (Jinja-style) | I4 (colon-directive) | Richest feature path, aligns with ecosystems |

## Files in this directory

- `_shared/tone-and-style.md` -- shared file used by import demos
- `comment-c1-double-percent.md` -- double-percent comment syntax
- `comment-c2-line-prefix.md` -- line-prefix comment syntax
- `comment-c3-fenced-block.md` -- fenced code block comment syntax
- `comment-c4-html-stripped.md` -- HTML comments stripped during build
- `comment-c5-jinja-style.md` -- Jinja/Nunjucks-style comment syntax
- `import-i1-directive.md` -- @import directive syntax
- `import-i2-mustache.md` -- Mustache/Handlebars-style import syntax
- `import-i3-link-syntax.md` -- Markdown image/link syntax import
- `import-i4-colon-directive.md` -- Colon-directive/container syntax import
- `import-i5-frontmatter.md` -- YAML frontmatter-based imports
