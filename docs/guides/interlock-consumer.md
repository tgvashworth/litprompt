# Consuming interlocks in a tool

litprompt stamps an interlock token into built skills and emits a manifest. It
does **not** implement the other half — the tool that reads the token and logs
or enforces it. That half is yours. This guide shows what to build, using an
MCP server as the example.

If you haven't set up the litprompt side yet, read the **Interlocks** section of
the [Interlocks section of the Introduction](../README.md#interlocks) first. This doc picks up from the artifacts that
`litprompt build` produces.

## What you're working with

A stamped skill carries a line like this after its frontmatter:

```
Interlock: `chart-builder:db13d3df:fed100da` — you MUST pass this as `interlock_tokens` when you call the tool, or the call will be rejected.
```

The token is `slug:id:version`:

| Part | Example | Role | Verify? |
|---|---|---|---|
| `slug` | `chart-builder` | Human-readable label for logs | No — advisory |
| `id` | `db13d3df` | Stable identity of the skill | **Yes — this is the key** |
| `version` | `fed100da` | Content hash of the body | Staleness only — never gate on it |

And a manifest, `interlocks.json`, keyed by output path:

```json
{
  "version": 1,
  "interlocks": {
    "plugins/data/skills/chart/SKILL.md": {
      "slug": "chart-builder",
      "id": "db13d3df",
      "version": "fed100da",
      "name": "Chart Builder",
      "identitySource": "name"
    },
    "plugins/data/skills/query/SKILL.md": {
      "slug": "query-runner",
      "id": "416c66c1",
      "version": "eb715297",
      "name": "Query Runner",
      "identitySource": "name"
    }
  }
}
```

The manifest is your source of truth for the set of valid `id`s and the current
`version` per id. Ship it alongside your tool and reload it whenever you
redeploy skills.

## The two rules

1. **Enforce on `id`, never `version`.** The `id` is stable across content
   edits, so enforcement keeps working as you edit a skill. The `version`
   rotates on every edit, so gating on it would break the moment you fix a typo.
2. **Treat a `version` mismatch as a warning, not a rejection.** That is the
   whole point of splitting id from version: you learn "the agent read a stale
   copy" without ever blocking on it.

Both `analytics` and `enforce` modes emit the **same token** — the litprompt
mode only changes the wording around it. Your tool decides whether to log or
block.

## Implementing it (MCP server, TypeScript)

### 1. Accept the parameter

The skill instructs the agent to pass the token via whatever you set as
`interlock.param` (default `interlock_tokens`). Your tool must declare it:

```ts
{
  name: "generate_chart",
  description: "Generate a chart. Read the Chart Builder skill first.",
  inputSchema: {
    type: "object",
    properties: {
      spec: { type: "object", description: "Chart spec" },
      interlock_tokens: {
        type: "string",
        description:
          "Comma-separated interlock tokens from the skills you read " +
          "(the `Interlock:` line in each SKILL.md).",
      },
    },
    required: ["spec"], // add "interlock_tokens" here for a schema-level hard gate
  },
}
```

### 2. Load the manifest once

```ts
import { readFileSync } from "node:fs";

type Entry = {
  slug: string;
  id: string;
  version: string;
  name: string;
  identitySource: string;
};
type Manifest = { version: number; interlocks: Record<string, Entry> };

const manifest: Manifest = JSON.parse(readFileSync("interlocks.json", "utf8"));

const entries = Object.values(manifest.interlocks);
const validIds = new Set(entries.map((e) => e.id));
const versionById = new Map(entries.map((e) => [e.id, e.version]));
```

### 3. Parse the tokens

A token is `slug:id:version`; split on `:`, tolerate a missing version.

```ts
function parseTokens(raw: string | undefined) {
  return (raw ?? "")
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
    .map((t) => {
      const [slug, id, version] = t.split(":");
      return { slug, id, version };
    });
}
```

### 4a. Analytics — observe, never block

```ts
function recordAnalytics(raw: string | undefined) {
  for (const { id, version } of parseTokens(raw)) {
    const known = validIds.has(id);
    const stale = known && version !== versionById.get(id);
    log.info("interlock", {
      id,
      known, // was this a real skill id?
      stale, // did they read an outdated copy?
      passedVersion: version,
      currentVersion: versionById.get(id),
    });
  }
  // ...then run the tool normally regardless.
}
```

### 4b. Enforce — reject calls without a valid id

The baseline contract is "at least one recognized id must be present":

```ts
function enforceAnyKnown(raw: string | undefined) {
  if (!parseTokens(raw).some((t) => validIds.has(t.id))) {
    throw new Error(
      "This tool requires reading a paired skill first. " +
        "Pass its Interlock token via `interlock_tokens`.",
    );
  }
}
```

That only proves the agent read *a* skill, not *this* one. For a real gate, tie
the specific tool to its specific skill's id:

```ts
const REQUIRED_ID = "db13d3df"; // chart-builder

function enforceSpecific(raw: string | undefined) {
  const match = parseTokens(raw).find((t) => t.id === REQUIRED_ID);
  if (!match) {
    throw new Error(
      "generate_chart requires the Chart Builder skill. " +
        "Read SKILL.md and pass its Interlock token via interlock_tokens.",
    );
  }
  // Non-blocking staleness warning.
  if (match.version !== versionById.get(REQUIRED_ID)) {
    log.warn("agent read a stale Chart Builder skill", {
      passed: match.version,
      current: versionById.get(REQUIRED_ID),
    });
  }
}
```

### 5. Wire it into the handler

```ts
server.setRequestHandler(CallToolRequestSchema, async (req) => {
  if (req.params.name === "generate_chart") {
    const tokens = req.params.arguments?.interlock_tokens as string | undefined;

    enforceSpecific(tokens); // hard gate
    // or: recordAnalytics(tokens); // observe only

    return runGenerateChart(req.params.arguments);
  }
});
```

## Rollout

Start every new interlock in `analytics` mode. Watch the logs to confirm agents
are passing tokens and that the ids/versions look right, then flip the build to
`enforce`. The token format is identical between modes, so nothing on the tool
side has to change — you just start throwing instead of logging.

## Checklist

- [ ] Tool declares the `interlock_tokens` parameter.
- [ ] Server loads `interlocks.json` at startup (and reloads on skill redeploy).
- [ ] Tokens parsed by splitting on `:`.
- [ ] Enforcement matches on `id`; `slug` ignored.
- [ ] `version` mismatch logged as staleness, never rejected.
