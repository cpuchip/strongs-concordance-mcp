// Package strongs embeds the bundled, generated concordance data so the MCP
// binary is fully self-contained (it is spawned by absolute path from an
// arbitrary working directory in both Claude Code and the pg-ai-stewards
// bridge). The data is produced by cmd/build-data from the upstream sources.
package strongs

import _ "embed"

// LexiconGZ is the gzipped JSON dual lexicon (Strong's 1890 + STEPBible),
// keyed by canonical Strong's number. Built by cmd/build-data.
//
//go:embed data/strongs-lexicon.json.gz
var LexiconGZ []byte

// KJVGZ is the gzipped JSON KJV verse tagging (kaiserlik), shaped as
// abbrev -> chapter -> verse -> {text, words[]}. Built by cmd/build-data.
//
//go:embed data/kjv-strongs.json.gz
var KJVGZ []byte
