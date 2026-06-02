# strongs-concordance-mcp

An [MCP](https://modelcontextprotocol.io) server that provides **Strong's Concordance** — Hebrew and Greek word-study data keyed to the King James Bible — as tools for scripture study.

It is the Hebrew/Greek companion to [webster-mcp](https://github.com/cpuchip/webster-mcp): where Webster's 1828 dictionary illuminates the *English* of Restoration-era scripture, Strong's illuminates the *original languages* behind the KJV. Built to make Bible word-work as rich as 1828-English word-work.

## Why

The King James Bible is 1611 English over Hebrew and Greek, and the English often flattens the original (one English "love" over four Greek words; "lovingkindness" over Hebrew *chesed*; "soul" over *nephesh*). Strong's Concordance (James Strong, 1890) is the standard bridge — it keys every KJV word to a numbered Hebrew (`H####`) or Greek (`G####`) lemma, usable without knowing the languages. This server exposes that bridge as MCP tools.

## Design — a dual lexicon (the Webster parallel)

Like webster-mcp shows the 1828 *and* modern definition side by side, this server shows two layers per Strong's number:

1. **Strong's 1890** — James Strong's own definition, KJV-usage gloss, and derivation. The original article (public domain).
2. **STEPBible modern** — abridged **BDB** (Hebrew) / **Abbott-Smith** (Greek) gloss + definition, curated by Tyndale House (CC BY 4.0). Modern scholarship, "extended Strong's" (backward-compatible with classic numbers).

## Tools

| Tool | Input | Returns |
|------|-------|---------|
| `strongs_define` | `number` (e.g. `H7225`, `G26`) | lemma, transliteration, both definition layers (Strong's 1890 + STEPBible), KJV-usage gloss, derivation |
| `strongs_search` | `word` (KJV English or transliteration) | the Strong's number(s) behind that word, with brief glosses (reverse lookup) |
| `strongs_for_verse` | `reference` (e.g. `John 3:16`) | the verse's KJV words, each tagged with its Strong's number — the word-by-word bridge |

Planned (v2+): `strongs_occurrences` (every verse a lemma appears in, for tracing a word across the canon).

## Data sources & licenses

All sources are public-domain or openly licensed (CC BY / CC BY-SA). Data files are bundled offline under their own attribution; the code is MIT.

| Layer | Source | License | Notes |
|-------|--------|---------|-------|
| Strong's 1890 lexicon (Heb + Grk) | [openscriptures/strongs](https://github.com/openscriptures/strongs), [openscriptures/HebrewLexicon](https://github.com/openscriptures/HebrewLexicon) | Strong's text PD; OS compilation CC BY 4.0 / CC BY-SA | the original article |
| Modern lexicon (BDB / Abbott-Smith) | [STEPBible TBESH / TBESG](https://github.com/STEPBible/STEPBible-Data) | CC BY 4.0 (Tyndale House) | extended Strong's |
| KJV verse tagging | [kaiserlik/kjv](https://github.com/kaiserlik/kjv) | (verify) | **web-scraped — must be validated before trusting** |

**Validation discipline:** the KJV↔Strong's tagging is web-scraped, so the build pipeline spot-checks it verse-by-verse against a trusted reference before the data is bundled. A wrong concordance would silently corrupt every word-note built on it. Strong's glosses are a *starting point for study, not doctrine* — Strong's glosses, it does not exegete.

## Architecture

A single stdio MCP server (Go, [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go)) with bundled, offline, gzipped-JSON data — same shape as webster-mcp. The data is built once by a pipeline (`cmd/build-data`) that fetches, normalizes, and validates the sources into `data/*.json.gz`, committed to the repo.

```
cmd/strongs-mcp/main.go     entry — stdio MCP server
cmd/build-data/main.go      pipeline: fetch + normalize + validate -> data/*.json.gz
internal/concordance/       lexicon + tagging load/lookup
internal/mcpserver/         tool registration + handlers
data/*.json.gz              bundled, generated
```

### One binary, two homes

The same stdio binary serves both consumers:

- **Claude Code** — an entry in `.mcp.json`:
  ```json
  { "strongs": { "command": "/abs/path/to/strongs-mcp" } }
  ```
- **pg-ai-stewards** (the substrate) — a row in `stewards.mcp_servers` (transport `stdio`, command = the binary), then `stewards-mcp bridge refresh-tools` caches the tools, they auto-promote to `tool_defs`, and study/lesson agents are granted access. Identical to how webster is already registered there.

## Build phases

- **P0** — scaffold (module, dirs, this README). ✅
- **P1** — lexicon data pipeline: fetch openscriptures/strongs + STEPBible TBESH/TBESG -> unified `data/strongs-lexicon.json.gz` (19,570 entries). ✅
- **P2** — `strongs_define` + `strongs_search` over the lexicon. Smoke-tested. ✅
- **P3** — KJV tagging pipeline (kaiserlik) + **validation pass** -> `data/kjv-strongs.json.gz` (31,102 verses, 66 books); `strongs_for_verse`. Smoke-tested. ✅
- **P4** — registered in both spaces: `.mcp.json` (Claude Code) + the bridge image + `stewards.mcp_servers` seed (`strongs1-mcp-seed.sql`) with study/lesson/talk grants, live-verified via `compose_tools`. ✅
- **P5** — README usage + `data/ATTRIBUTION.md`. ✅

## Build & run

```sh
GOWORK=off go run ./cmd/build-data        # regenerate data/*.json.gz from .sources/
GOWORK=off go build -o strongs-mcp ./cmd/strongs-mcp
./strongs-mcp -stats                       # sanity: prints entry count
./strongs-mcp                              # serve MCP over stdio
```

This is a standalone module deliberately outside the parent workspace's
`go.work`, so builds use `GOWORK=off`. The lexicon + KJV tagging are embedded
in the binary (`embed.go`), so it runs self-contained from any working
directory.

## Usage

Once registered (see "One binary, two homes"), the three tools are called like
any MCP tool. Examples of what they return:

- **`strongs_define`** `{ "number": "H2617" }` → *chêçêd* (kindness; lovingkindness;
  mercy), with Strong's 1890 definition + KJV-usage gloss + derivation, and the
  STEPBible modern gloss/definition side by side.
- **`strongs_search`** `{ "word": "charity" }` → the Strong's numbers KJV renders
  "charity" (e.g. G26 *agápē*, G1654 *eleēmosynē*), ranked, with brief glosses.
- **`strongs_for_verse`** `{ "reference": "John 3:16" }` → the verse text plus the
  word-by-word tagging: `loved → G25 agapáō (to love)`, `world → G2889 kósmos`, …
  The book may be a full name, an abbreviation, or a common alias ("Jn", "Ps",
  "Song of Solomon", "1 Jn"). KJV only.

## Status

**v1 complete (P0–P5).** Dual lexicon (`strongs_define` / `strongs_search`, 19,570
entries) + KJV word-by-word tagging (`strongs_for_verse`, 31,102 verses / 66
books, validated). Spec ratified 2026-06-02 (dual lexicon · `for_verse` in v1 via
validated KJV tagging · full build). Registered in both consumers — Claude Code
(`.mcp.json`) and pg-ai-stewards (bridge image + `stewards.mcp_servers`, granted
to the study/lesson/talk agents). Part of the scripture-study canon-walk
toolchain, built ahead of the Old Testament walk. Planned v2+: `strongs_occurrences`
(every verse a lemma appears in).

## License

Code: MIT (see [LICENSE](LICENSE)). Bundled data retains its source license/attribution (see the data sources table and `data/ATTRIBUTION.md`).
