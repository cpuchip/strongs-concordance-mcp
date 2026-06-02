# Data attribution

The bundled `data/*.json.gz` files are built (by `cmd/build-data`) from the
public-domain / openly-licensed sources below. The code is MIT; the bundled
data retains its source licensing and the attributions here.

## `strongs-lexicon.json.gz` — the dual lexicon (19,570 entries)

| Layer | Source | License |
|-------|--------|---------|
| Strong's 1890 (Hebrew + Greek): lemma, transliteration, definition, KJV-usage gloss, derivation | [openscriptures/strongs](https://github.com/openscriptures/strongs), [openscriptures/HebrewLexicon](https://github.com/openscriptures/HebrewLexicon) | James Strong's text is **public domain**; the OpenScriptures machine-readable compilation is **CC BY 4.0 / CC BY-SA** |
| Modern lexicon (abridged BDB for Hebrew, Abbott-Smith for Greek): gloss, definition, morphology — "extended Strong's" | [STEPBible TBESH / TBESG](https://github.com/STEPBible/STEPBible-Data) (Tyndale House) | **CC BY 4.0** |

## `kjv-strongs.json.gz` — KJV verse tagging (31,102 verses, 66 books)

| Layer | Source | License |
|-------|--------|---------|
| King James Version text | (public domain in the United States) | public domain |
| Word-by-word KJV → Strong's tagging | [kaiserlik/kjv](https://github.com/kaiserlik/kjv) | KJV text public domain; the tagging is community-compiled (originally web-scraped) |

**Validation discipline.** The KJV↔Strong's tagging is community-compiled, so
the build pipeline (`cmd/build-data`) validates it before bundling: exact-text
checks against known verses (Gen 1:1, John 1:1, John 3:16, Ps 23:1), Strong's
tag spot-checks, structural counts (31,102 verses / 66 books / 0 duplicate-text
conflicts), and a global scan asserting no residual markup (`<em>` italics,
`[[…]]` Psalm superscriptions, `[fn]` markers) leaks into any verse. A wrong
concordance would silently corrupt every word-note built on it, so the build
hard-fails on any drift.

## A note on Strong's glosses

Strong's numbers and their glosses are a **starting point for study, not
doctrine**. Strong's glosses; it does not exegete. A one-word gloss can flatten
or mislead — always read the fuller definition (and the STEPBible layer) and,
for anything load-bearing, the verse in context.
