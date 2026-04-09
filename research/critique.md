# Critique: literate prompting

## Is the problem real?

Yes, partially. Prompt management is genuinely painful at scale. When you have dozens of agents sharing fragments of instructions, copy-paste drift becomes a real source of bugs. And long prompts do accumulate authoring notes, TODOs, and contextual explanations that waste tokens if left in the final output. Anyone running an agentic system with more than a handful of prompts has felt this.

But it is worth asking how many people are actually at this scale. The vast majority of prompt authors are still working with single prompts in a playground or a short string in application code. The audience for a build system is the subset who (a) manage prompts as files, (b) have enough of them that composition matters, and (c) care about token cost enough to strip comments. That is a real audience -- it includes teams building serious agentic products -- but it is not yet a large one. The risk is building tooling for a problem that is currently felt by hundreds of teams, not thousands.

The comments feature solves a real annoyance but a minor one. Most prompt authors already use ad-hoc conventions (lines starting with `//`, HTML comments, or just separate documentation files). The import/composition feature is where the real value lies.

## Is this the right solution?

The strongest argument against literate prompting is: "Why not just use Jinja2?" Template engines already solve the composition problem. They are well-understood, widely supported, and battle-tested. A team could adopt `jinja2-cli` today, define their prompt fragments as partials, and get 80% of this value with zero new tooling.

The counter-argument is that template engines are not markdown-compatible. A `.jinja2` file will not render nicely on GitHub, will not work with markdown linters, and will confuse editors. Literate prompting's bet is that being a strict superset of markdown is a meaningful advantage. That is a reasonable bet -- prompts live in markdown today, and preserving that ecosystem compatibility matters -- but it needs to be stated clearly as the core differentiator, because without it the "just use a template engine" argument is fatal.

There is also a middle ground nobody talks about: a simple shell script or Makefile that concatenates markdown files and strips HTML comments. This would take 20 minutes to build and would cover a surprising number of use cases. The question literate prompting must answer is: what does a dedicated tool provide beyond that?

## Complexity vs. value

Introducing a build step into prompt workflows has real costs. Today, editing a prompt is instant: change the file, the agent reads it. A build step means there is now a source file and an output file. Which one does the agent read? Which one do you commit? Do you commit both? What happens when someone edits the output file directly?

These are the same problems that plague every compiled-from-source workflow (CSS preprocessors, TypeScript, protobuf). They are solvable, but they are not free. The tool needs to either integrate tightly into existing workflows (e.g., a file watcher, a pre-commit hook, a CI step) or be so fast and invisible that people forget it is there. If the build step ever becomes the reason a prompt change takes longer to deploy, people will abandon it.

The linting feature is interesting and potentially high-value independent of the build system. A standalone markdown-aware prompt linter (checking for common anti-patterns, validating frontmatter, ensuring imports resolve) could be useful even without the compilation step.

## Adoption barriers

The biggest barrier is inertia. String concatenation works. Template engines work. Every team that manages prompts already has some approach, and switching costs are real. Literate prompting needs to either catch people before they have built their own solution (hard to time) or be so clearly superior that migration is worth it.

A second barrier is trust. Prompts are sensitive. They often contain proprietary instructions, jailbreak mitigations, or business logic. Any tool in the prompt pipeline needs to be auditable and deterministic. If a build produces unexpected output even once, trust evaporates.

Third, the tooling story needs to be complete on day one. A CLI alone is not enough. People need editor support (syntax highlighting, import resolution, go-to-definition), CI integration, and clear documentation. The language server ambition is correct but it is a lot of work.

## Scope creep risks

This is the most dangerous dimension. The moment you have imports, people will ask for conditionals, variables, loops, and inheritance. Each is reasonable in isolation. Together, they turn literate prompting into a full template engine -- at which point you are competing with Jinja2 but with a smaller ecosystem, fewer features, and less documentation. This is the classic trap of domain-specific languages: they start minimal and end up as poorly-specified general-purpose languages.

The discipline required is to say "no" to all of these, possibly forever. Comments and imports. That is it. The moment you add conditionals, you have lost the simplicity argument that justifies the tool's existence.

## The cross-repo import problem

Git-based cross-repository imports sound elegant but are operationally complex. Security (importing content authored by someone else is a supply-chain attack vector), versioning (hashes are opaque, branches are non-reproducible, tags require upstream conventions), availability (builds depend on network access), and caching/staleness all need answers. This feature has the complexity profile of a package manager. Go modules, Terraform modules, and Deno's URL imports all faced these exact problems and each spent years getting the ergonomics right. Defer this entirely and focus on single-repo composition first.

## Alternative framings

It is worth questioning whether markdown is the right substrate. Prompts are increasingly structured: they have sections, metadata, and typed slots for runtime data. This looks less like a document and more like a configuration object.

An alternative: prompts are programs, not documents. They have control flow (few-shot examples are loops), abstraction (reusable instruction blocks are functions), and a runtime (the LLM). Maybe the right tool is not a markdown preprocessor but a lightweight prompt programming language with markdown as its output format.

Another alternative: prompts are packages. Instead of a build system, think of a prompt registry -- versioned, publishable, composable prompt packages with declared dependencies. This shifts the framing from "files that compile" to "modules that compose."

## What could go wrong

- **Whitespace sensitivity**: LLMs are sensitive to whitespace in subtle, model-specific ways. A build step that normalises whitespace could silently alter prompt behaviour, with failure modes that manifest as quality degradation rather than errors.
- **Import ordering**: The order of composed fragments matters for LLM interpretation. The tool needs clear, predictable ordering guarantees.
- **Frontmatter conflicts**: If two imported files both have YAML frontmatter, what happens? Merge? Error? Ignore?
- **Encoding issues**: Prompts may contain Unicode, emoji, or special tokens. The pipeline must be encoding-transparent.

The deepest risk is timing. If prompt management moves into platforms (OpenAI's prompt management, Anthropic's equivalent, or third-party tools), then file-based prompt management becomes niche. The bet here is that prompts-as-code will win over prompts-as-platform-config -- a reasonable but not certain bet.
