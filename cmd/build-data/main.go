// Command build-data fetches/normalizes the upstream Strong's sources (in
// .sources/, gitignored) into the bundled data/*.json.gz the MCP server ships.
//
// v1 builds the dual lexicon (openscriptures Strong's 1890 + STEPBible
// BDB/Abbott-Smith). The KJV verse-tagging output (kjv-strongs.json.gz) is
// added in P3. Run from the repo root:
//
//	go run ./cmd/build-data
//
// Source formats are documented in docs/data-formats.md.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const sourcesDir = ".sources"

// Entry is one merged lexicon record, keyed by canonical Strong's number.
type Entry struct {
	Number     string `json:"number"`               // canonical, e.g. "H7225"
	Lang       string `json:"lang"`                 // "hebrew" | "greek"
	Lemma      string `json:"lemma,omitempty"`      // original-language word
	Translit   string `json:"translit,omitempty"`   // transliteration
	Pron       string `json:"pron,omitempty"`       // pronunciation (Hebrew)
	StrongsDef string `json:"strongs_def,omitempty"` // Strong's 1890 definition
	KJVDef     string `json:"kjv_def,omitempty"`     // Strong's KJV-usage gloss
	Derivation string `json:"derivation,omitempty"` // Strong's derivation
	StepGloss  string `json:"step_gloss,omitempty"` // STEPBible brief gloss
	StepDef    string `json:"step_def,omitempty"`   // STEPBible meaning (abridged BDB/A-S)
	StepMorph  string `json:"step_morph,omitempty"` // STEPBible morphology code
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

func main() {
	lex := map[string]*Entry{}

	gGrk := loadOpenScriptures(sourcesDir+"/os-strongs/greek/strongs-greek-dictionary.js", "greek", lex)
	gHeb := loadOpenScriptures(sourcesDir+"/os-strongs/hebrew/strongs-hebrew-dictionary.js", "hebrew", lex)
	sHeb := loadStepBible(sourcesDir+"/stepbible/Lexicons/TBESH - Translators Brief lexicon of Extended Strongs for Hebrew - STEPBible.org CC BY.txt", "hebrew", lex)
	sGrk := loadStepBible(sourcesDir+"/stepbible/Lexicons/TBESG - Translators Brief lexicon of Extended Strongs for Greek - STEPBible.org CC BY.txt", "greek", lex)

	// stats
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

	must(os.MkdirAll("data", 0o755), "mkdir data")
	out, err := os.Create("data/strongs-lexicon.json.gz")
	must(err, "create output")
	defer out.Close()
	gz := gzip.NewWriter(out)
	enc := json.NewEncoder(gz)
	must(enc.Encode(lex), "encode") // map keys are sorted by encoding/json -> deterministic
	must(gz.Close(), "gzip close")

	fmt.Printf("openscriptures: %d greek + %d hebrew\n", gGrk, gHeb)
	fmt.Printf("stepbible:      %d greek + %d hebrew primary senses\n", sGrk, sHeb)
	fmt.Printf("merged lexicon: %d entries (%d hebrew, %d greek), %d with STEPBible layer\n", len(lex), heb, grk, withStep)

	// smoke: two known entries
	for _, k := range []string{"H7225", "G26"} {
		b, _ := json.MarshalIndent(lex[k], "", "  ")
		fmt.Printf("\n--- %s ---\n%s\n", k, b)
	}
}
