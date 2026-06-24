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

The installer detects your OS/arch, downloads the matching static binary, verifies its checksum, and puts `skillforge` on your `PATH`. No runtime dependencies. Override the install dir with `SKILLFORGE_BIN_DIR`, pin a version with `SKILLFORGE_VERSION`, or add a short alias with `SKILLFORGE_ALIAS=forge`.

Or build from source (Go 1.25+):

```sh
make build && make install
```

## Quickstart

```sh
skillforge setup                      # configure AI (stores key, verifies it works)
skillforge new                        # describe your skill in plain words; AI drafts it (chat UI)
skillforge compile ./docs ./notes.md  # synthesize a skill from your files, then refine
skillforge build pdf-extractor        # validate (+ --optimize --fix to AI-improve)
skillforge eval pdf-extractor --baseline   # benchmark with-vs-without across providers
skillforge package pdf-extractor      # bundle → pdf-extractor.skill
skillforge publish pdf-extractor      # .skill + manifest + share instructions
skillforge serve-mcp pdf-extractor    # expose skills as MCP tools over stdio
skillforge schema pdf-extractor       # emit MCP / OpenAI / Anthropic tool schemas
```

## Commands

| Command | What it does |
|---|---|
| `setup` | Configure an AI provider (OpenRouter or Ollama), store the key securely, and verify it with a live test call. |
| `new [name]` | Build a skill conversationally — a full-screen chat UI where you describe it and AI drafts the name, description, and SKILL.md, which you refine by chatting. Auto-derives the kebab-case name. Falls back to a form with no AI; `-y` non-interactive; `--type plugin`, `--compliance`. |
| `compile <path...>` | Read your files/folders and synthesize a skill with AI, then refine it in the chat UI (`-y` to write directly). |
| `build [path]` | Validate `SKILL.md` (frontmatter rules + warnings). `--optimize` refines the description via AI; `--fix` applies it; `--json` for CI. |
| `eval` / `test` `<path>` | Benchmark a skill's evals across your provider, AI-judging each expectation. `--baseline` measures lift; `--html` writes a report. |
| `package [path]` | Validate, then zip into a `.skill` (excludes `evals/` and build artifacts). |
| `publish [path]` | Package + sha256 + JSON manifest + how others install it. |
| `import <file\|url>` | Install a skill from a `.skill` file or URL (zip-slip-safe). |
| `serve-mcp [path...]` | Run an MCP server exposing skills as tools over stdio (`--execute` runs them via your provider). |
| `schema <path>` | Emit cross-provider tool schemas (MCP, OpenAI, Anthropic) for a skill. |
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

`package` verifies the audit chain and records a provenance entry.

## Cross-provider output

From one skill, Skill Forge emits the same tool three ways — `skillforge schema` writes the MCP, OpenAI function-calling, and Anthropic tool definitions, and `skillforge serve-mcp` serves your skills as live MCP tools over stdio. One definition, every provider.

## Format parity

Skill Forge mirrors the official `skill-creator` validation and packaging rules exactly, so anything it generates passes the reference `quick_validate.py` and loads cleanly into Claude.

## Roadmap

- `compile` connectors beyond local files (Box, Drive)
- a skill registry for `publish`/`import` discovery
- richer plugin/marketplace packaging

## License

MIT
