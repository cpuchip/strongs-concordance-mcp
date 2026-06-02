# Upstream data formats (verified 2026-06-02)

Captured from the actual cloned sources in `.sources/` (gitignored). The build
pipeline (`cmd/build-data`) reads these and emits the bundled `data/*.json.gz`.

## Canonical key

All Strong's numbers are canonicalized to **unpadded classic form**: prefix
(`H`/`G`) + integer, no leading zeros, no extended-letter suffix.
`H0001` → `H1`; `H0001G` (dStrong) → `H1`; `H7225` → `H7225`; `G0026` → `G26`.

## Lexicon layer 1 — openscriptures (Strong's 1890, CC-BY-SA)

- `.sources/os-strongs/greek/strongs-greek-dictionary.js`
- `.sources/os-strongs/hebrew/strongs-hebrew-dictionary.js`

JS-wrapped JSON: a `/** ... */` comment header (no braces), then
`var strongsXDictionary = { ...one big object... }; module.exports = ...;`.
Extract from the first `{` to the last `}`. Entries:

```
"G26":  {"strongs_def":"...","derivation":"from G25 (ἀγαπάω);","translit":"agápē","lemma":"ἀγάπη","kjv_def":"(feast of) charity(-ably), dear, love"}
"H7225":{"lemma":"רֵאשִׁית","xlit":"rêʼshîyth","pron":"ray-sheeth'","derivation":"from the same as H7218 (רֹאשׁ);","strongs_def":"the first, in place, time, order or rank...","kjv_def":"beginning, chief(-est), first(-fruits, part, time)..."}
```

Note: Greek uses `translit`; Hebrew uses `xlit` + adds `pron`. Normalize both
into `translit` (+ keep `pron` when present).

## Lexicon layer 2 — STEPBible (modern BDB/Abbott-Smith, CC BY 4.0)

- `.sources/stepbible/Lexicons/TBESH ... Hebrew ... .txt`
- `.sources/stepbible/Lexicons/TBESG ... Greek ... .txt`

Tab-separated, with a long header block. Data rows start where col0 matches
`^[GH]\d`. Columns:

```
eStrong#  dStrong  uStrong  Hebrew/Greek  Transliteration  Morph  Gloss  Meaning
H0001     H0001G = H0001G   אָב           av               H:N-M  father 1) father of an individual<br>2) of God...
```

Multiple rows share a base `eStrong#` (disambiguated senses); **first row per
canonical base wins** for the primary gloss/meaning/morph. `Meaning` contains
`<br>`, `<b>`, `<i>`, `<ref=...>` markup → strip tags, convert `<br>`→`; `.

(TFLSJ = full LSJ Greek, much larger — not used in v1; TBESG's Abbott-Smith is
enough. Could add TFLSJ as a third Greek layer later.)

## Verse tagging — kaiserlik/kjv (KJV-with-Strong's; web-scraped → VALIDATE)

Per-book JSON files named by 3-letter abbrev (`3Jo.json`, `Act.json`, ...),
plus `books.json` (full-name → abbrev map) and `chapter_count.json`. Structure:

```json
{ "3 John": { "3Jo|1": {
    "3Jo|1|1": {
      "en": "The elder[G4245] unto the wellbeloved[G27] Gaius,[G1050] whom[G3739] I[G1473] love[G25] in[G1722] the truth.[G225]",
      "bg": "...", "ch": "...", "sp": "..."   // other languages — ignore, en only
} } } }
```

The `en` field is the KJV verse text with inline `[G####]`/`[H####]` tags
appended after each tagged word. `for_verse` parses `en` into an ordered list
of `{word, strongs[]}`. A tag attaches to the word(s) immediately preceding it.

Verse key = `ABBR|chapter|verse`. Inner top key = full book name. Build a
name↔abbrev index from `books.json` to resolve references like "John 3:16".

**Validation (mandatory before trusting):** spot-check several verses
(e.g. Gen 1:1, John 1:1, John 3:16) — both the KJV text and the tag accuracy —
against a trusted reference. Inspected sample 3Jo|1|1 checks out (G4245
presbyteros=elder, G27 agapetos=wellbeloved, G25 agapao=love, G225
aletheia=truth). Web-scraped origin means systematic spot-checking is required.

## Versification note

kaiserlik uses KJV English versification (good — matches `gospel-library` KJV).
Hebrew-text sources (OSHB) differ in places; not used in v1, so no remap needed.
