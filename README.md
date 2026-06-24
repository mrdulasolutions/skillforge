# Skill Forge

> Forge portable agentic **Skills** and plugins — free-form, or AI-compiled from your own data.

Skill Forge is a tiny, fast, offline-first CLI for authoring [Agent Skills](https://docs.anthropic.com) (the `SKILL.md` format). It scaffolds best-practice structure, validates with **strict parity** to the official tooling, optionally optimizes your skill's description with an AI model, and bundles a clean, distributable `.skill` archive — with an opt-in **compliance/audit** profile for regulated workflows.

```
  SKILL FORGE
  ✦ forge portable agentic skills & plugins
```

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/mrdulasolutions/skillforge/main/install.sh | sh
```

The installer (macOS and Linux) detects your OS/arch, downloads the matching static binary, **verifies its checksum (fail-closed)**, and puts `skillforge` on your `PATH`. No runtime dependencies. Override the install dir with `SKILLFORGE_BIN_DIR`, pin a version with `SKILLFORGE_VERSION`, or add a short alias with `SKILLFORGE_ALIAS=forge`.

Or build from source (Go 1.25+):

```sh
make build && make install
```

## Quickstart

```sh
skillforge setup                      # configure AI (stores key, verifies it works)
skillforge                            # launch the chat builder (describe a skill; AI drafts it)
skillforge compile ./docs ./notes.md  # synthesize a skill from your files, then refine
skillforge build pdf-extractor        # validate (+ --optimize --fix to AI-improve)
skillforge eval pdf-extractor --baseline   # benchmark with-vs-without across providers
skillforge package pdf-extractor      # bundle → pdf-extractor.skill
skillforge publish pdf-extractor      # .skill + manifest + share instructions
skillforge serve-mcp pdf-extractor    # expose skills as MCP tools over stdio
skillforge schema pdf-extractor       # emit MCP / OpenAI / Anthropic tool schemas
```

## Commands

Running `skillforge` with no arguments launches the chat builder (the headline experience).

| Command | What it does |
|---|---|
| `setup` | Configure an AI provider (OpenRouter or Ollama), store the key securely, and verify it with a live test call. |
| `chat` | Full-screen chat builder: describe a skill in plain words, AI drafts it, refine by chatting, say "go" to write the files. Type `/` for slash commands — `/build`, `/plugin`, `/compliance`, and `/skills`, `/export`, `/mcp` to list, package, or MCP-wire skills you've already built. Bare `skillforge` runs this. |
| `new [name]` | Scaffold a skill. With AI configured on a TTY it opens the chat builder; otherwise (or with `-y`) it scaffolds from a form/flags. Auto-derives the kebab-case name. `--type plugin`, `--compliance`. |
| `compile <path...>` | Read your files/folders and synthesize a skill with AI, then refine it in the chat UI (`-y` to write directly). |
| `build [path]` | Validate `SKILL.md` (frontmatter rules + warnings). `--optimize` refines the description via AI; `--fix` applies it; `--json` for CI. |
| `eval` / `test` `<path>` | Benchmark a skill's evals across your provider, AI-judging each expectation. `--baseline` measures lift; `--html` writes a report. |
| `package [path]` | Validate, then zip into a `.skill` (excludes `evals/` and build artifacts). |
| `publish [path]` | Package + sha256 + JSON manifest + how others install it. |
| `import <file\|url>` | Install a skill from a `.skill` file or URL (zip-slip-safe). |
| `serve-mcp [path...]` | Run an MCP server exposing skills as tools over stdio (`--execute` runs them via your provider). |
| `schema <path>` | Emit cross-provider tool schemas (MCP, OpenAI, Anthropic) for a skill. |
| `audit verify [path]` | Verify a compliance skill's HMAC-chained audit log; exits non-zero if the chain is broken. |
| `doctor` | Report version, AI provider availability, and config writability. |

## AI providers

Run **`skillforge setup`** to configure AI: pick OpenRouter (cloud) or Ollama (local), store the key in your OS keychain (0600-file fallback), and verify it with a live test call. This powers the conversational builder and `build --optimize`.

Resolution order (so you can override per-shell or in CI):

- **OpenRouter** — `OPENROUTER_API_KEY` / `OPENROUTER_MODEL` env, else the key/model saved by `setup`.
- **Ollama** — `OLLAMA_HOST` / `OLLAMA_MODEL` env, else saved config, else `http://localhost:11434`.

If neither is configured, AI steps degrade gracefully — every other command works fully offline.

## Compliance profile (opt-in)

`new --compliance` (or `--compliance` on a skill that already has it) turns on:

- **HMAC-chained, append-only audit log** at `<skill>/.skillforge/audit.jsonl` — tamper-evident; the signing key lives in your user config dir and never travels with a packaged skill.
- **Untrusted-input sanitization** — strips zero-width / bidi / homoglyph characters and flags prompt-injection patterns.
- **AI-disclosure & version-pinning template** in `references/disclosure.md` to append to generated artifacts.

`package --compliance` verifies the audit chain, records a package event, and writes a provenance manifest (`<bundle>.provenance.json`) pinning the tool version and sha256 of the bundle and every packaged file. `audit verify` re-checks the chain at any time.

## Cross-provider output

From one skill, Skill Forge emits the same tool three ways — `skillforge schema` writes the MCP, OpenAI function-calling, and Anthropic tool definitions, and `skillforge serve-mcp` serves your skills as live MCP tools over stdio. One definition, every provider.

## Format parity

Skill Forge mirrors the official `skill-creator` validation and packaging rules, so anything it generates passes the reference `quick_validate.py` and produces byte-identical `.skill` bundles — verified in CI against the vendored official scripts. The only intentional difference: a packaged bundle additionally drops Skill Forge's own generated, machine-specific MCP artifacts (`.mcp.json`, `schemas/`) so shared bundles stay portable.

## Roadmap

- `compile` connectors beyond local files (Box, Drive)
- a skill registry for `publish`/`import` discovery
- richer plugin/marketplace packaging

## License

MIT
