# litprompt — a markdown preprocessor for LLM prompts

*2026-04-10T07:45:47Z by Showboat 0.6.1*
<!-- showboat-id: fd431dd7-674b-4912-9490-06dd78710943 -->

litprompt strips author-only comments and resolves imports in markdown files, producing a single flattened prompt. Let's see it in action.

## Setup

First, build the binary from source.

```bash
go build -o bin/litprompt . && bin/litprompt --version
```

```output
litprompt version dev
```

## Comments

`<!-- @ ... -->` comments are stripped from the output. Regular HTML comments pass through.

```bash
cat <<'PROMPT' | bin/litprompt build -
# Coding assistant

<!-- @
Based on Q1 user research. "Rules" framing tested 15% better than "guidelines".
Contact: tom@example.com
-->

You are a senior software engineer.

<!-- A regular HTML comment — this stays -->

## Rules

<!-- @
TODO: Add test coverage rule once we settle on a threshold.
-->

1. Explain your reasoning before writing code.
2. Fix bugs directly.
PROMPT
```

```output
# Coding assistant

You are a senior software engineer.

<!-- A regular HTML comment — this stays -->

## Rules

1. Explain your reasoning before writing code.
2. Fix bugs directly.
```

Both `<!-- @ -->` comments were stripped. The regular HTML comment was preserved. Author annotations are free — they cost zero tokens in the final prompt.

## Imports

`@[label](./path.md)` imports inline content from other files. Let's create a multi-file prompt.

```bash
mkdir -p /tmp/litprompt-demo/shared

cat > /tmp/litprompt-demo/shared/tone.md << 'EOF'
## Tone and style

- Be direct and concise. No filler phrases.
- Use a professional but approachable tone.
EOF

cat > /tmp/litprompt-demo/shared/safety.md << 'EOF'
---
title: Safety rules
author: security team
---

## Safety

- Do not execute arbitrary code without permission.
- If uncertain about safety, ask for clarification.
EOF

cat > /tmp/litprompt-demo/agent.md << 'EOF'
---
model: claude-4
---

# Coding assistant

<!-- @
This prompt powers the main coding agent.
Last updated after Q1 research review.
-->

You are a senior software engineer.

@[tone](./shared/tone.md)

## Rules

1. Explain your reasoning before writing code.
2. Fix bugs directly.

@[safety](./shared/safety.md)
EOF

echo "Files created:"
find /tmp/litprompt-demo -name "*.md" | sort
```

```output
Files created:
/tmp/litprompt-demo/agent.md
/tmp/litprompt-demo/shared/safety.md
/tmp/litprompt-demo/shared/tone.md
```

```bash
bin/litprompt build /tmp/litprompt-demo/agent.md
```

```output
---
model: claude-4
---

# Coding assistant

You are a senior software engineer.

## Tone and style

- Be direct and concise. No filler phrases.
- Use a professional but approachable tone.

## Rules

1. Explain your reasoning before writing code.
2. Fix bugs directly.

## Safety

- Do not execute arbitrary code without permission.
- If uncertain about safety, ask for clarification.
```

The comment was stripped, both imports were inlined, and the frontmatter from `safety.md` was removed (only the root file's frontmatter is preserved). The shared fragments can be reused across any number of prompts.

## Directory mode

Build an entire directory tree recursively. Use `--match` to filter which files are built.

```bash
mkdir -p /tmp/litprompt-demo/out
bin/litprompt build /tmp/litprompt-demo/ -o /tmp/litprompt-demo/out/ --match "agent.md"
echo "--- Built files ---"
find /tmp/litprompt-demo/out -name "*.md" | sort
echo ""
echo "--- Output ---"
cat /tmp/litprompt-demo/out/agent.md | head -5
```

```output
--- Built files ---
/tmp/litprompt-demo/out/agent.md

--- Output ---
---
model: claude-4
---

# Coding assistant
```

## Validation

`litprompt check` validates without producing output — useful in CI to ensure imports resolve and there are no circular dependencies.

```bash
bin/litprompt check /tmp/litprompt-demo/agent.md
```

```output
ok: 1 file(s) checked
```

## Error handling

Clear errors for missing imports, circular dependencies, and other issues.

```bash
echo "@[missing](./nope.md)" | bin/litprompt build - 2>&1 || true
```

```output
Error: import not found: ./nope.md
```

```bash
bin/litprompt build /tmp/litprompt-demo/circ-a.md 2>&1 || true
```

```output
Error: building /tmp/litprompt-demo/circ-a.md: circular import detected: circ-a.md -> circ-b.md -> circ-a.md
```

The full import chain is shown in circular dependency errors, making them easy to debug.

## Summary

litprompt does two things well:

- **Comments** (`<!-- @ ... -->`) — annotate prompts for humans without wasting LLM tokens
- **Imports** (`@[label](./path.md)`) — compose prompts from reusable fragments, locally or from remote git repos

The syntax is a backwards-compatible superset of markdown. Without litprompt, your files are still valid markdown with clickable links and visible (but harmless) comments.
