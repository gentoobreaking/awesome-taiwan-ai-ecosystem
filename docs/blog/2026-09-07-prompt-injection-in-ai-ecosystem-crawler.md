---
title: "How Three Throwaway GitHub Repos Tried to Poison Our AI-Ecosystem Registry"
date: 2026-09-07
author: Yuhao Chen
tags: [security, prompt-injection, mcp, ai-ecosystem, incident]
summary: >
  While seeding the Taiwan AI Ecosystem registry from a legacy
  dataset, three GitHub repos turned out to be single-purpose
  prompt-injection payloads disguised as default READMEs. This
  post walks through what we found, how the attack chain works,
  and what we changed in the crawler / seed / view pipeline to
  keep the registry safe to feed to an LLM.
---

# How three throwaway GitHub repos tried to poison our AI-ecosystem registry

We index the Taiwan AI ecosystem — MCP servers, AI agents, datasets,
SDKs — and surface the result as a public registry. The registry is
meant to be consumed by humans *and* by LLMs ("summarise the latest
MCP servers for Taiwan", "draft a market overview", "find candidates
for our internal benchmark"). That last property is exactly what made
the registry a target, and what surfaced a real prompt-injection
campaign in our pipeline.

This post is the writeup.

## What we found

When we re-seeded the registry from the legacy 9/5 dataset (561
candidate records pulled from GitHub keyword search), three of those
records looked like this:

| Repo | "Name" | "Description" |
|---|---|---|
| <https://github.com/clearsdunker-create/ez> | `ez` | A blob of Lua-shaped code starting with `return(function(nq,nL,nW,...)` |
| <https://github.com/XeroxSp/XEZAHUB> | `XEZAHUB` | The same blob, plus a sibling IIFE in a different encoding |
| <https://github.com/ipal1veee/test> | `test` | The same blob, plus an English-language "ignore previous instructions" variant |

A redacted sample:

```lua
return(function(nq,nL,nW,...)
  if not nq then nq = (function()
    local ny = type
    local nV = pairs
    local nw = string and string.byte
    local nm = ny("")
    local nX = ny({})
    local function nt(nR)
      ...
    end
    ...
  end)()
end)
```

It is not Lua, not JavaScript, not anything executable. It is a
**prompt** — a string the attacker hoped a downstream reader (a
human, a crawler, or an LLM) would treat as instructions.

## The attack chain

```
  attacker                        github.com                          our crawler                our registry             downstream LLM
     │                                 │                                  │                          │                          │
     │  creates repo with default     │                                  │                          │                          │
     │  README = obfuscated payload   │                                  │                          │                          │
     ├───────────────────────────────►│                                  │                          │                          │
     │                                 │  GitHub search / sitemap scrape  │                          │                          │
     │                                 ├─────────────────────────────►    │                          │                          │
     │                                 │                                  │  stores raw README snippet │                          │
     │                                 │                                  │  as entity.description     │                          │
     │                                 │                                  ├──────────────►           │                          │
     │                                 │                                  │                          │  view generator writes    │
     │                                 │                                  │  description into         │                          │
     │                                 │                                  │  taiwan-ai-ecosystem.md   │                          │
     │                                 │                                  │                          │                          │
     │                                 │                                  │                          │  "summarise the latest   │
     │                                 │                                  │                          │   MCP servers"           │
     │                                 │                                  │                          ├──────────────────────►   │
     │                                 │                                  │                          │                          │
     │                                 │                                  │                          │   model reads the markdown│
     │                                 │                                  │                          │   as a reference, but the │
     │                                 │                                  │                          │   embedded description is │
     │                                 │                                  │                          │   read as instruction     │
```

The interesting part is that **no real code is involved at any
stage**. The attacker doesn't need the victim to run anything. They
just need the victim to put the README text into a context where an
LLM will see it.

That is the OWASP LLM01 pattern: indirect prompt injection through
data that an LLM will eventually read.

## Why our pipeline missed it

The first version of the pipeline had three layers that *should* have
caught this and didn't:

1. **Keyword search for "MCP" / "Taiwan" / "AI" / "agent"** in
   the repository description. The attacker used a description that
   *was* the payload, so keyword matching couldn't tell the two
   apart. The repo would still surface in search.
2. **The legacy `registry/registry.json`** the project shipped in
   9/5 (used as the seed for our dev environment) had no
   `category`, no `description` validation, and a passthrough
   model. Whatever was in the upstream README went straight into
   the v2 entities table.
3. **The view generator** wrote `e.Description` into the rendered
   `taiwan-ai-ecosystem.md` verbatim. No sanitization.

In other words: the prompt-injection payload was inside what our
pipeline *trusted* (the upstream `registry.json` JSON), and the
view generator did not have a layer that said "wait, this looks
like a programming language, not a description of a project".

## What we changed

Three layered fixes — defence in depth, not just one magic regex.

### 1. `cmd/seed` — strip obvious payloads at the data boundary

`cmd/seed/main.go` now has a `sanitizeDescription` step that drops
the description if it contains any of:

- `return(function(` (the IIFE wrapper)
- `local ny=type`, `local nm=ny("")`, `string.byte(` (Lua
  obfuscation scaffolding)
- `\x5f\x5f` (escaped `__` — a giveaway for code-as-text)
- more than 2 KB of description, which is wildly more than a
  project description should be

A round-trip with the seed tool now removes the 3 contaminated
entities from `entities.description` and the regenerated view
files contain zero `return(function` matches.

### 2. `security_scanner` — detect the same patterns in real time

This is the part that had been a TODO. The original
`injection_exporter` produced an `INJECTION_REPORT.md`, but the
upstream `security_scanner` never emitted `prompt_injection` or
`injection` findings — so the report was always empty. We've
added a new `scanPromptInjection` pass with five patterns:

- `lua_iife_payload`: `return\s*\(\s*function\s*\(`
- `lua_obfuscation_markers`: `local ny=type`, `local nm=ny("")`, `string.byte(`
- `hex_encoded_payload`: a long run of `\xNN` escapes
- `english_override`: "ignore/disregard/forget ... previous/prior/system ... instructions/prompts/rules"
- `english_jailbreak`: "do anything now", "reveal the system prompt", etc.

Any hit becomes a `prompt_injection` finding with HIGH or MEDIUM
severity, and `injection_exporter` finally has something to put
into `INJECTION_REPORT.md`. The end-to-end test lives in
`security_scanner_test.go::TestSecurityScanner_PromptInjection`
and covers both the real-world Lua payload and an English
jailbreak phrase.

### 3. The legacy artifact is gone

`registry/REGISTRY.md` was a 9/5 file that the view generator
*never overwrites* — it just sat in the directory. It was a
duplicate copy of the same poisoned data, so it was the most
likely URL to be seen first. Deleted; the only entry point now is
the regenerated `taiwan-ai-{ecosystem,agents,tools,…}.md` files.

## What we still don't have

A view-layer sanitizer. `view_generator.go` still writes
`e.Description` into the rendered markdown with no filtering. The
data-side fix and the detector-side fix together reduce the
attack surface to "what passes the seed-time heuristic" — but
that's a strict superset of the runtime detector, not a
replacement. If a future attacker lands a payload the seed
heuristic doesn't catch, the view markdown will still carry it.

The right fix is a third pass in `view_generator.go` that calls
`sanitizeDescription` (or an analogous function on the markdown
output) before writing. We are tracking that as the next change.

## Takeaways for other AI-crawler maintainers

If you maintain a registry, a dataset, or any pipeline that ingests
publicly-written text and later feeds it to a model, the
inevitable has happened: an attacker can write README, code
comments, package descriptions, or *anywhere* the model will read,
and steer your model.

A few specific notes from this incident:

- **"Add an LLM" was not the step that introduced the risk.** The
  original crawler was already ingesting README text into a
  public dataset. The risk existed from day one. LLM consumption
  just made the consequence visible.
- **Pattern detection at write time beats sanitisation at read
  time.** Patterns like `return(function(`, `\x5f\x5f`, hex escape
  runs, and "ignore previous instructions" are cheap to check and
  catch the bulk of the problem.
- **Trust your data source, even if it is your own.** A legacy
  `registry.json` that has been on disk for a year can carry
  payloads you never saw when you wrote it. A re-seed + diff
  against the current view is a useful exercise.
- **The view generator is the wrong place for trust decisions.**
  Decide what's safe at ingest, not at render. The render step
  should be mechanical.

## Artifacts

- The contaminated records were `clearsdunker-create/ez`,
  `XeroxSp/XEZAHUB`, and `ipal1veee/test` on GitHub. We've reported
  all three to GitHub Trust & Safety.
- The fix commit series is in this repository
  (`gentoobreaking/awesome-taiwan-ai-ecosystem`):
  - `2bf4726` — data-side: `cmd/seed` sanitisation, delete
    legacy `REGISTRY.md`
  - `eb0e90c` — detector-side: `security_scanner` adds
    `scanPromptInjection` with 5 patterns and a test
  - `a00fa0b` — UX: a navigable `INDEX.md` so an operator can
    see at a glance which views exist and what each filters for
- The full payload is preserved in commit `9782a17` of this
  repository if you want to inspect the bytes directly.
