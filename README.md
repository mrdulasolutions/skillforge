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
skillforge new                        # describe your skill in plain words; AI drafts it
skillforge build pdf-extractor        # validate + best-practice warnings
skillforge build pdf-extractor --optimize --fix   # AI-improve the description
skillforge package pdf-extractor      # bundle → pdf-extractor.skill
skillforge doctor                     # check providers & environment
```

## Commands

| Command | What it does |
|---|---|
| `setup` | Configure an AI provider (OpenRouter or Ollama), store the key securely, and verify it with a live test call. |
| `new [name]` | Build a skill conversationally — describe it in plain words and AI drafts the name, description, and SKILL.md, which you refine by chatting. Auto-derives the kebab-case name (never type it). Falls back to a quick form with no AI; `-y` for non-interactive; `--type plugin`, `--compliance`. |
| `build [path]` | Validate `SKILL.md` (frontmatter rules + warnings). `--optimize` refines the description via AI; `--fix` applies it; `--json` for CI. |
| `package [path]` | Validate, then zip into a `.skill` (excludes `evals/` and build artifacts). |
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

## Format parity

Skill Forge mirrors the official `skill-creator` validation and packaging rules exactly, so anything it generates passes the reference `quick_validate.py` and loads cleanly into Claude.

## Roadmap

- `eval` / `test` across providers (benchmark parity with skill-creator)
- `serve-mcp` — expose skills as MCP tools + emit OpenAI fallback schemas
- `compile` — synthesize a skill from local files and connectors (Box)
- `publish` / `import`

## License

MIT
