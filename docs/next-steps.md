# Resume runbook — P3 (`strongs_for_verse`) + P4 (registration)

Executable handoff so a fresh context finishes the build without re-deriving.
**Done so far:** P0 scaffold · P1 dual lexicon (`data/strongs-lexicon.json.gz`,
19,570 entries) · P2 `strongs_define` + `strongs_search` (working, smoke-tested).
All committed + pushed. Build with **`GOWORK=off`** (standalone module).

If `.sources/` is gone (gitignored, not committed), re-fetch:

```sh
cd projects/strongs-concordance-mcp && mkdir -p .sources && cd .sources
git clone --depth 1 https://github.com/kaiserlik/kjv kaiserlik-kjv
# (os-strongs + stepbible only needed to rebuild the lexicon; already bundled)
```

---

## P3 — `strongs_for_verse`

### Source: kaiserlik-kjv (`.sources/kaiserlik-kjv/{ABBR}.json`)

Per-book JSON. **GOTCHA: the top-level key is inconsistent** — `Gen.json` →
`"Gen"`, but `3Jo.json` → `"3 John"`. **Do not trust the top key.** Descend into
the single top value, iterate chapter keys, then verse keys. The **verse key is
the reliable structure**: `"ABBR|chapter|verse"` (e.g. `"Gen|1|1"`). Split on
`|` to get (abbr, ch, vs). Use only the `en` field (other langs bg/ch/sp ignore).

### Book name → abbrev map (from books.json — bake this in for runtime ref resolution)

```
Genesis Gen · Exodus Exo · Leviticus Lev · Numbers Num · Deuteronomy Deu
Joshua Jos · Judges Jdg · Ruth Rth · 1 Samuel 1Sa · 2 Samuel 2Sa
1 Kings 1Ki · 2 Kings 2Ki · 1 Chronicles 1Ch · 2 Chronicles 2Ch · Ezra Ezr
Nehemiah Neh · Esther Est · Job Job · Psalms Psa · Proverbs Pro
Ecclesiastes Ecc · Song of Songs Sng · Isaiah Isa · Jeremiah Jer
Lamentations Lam · Ezekiel Eze · Daniel Dan · Hosea Hos · Joel Joe · Amos Amo
Obadiah Oba · Jonah Jon · Micah Mic · Nahum Nah · Habakkuk Hab · Zephaniah Zep
Haggai Hag · Zechariah Zec · Malachi Mal · Matthew Mat · Mark Mar · Luke Luk
John Jhn · Acts Act · Romans Rom · 1 Corinthians 1Co · 2 Corinthians 2Co
Galatians Gal · Ephesians Eph · Philippians Phl · Colossians Col
1 Thessalonians 1Th · 2 Thessalonians 2Th · 1 Timothy 1Ti · 2 Timothy 2Ti
Titus Tit · Philemon Phm · Hebrews Heb · James Jas · 1 Peter 1Pe · 2 Peter 2Pe
1 John 1Jo · 2 John 2Jo · 3 John 3Jo · Jude Jde · Revelation Rev
```

Runtime ref resolution: accept "John 3:16", "Jhn 3:16", "1 John 2:1". Normalize
(lowercase, collapse spaces); match against full names + abbrevs + common
aliases (e.g. "Song of Solomon"→Sng, "Psalm"→Psa, "Revelation of John"→Rev).
Store output keyed by abbrev so resolution is one lookup.

### Tag parsing rules (the `en` field)

`en` is KJV text with inline `[G####]`/`[H####]` tags **appended after** the
word(s) they tag. Rules learned from real data:

- Tags can **stack**: `created[H1254][H853]` → "created" carries H1254 **and** H853.
- Tags can attach to **function words**: `and[H853]` (H853 = *et*, the untranslated
  direct-object marker). Keep them.
- Some words have **no tag** (e.g. "In", "the" in Gen 1:1) → empty strongs list.
- Parse into an ordered list `[{word, strongs:[...]}]` AND a clean `text`
  (strip all `[...]` tags, normalize spaces).

Parser sketch: regex `(\S+?)((?:\[[GH]\d+\])+)?` won't cleanly handle
punctuation; simpler — split on whitespace into tokens, for each token pull
trailing `\[[GH]\d+\]` groups off the end (repeatedly), the remainder is the
display word (may carry punctuation, which is fine). Accumulate.

### Validation pass (MANDATORY — web-scraped source)

In `cmd/build-data`, after building, print these for eyeball check + assert the
tags are present:

- **Gen 1:1** (verbatim, captured 2026-06-02):
  `In the beginning[H7225] God[H430] created[H1254][H853] the heaven[H8064] and[H853] the earth.`
  → H7225=beginning (matches lexicon ✓), H430=Elohim/God, H1254=bara/created.
- **John 1:1** and **John 3:16** — fetch + eyeball both text and tag accuracy.
- Sanity counts: total verses should be ≈ **31,102** (KJV); books = 66; flag if
  off by more than a little. Log per-book verse counts on `-v`.

If anything looks wrong, STOP and surface to Michael — do not bundle a bad
concordance (it would silently corrupt every word-note built on it).

### Build steps

1. **`cmd/build-data`** — add a second output. Iterate all 66 `{ABBR}.json`,
   parse per above → `data/kjv-strongs.json.gz`. Suggested shape:
   `{ "Gen": { "1": { "1": {"text":"...", "words":[{"w":"In","s":[]},{"w":"the","s":[]},{"w":"beginning","s":["H7225"]},...] } } } }`
   keyed by abbrev/ch/vs (strings). Deterministic (json sorts map keys).
2. **`embed.go`** — add `//go:embed data/kjv-strongs.json.gz` → `var KJVGZ []byte`.
3. **`internal/concordance`** — new `verses.go`: `Verses` type, `LoadVerses(gz)`,
   `ForVerse(book, ch, vs)` → `{text, []{word, strongs}}`. Add a `ResolveRef(s)`
   helper (name/abbrev/alias + "C:V" parse). Reuse `Lexicon` to attach a brief
   gloss per Strong's number in the output.
4. **`internal/mcpserver`** — add `strongs_for_verse` tool (input `reference`,
   e.g. "John 3:16"). Output: clean verse text, then a word-by-word list
   `word → Strong's# (lemma, gloss)`. Hold the `*Lexicon` in `Server` (already
   there) to enrich.
5. **`cmd/strongs-mcp/main.go`** — load `KJVGZ` too; pass verses into the server.
6. Rebuild (`GOWORK=off go build`), smoke `strongs_for_verse "John 3:16"` and
   "Genesis 1:1" via JSON-RPC. Commit + push **P3**.

---

## P4 — register in both spaces

### Claude Code (quick, this workspace)

Add to the repo-root `.mcp.json` (gitignored — has tokens; edit in place):

```json
"strongs": { "command": "C:\\Users\\cpuch\\Documents\\code\\stuffleberry\\scripture-study\\projects\\strongs-concordance-mcp\\strongs-mcp.exe" }
```

Build the `.exe` first (`GOWORK=off go build -o strongs-mcp.exe ./cmd/strongs-mcp`).
Restart Claude Code → first-run approval dialog. Verify `strongs_define`/
`strongs_search`/`strongs_for_verse` appear and work.

### pg-ai-stewards (the substrate — do with care)

The bridge proxies external stdio MCP servers via the `stewards.mcp_servers`
registry — webster is already registered this way. Pattern (see
`projects/pg-ai-stewards/docs/3e-mcp-findings.md` + existing seeds like
`3e2-1-mcp-bridge-schemas.sql`, `3e2-7-git-mcp-seed.sql`):

1. **Soak-pause** (CLAUDE.md substrate §5):
   `UPDATE stewards.watchman_config SET schedule_enabled=false WHERE id=1;`
2. **Seed row** — write `extension/<Nx>-strongs-mcp-seed.sql`:
   `INSERT INTO stewards.mcp_servers (name, transport, command, args, enabled) VALUES ('strongs','stdio','<abs path to strongs-mcp[.exe]>','[]', true) ON CONFLICT (name) DO UPDATE ...`
   **Confirm where the bridge runs first** — findings note it ran on the
   **Windows host** (so the `.exe` path), with Linux-in-Docker as a future shape.
   Match the command path to the bridge's environment. Live-apply via
   `docker cp + psql -f` (no rebuild needed — registry is a table).
3. **Refresh** — `stewards-mcp bridge refresh-tools` (or restart the bridge
   daemon) → caches strongs tools → auto-promotes to `tool_defs`.
4. **Grant** — `agent_tool_perms` (source='manual') for the scripture agents:
   `study`, `lesson`, `talk` → `strongs_define`, `strongs_search`,
   `strongs_for_verse` (mirror the existing webster_define grants).
5. **Smoke** — `refresh-tools` shows `strongs` with 3 tools; a granted agent's
   `compose_tools('study')` includes them.
6. **Soak-resume:** `... SET schedule_enabled=true WHERE id=1;`

This is the only step touching the running substrate — gated, reversible, but
deliberate. Don't `docker compose down -v`. Don't echo secrets.

---

## P5 — finalize

README usage section + a `data/ATTRIBUTION.md` (openscriptures CC-BY-SA,
STEPBible CC BY 4.0, kaiserlik — note the validation). Flip README phase
markers. Update workspace `.mind/active.md` + the proposal status.
