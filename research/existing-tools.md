# Existing tools and prior art

Research into whether "literate prompting" — a markdown preprocessor for LLM prompts supporting comments, imports, and frontmatter — already exists as a concept or tool.

## Prompt management and templating frameworks

### Priompt (Anysphere)

Priompt is a JSX-based priority-driven prompt compiler developed by Anysphere (the company behind Cursor). It treats prompt construction as a layout problem: you define prompt sections as JSX components with priority weights, and Priompt compiles them down to fit within a token budget. It supports composition via JSX component nesting. It does not use markdown as its authoring format — prompts are written in TypeScript/JSX. No built-in comment stripping or markdown import semantics. It solves a different problem (token budget management) but shares the "prompt as compiled artifact" philosophy.

### Promptfoo

Promptfoo is primarily a prompt testing and evaluation framework. Prompts are stored as standalone text/markdown files or Nunjucks templates. It supports basic variable interpolation (`{{variable}}`) and can load prompts from files, but has no import/include directive for composing prompt fragments from multiple files. No comment stripping. Its focus is eval, not authoring.

### Guidance (Microsoft)

Guidance is a Python library for structured LLM output using a template language embedded in Python. It supports control flow (loops, conditionals) and constrained generation. Prompts are defined as Python strings with Handlebars-like syntax. It is not markdown-based. Composition is achieved through Python functions, not file-level imports. No comment stripping concept.

### LMQL

LMQL is a query language for LLMs that embeds prompt templates in a Python-like syntax with constraints. It supports composition through Python function calls and scripted prompting. It is not markdown-based and has no file-level import system. It is more of a programming language for LLM interaction than a document preprocessor.

### LangChain prompt templates

LangChain provides `PromptTemplate` and `ChatPromptTemplate` classes with variable substitution. Composition is handled via `PipelinePromptTemplate` which chains templates together programmatically. Templates are strings in Python code, not markdown files. No comment stripping or file-based imports.

### Humanloop / Braintrust / PromptLayer

These prompt management platforms store prompts as versioned strings with variable interpolation. They focus on versioning, A/B testing, and observability. None provide a file-based markdown preprocessor with imports or comment stripping. Composition is handled at the API/platform level, not at the document level.

## Markdown preprocessors with import/include semantics

### markdown-it-include / remark-include

Various markdown ecosystem plugins add `@include(file.md)` or `!!!include(file.md)!!!` syntax. These are designed for documentation generation (e.g., building docs sites), not prompt compilation. They handle file inclusion but not comment stripping or YAML frontmatter merging. They could theoretically be repurposed but are not prompt-aware.

### mdBook / mdx-bundler / Markdoc (Stripe)

These are markdown-based content systems. mdBook (Rust) supports `{{#include file.md}}` for documentation. Markdoc supports partials and custom tags. MDX allows importing React components. All are oriented toward rendering HTML, not producing clean text output for LLM consumption. None strip comments for token optimization.

### Pandoc

Pandoc is the Swiss Army knife of document conversion. It can process markdown with YAML frontmatter and supports Lua filters for arbitrary transformation, which could theoretically implement comment stripping and includes. However, there is no existing filter or workflow designed for prompt compilation. You would need to build the prompt-specific semantics yourself.

## Literate programming in the LLM/prompt context

The term "literate prompting" does not appear to be established in the literature or tooling ecosystem as of mid-2025. The concept of literate programming (Knuth, 1984) — where documentation and code are interwoven and a build step extracts the executable artifact — has not been formally applied to prompt engineering in any widely-known tool.

Some adjacent concepts exist:

- **Prompt chains as code**: Tools like Langflow, Flowise, and similar visual prompt builders treat prompts as nodes in a graph, but the authoring format is a visual UI, not markdown.
- **`.cursorrules` / `CLAUDE.md` / `.github/copilot-instructions.md`**: These are markdown-based instruction files consumed by AI coding assistants. They are essentially "literate prompts" but with no preprocessing — what you write is what gets sent. No comment stripping, no imports.
- **Fabric (Daniel Miessler)**: A prompt management CLI that organizes prompts as markdown files in a directory structure. Supports selecting and composing prompts at runtime, but has no preprocessor, no imports, and no comment stripping.

## The gap

No existing tool combines all of:

1. **Markdown as the authoring format** (preserving compatibility with editors, linters, renderers)
2. **A build/compile step** that produces a clean output prompt
3. **Comment stripping** (so authors can annotate prompts without wasting tokens)
4. **File imports/includes** (so prompt fragments can be shared across multiple prompts)
5. **YAML frontmatter support** (for metadata, configuration, or variables)

The closest precedents are markdown include plugins (which handle imports but not comments or prompt-specific concerns) and Priompt (which has the "compiled prompt" concept but uses JSX, not markdown). The specific combination of a markdown-native preprocessor designed for prompt authoring appears to be a genuine gap in the tooling landscape.

## Summary table

| Tool | Markdown-based | Imports/includes | Comment stripping | Build step | Prompt-focused |
|------|---------------|-----------------|-------------------|------------|----------------|
| Priompt | No (JSX) | Via components | No | Yes | Yes |
| Promptfoo | Partial | No | No | No | Yes (eval) |
| Guidance | No | Via Python | No | No | Yes |
| LMQL | No | Via Python | No | No | Yes |
| LangChain templates | No | Via Python | No | No | Yes |
| markdown-it-include | Yes | Yes | No | Yes | No |
| Markdoc | Yes | Yes (partials) | No | Yes | No |
| Pandoc + filters | Yes | Possible | Possible | Yes | No |
| Fabric | Yes | No | No | No | Yes |
| CLAUDE.md et al. | Yes | No | No | No | Yes |

**Key finding**: The specific concept — a markdown preprocessor that strips comments, resolves imports, and produces clean prompt output — does not appear to exist as a named tool or established practice. The space has prompt-focused tools that aren't markdown-native, and markdown tools that aren't prompt-aware. This is a genuine gap.
