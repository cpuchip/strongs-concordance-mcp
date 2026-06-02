// Command build-data fetches/normalizes the upstream Strong's sources (in
// .sources/, gitignored) into the bundled data/*.json.gz the MCP server ships.
//
// It builds two outputs:
//   - data/strongs-lexicon.json.gz — the dual lexicon (openscriptures Strong's
//     1890 + STEPBible BDB/Abbott-Smith), keyed by canonical Strong's number.
//   - data/kjv-strongs.json.gz — the KJV verse tagging (kaiserlik), abbrev ->
//     chapter -> verse -> {text, words[]}, the word-by-word Strong's bridge.
//
// Run from the repo root:
//
//	GOWORK=off go run ./cmd/build-data
//
// Source formats are documented in docs/data-formats.md.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const sourcesDir = ".sources"

// Entry is one merged lexicon record, keyed by canonical Strong's number.
type Entry struct {
	Number     string `json:"number"`                // canonical, e.g. "H7225"
	Lang       string `json:"lang"`                  // "hebrew" | "greek"
	Lemma      string `json:"lemma,omitempty"`       // original-language word
	Translit   string `json:"translit,omitempty"`    // transliteration
	Pron       string `json:"pron,omitempty"`        // pronunciation (Hebrew)
	StrongsDef string `json:"strongs_def,omitempty"` // Strong's 1890 definition
	KJVDef     string `json:"kjv_def,omitempty"`     // Strong's KJV-usage gloss
	Derivation string `json:"derivation,omitempty"`  // Strong's derivation
	StepGloss  string `json:"step_gloss,omitempty"`  // STEPBible brief gloss
	StepDef    string `json:"step_def,omitempty"`    // STEPBible meaning (abridged BDB/A-S)
	StepMorph  string `json:"step_morph,omitempty"`  // STEPBible morphology code
}

// osEntry is the raw openscriptures shape (Greek uses translit; Hebrew uses xlit+pron).
type osEntry struct {
	Lemma      string `json:"lemma"`
	Xlit       string `json:"xlit"`
	Translit   string `json:"translit"`
	Pron       string `json:"pron"`
	StrongsDef string `json:"strongs_def"`
	KJVDef     string `json:"kjv_def"`
	Derivation string `json:"derivation"`
}

var tagRe = regexp.MustCompile(`(?i)<br\s*/?>`)
var anyTagRe = regexp.MustCompile(`<[^>]+>`)
var dataRowRe = regexp.MustCompile(`^[GH]\d`)

// canon normalizes any Strong's id to unpadded classic form: H0001/H0001G -> H1.
func canon(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || (raw[0] != 'H' && raw[0] != 'G') {
		return ""
	}
	i := 1
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		i++
	}
	n, err := strconv.Atoi(raw[1:i])
	if err != nil || n == 0 {
		return ""
	}
	return fmt.Sprintf("%c%d", raw[0], n)
}

func must(err error, ctx string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL %s: %v\n", ctx, err)
		os.Exit(1)
	}
}

// loadOpenScriptures parses a strongs-*-dictionary.js (JS-wrapped JSON object).
func loadOpenScriptures(path, lang string, out map[string]*Entry) int {
	b, err := os.ReadFile(path)
	must(err, "read "+path)
	s := string(b)
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j <= i {
		must(fmt.Errorf("no JSON object found"), path)
	}
	var m map[string]osEntry
	must(json.Unmarshal([]byte(s[i:j+1]), &m), "parse "+path)
	n := 0
	for k, v := range m {
		c := canon(k)
		if c == "" {
			continue
		}
		e := out[c]
		if e == nil {
			e = &Entry{Number: c, Lang: lang}
			out[c] = e
		}
		e.Lemma = strings.TrimSpace(v.Lemma)
		if v.Translit != "" {
			e.Translit = strings.TrimSpace(v.Translit)
		} else {
			e.Translit = strings.TrimSpace(v.Xlit)
		}
		e.Pron = strings.TrimSpace(v.Pron)
		e.StrongsDef = strings.TrimSpace(v.StrongsDef)
		e.KJVDef = strings.TrimSpace(v.KJVDef)
		e.Derivation = strings.TrimSpace(v.Derivation)
		n++
	}
	return n
}

// cleanMarkup strips the STEPBible HTML-ish markup into a plain string.
func cleanMarkup(s string) string {
	s = tagRe.ReplaceAllString(s, "; ")
	s = anyTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace
	return strings.TrimSpace(strings.Trim(s, "; "))
}

// loadStepBible parses a TBESH/TBESG tab-separated lexicon; first row per
// canonical base wins (the primary sense).
func loadStepBible(path, lang string, out map[string]*Entry) int {
	f, err := os.Open(path)
	must(err, "open "+path)
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		if !dataRowRe.MatchString(line) {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 8 {
			continue
		}
		c := canon(cols[0])
		if c == "" {
			continue
		}
		e := out[c]
		if e == nil {
			e = &Entry{Number: c, Lang: lang}
			out[c] = e
		}
		if e.StepGloss == "" && e.StepDef == "" { // first (primary) wins
			e.StepGloss = strings.TrimSpace(cols[6])
			e.StepDef = cleanMarkup(cols[7])
			e.StepMorph = strings.TrimSpace(cols[5])
			n++
		}
	}
	must(sc.Err(), "scan "+path)
	return n
}

// buildLexicon merges the openscriptures + STEPBible sources into the dual
// lexicon and writes data/strongs-lexicon.json.gz.
func buildLexicon() {
	lex := map[string]*Entry{}

	gGrk := loadOpenScriptures(sourcesDir+"/os-strongs/greek/strongs-greek-dictionary.js", "greek", lex)
	gHeb := loadOpenScriptures(sourcesDir+"/os-strongs/hebrew/strongs-hebrew-dictionary.js", "hebrew", lex)
	sHeb := loadStepBible(sourcesDir+"/stepbible/Lexicons/TBESH - Translators Brief lexicon of Extended Strongs for Hebrew - STEPBible.org CC BY.txt", "hebrew", lex)
	sGrk := loadStepBible(sourcesDir+"/stepbible/Lexicons/TBESG - Translators Brief lexicon of Extended Strongs for Greek - STEPBible.org CC BY.txt", "greek", lex)

	var heb, grk, withStep int
	for _, e := range lex {
		if e.Lang == "hebrew" {
			heb++
		} else {
			grk++
		}
		if e.StepGloss != "" || e.StepDef != "" {
			withStep++
		}
	}

	writeGz("data/strongs-lexicon.json.gz", lex)

	fmt.Printf("openscriptures: %d greek + %d hebrew\n", gGrk, gHeb)
	fmt.Printf("stepbible:      %d greek + %d hebrew primary senses\n", sGrk, sHeb)
	fmt.Printf("merged lexicon: %d entries (%d hebrew, %d greek), %d with STEPBible layer\n", len(lex), heb, grk, withStep)

	for _, k := range []string{"H7225", "G26"} {
		b, _ := json.MarshalIndent(lex[k], "", "  ")
		fmt.Printf("  %s -> %s %s\n", k, lex[k].Lemma, lex[k].Translit)
		_ = b
	}
}

// --- KJV verse tagging (P3) -------------------------------------------------

// KJVWord is one display word and the Strong's numbers it carries.
type KJVWord struct {
	W string   `json:"w"`
	S []string `json:"s,omitempty"`
}

// KJVVerse is the clean verse text plus its word-by-word Strong's tagging.
type KJVVerse struct {
	Text  string    `json:"text"`
	Words []KJVWord `json:"words"`
}

// The kaiserlik per-book files are messy: filenames don't always match their
// contents (1Ch.json holds 1 Chronicles repeated ~16x), some files concatenate
// extra books, and bg/ch/sp fields have unescaped quotes that break a strict
// JSON parse. But the *en* field is always the first key in each verse object
// and is cleanly escaped, and the verse KEY ("ABBR|ch|vs") is authoritative.
// So we regex-extract every (verse-key, en) pair across all files and dedup by
// verse key. Verified: 31,102 unique keys, 66 books, zero text conflicts.
var versePat = regexp.MustCompile(`"([^"|]+\|\d+\|\d+)":\s*\{\s*"en"\s*:\s*"((?:\\.|[^"\\])*)"`)
var kjvTagPat = regexp.MustCompile(`\[([GH]\d+)\]`)
var kjvHTMLPat = regexp.MustCompile(`<[^>]*>`)     // KJV italics: <em>supplied</em>
var kjvFnPat = regexp.MustCompile(`\[[^\]]*\]`)    // residual editorial markers, e.g. [fn]

// non-verse files in the kaiserlik dir to skip.
var kjvSkip = map[string]bool{"books.json": true, "chapter_count.json": true, "lexicon.json": true}

// cleanWord strips the residual editorial markup the kaiserlik text carries
// once the Strong's tags are off: [[ ]] Psalm-superscription delimiters and
// [fn]-style footnote markers. The word itself (e.g. "David.") is kept.
func cleanWord(w string) string {
	w = strings.ReplaceAll(w, "[[", "")
	w = strings.ReplaceAll(w, "]]", "")
	w = kjvFnPat.ReplaceAllString(w, "")
	return w
}

// parseEn splits a tagged KJV verse ("In the beginning[H7225] God[H430]...")
// into clean text + word/Strong's pairs. It first strips HTML italics markup
// (<em>…</em> for translator-supplied words), then per token pulls the Strong's
// tags and cleans residual superscription/footnote brackets.
func parseEn(en string) KJVVerse {
	en = kjvHTMLPat.ReplaceAllString(en, "")
	var words []KJVWord
	var display []string
	for _, tok := range strings.Fields(en) {
		var nums []string
		for _, m := range kjvTagPat.FindAllStringSubmatch(tok, -1) {
			if c := canon(m[1]); c != "" {
				nums = append(nums, c)
			}
		}
		w := cleanWord(kjvTagPat.ReplaceAllString(tok, ""))
		if w == "" {
			// tag-only / markup-only token: attach its numbers to the previous word.
			if len(words) > 0 {
				words[len(words)-1].S = append(words[len(words)-1].S, nums...)
			}
			continue
		}
		words = append(words, KJVWord{W: w, S: nums})
		display = append(display, w)
	}
	return KJVVerse{Text: strings.Join(display, " "), Words: words}
}

// buildKJV parses the kaiserlik KJV-with-Strong's tagging into
// data/kjv-strongs.json.gz, validating against known verses before writing.
func buildKJV() {
	dir := filepath.Join(sourcesDir, "kaiserlik-kjv")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	must(err, "glob kaiserlik")

	// abbrev -> chapter(str) -> verse(str) -> *KJVVerse
	out := map[string]map[string]map[string]*KJVVerse{}
	rawPairs, conflicts := 0, 0
	for _, path := range files {
		if kjvSkip[filepath.Base(path)] {
			continue
		}
		b, err := os.ReadFile(path)
		must(err, "read "+path)
		for _, m := range versePat.FindAllStringSubmatch(string(b), -1) {
			vk := m[1]
			parts := strings.Split(vk, "|")
			if len(parts) != 3 {
				continue
			}
			abbr, ch, vs := parts[0], parts[1], parts[2]
			var en string
			must(json.Unmarshal([]byte(`"`+m[2]+`"`), &en), "unescape "+vk)
			rawPairs++
			if out[abbr] == nil {
				out[abbr] = map[string]map[string]*KJVVerse{}
			}
			if out[abbr][ch] == nil {
				out[abbr][ch] = map[string]*KJVVerse{}
			}
			parsed := parseEn(en)
			if prev, ok := out[abbr][ch][vs]; ok {
				if prev.Text != parsed.Text {
					conflicts++
					fmt.Fprintf(os.Stderr, "WARN conflict %s: %q vs %q\n", vk, prev.Text, parsed.Text)
				}
				continue // first wins
			}
			v := parsed
			out[abbr][ch][vs] = &v
		}
	}

	// counts
	totalVerses, books := 0, len(out)
	for _, chs := range out {
		for _, vss := range chs {
			totalVerses += len(vss)
		}
	}

	validateKJV(out, rawPairs, conflicts, totalVerses, books)

	writeGz("data/kjv-strongs.json.gz", out)
	fmt.Printf("kjv tagging:    %d raw pairs -> %d unique verses, %d books, %d conflicts\n",
		rawPairs, totalVerses, books, conflicts)
}

// validateKJV is the mandatory pre-bundle gate: a wrong concordance would
// silently corrupt every word-note built on it, so we hard-fail on drift.
func validateKJV(out map[string]map[string]map[string]*KJVVerse, rawPairs, conflicts, totalVerses, books int) {
	get := func(abbr, ch, vs string) *KJVVerse {
		if out[abbr] != nil && out[abbr][ch] != nil {
			return out[abbr][ch][vs]
		}
		return nil
	}
	const expectVerses, expectBooks = 31102, 66

	type check struct{ abbr, ch, vs, wantText string }
	checks := []check{
		{"Gen", "1", "1", "In the beginning God created the heaven and the earth."},
		{"Jhn", "1", "1", "In the beginning was the Word, and the Word was with God, and the Word was God."},
		{"Jhn", "3", "16", "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life."},
		// Ps 23:1 exercises both markup cases: a [[superscription]] and an
		// <em>italic</em> supplied word ("is"). Must come out clean.
		{"Psa", "23", "1", "A Psalm of David. The LORD is my shepherd; I shall not want."},
	}
	// known tag spot-checks: a specific word must carry a specific number.
	type tagCheck struct {
		abbr, ch, vs, word, want string
	}
	tagChecks := []tagCheck{
		{"Gen", "1", "1", "beginning", "H7225"},
		{"Gen", "1", "1", "God", "H430"},
		{"Gen", "1", "1", "created", "H1254"},
		{"Jhn", "3", "16", "loved", "G25"},
	}

	fail := false
	fmt.Println("--- KJV validation ---")
	for _, c := range checks {
		v := get(c.abbr, c.ch, c.vs)
		if v == nil {
			fmt.Fprintf(os.Stderr, "FATAL %s %s:%s missing\n", c.abbr, c.ch, c.vs)
			fail = true
			continue
		}
		ok := v.Text == c.wantText
		fmt.Printf("  %s %s:%s text %s\n", c.abbr, c.ch, c.vs, okMark(ok))
		if !ok {
			fmt.Fprintf(os.Stderr, "    got:  %q\n    want: %q\n", v.Text, c.wantText)
			fail = true
		}
	}
	for _, tc := range tagChecks {
		v := get(tc.abbr, tc.ch, tc.vs)
		hit := false
		if v != nil {
			for _, w := range v.Words {
				if w.W == tc.word {
					for _, s := range w.S {
						if s == tc.want {
							hit = true
						}
					}
				}
			}
		}
		fmt.Printf("  %s %s:%s %q->%s %s\n", tc.abbr, tc.ch, tc.vs, tc.word, tc.want, okMark(hit))
		if !hit {
			fail = true
		}
	}
	vOK, bOK := totalVerses == expectVerses, books == expectBooks
	fmt.Printf("  verses %d (want %d) %s | books %d (want %d) %s | conflicts %d\n",
		totalVerses, expectVerses, okMark(vOK), books, expectBooks, okMark(bOK), conflicts)
	if !vOK || !bOK || conflicts != 0 {
		fail = true
	}

	// Global cleanliness: no residual markup may leak into any verse text or
	// word across all 31k verses. (Three clean sample verses hid the <em> /
	// [[superscription]] / [fn] markup that pervades the Psalms and epistles —
	// this scan closes that gap so a markup leak can never ship silently.)
	markupBad := 0
	var markupSample string
	for abbr, chs := range out {
		for ch, vss := range chs {
			for vs, v := range vss {
				if strings.ContainsAny(v.Text, "<>[]") {
					markupBad++
					if markupSample == "" {
						markupSample = fmt.Sprintf("%s %s:%s text %q", abbr, ch, vs, v.Text)
					}
				}
				for _, w := range v.Words {
					if strings.ContainsAny(w.W, "<>[]") {
						markupBad++
						if markupSample == "" {
							markupSample = fmt.Sprintf("%s %s:%s word %q", abbr, ch, vs, w.W)
						}
					}
				}
			}
		}
	}
	fmt.Printf("  markup scan: %d verses/words with residual <>[] %s\n", markupBad, okMark(markupBad == 0))
	if markupBad != 0 {
		fmt.Fprintf(os.Stderr, "    first offender: %s\n", markupSample)
		fail = true
	}
	if fail {
		fmt.Fprintln(os.Stderr, "FATAL KJV validation failed — refusing to bundle a bad concordance.")
		os.Exit(1)
	}
	fmt.Println("  all KJV checks passed.")
}

func okMark(ok bool) string {
	if ok {
		return "OK"
	}
	return "FAIL"
}

// writeGz encodes v as gzipped JSON to path. encoding/json sorts map keys, so
// the output is deterministic.
func writeGz(path string, v any) {
	must(os.MkdirAll(filepath.Dir(path), 0o755), "mkdir "+filepath.Dir(path))
	out, err := os.Create(path)
	must(err, "create "+path)
	defer out.Close()
	gz := gzip.NewWriter(out)
	enc := json.NewEncoder(gz)
	must(enc.Encode(v), "encode "+path)
	must(gz.Close(), "gzip close "+path)
}

func main() {
	buildLexicon()
	buildKJV()
}
