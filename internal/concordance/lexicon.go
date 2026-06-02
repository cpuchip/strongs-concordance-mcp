// Package concordance loads and queries the bundled Strong's dual lexicon.
package concordance

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Entry is one merged lexicon record (the consumer-side mirror of the shape
// cmd/build-data emits; JSON tags are the contract).
type Entry struct {
	Number     string `json:"number"`
	Lang       string `json:"lang"`
	Lemma      string `json:"lemma,omitempty"`
	Translit   string `json:"translit,omitempty"`
	Pron       string `json:"pron,omitempty"`
	StrongsDef string `json:"strongs_def,omitempty"`
	KJVDef     string `json:"kjv_def,omitempty"`
	Derivation string `json:"derivation,omitempty"`
	StepGloss  string `json:"step_gloss,omitempty"`
	StepDef    string `json:"step_def,omitempty"`
	StepMorph  string `json:"step_morph,omitempty"`
}

// Lexicon holds the merged entries keyed by canonical Strong's number.
type Lexicon struct {
	entries map[string]*Entry
}

// Load decompresses and parses the bundled gzipped JSON lexicon.
func Load(gzData []byte) (*Lexicon, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var m map[string]*Entry
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &Lexicon{entries: m}, nil
}

// Count returns the number of loaded entries.
func (l *Lexicon) Count() int { return len(l.entries) }

// Canon normalizes user input ("h7225", "G 26", "H0001", "G0026G") to the
// canonical unpadded classic key ("H7225", "G26").
func Canon(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
	if raw == "" {
		return ""
	}
	p := raw[0]
	if p == 'h' {
		p = 'H'
	} else if p == 'g' {
		p = 'G'
	}
	if p != 'H' && p != 'G' {
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
	return string(p) + strconv.Itoa(n)
}

// Define looks up a Strong's number.
func (l *Lexicon) Define(number string) (*Entry, bool) {
	c := Canon(number)
	if c == "" {
		return nil, false
	}
	e, ok := l.entries[c]
	return e, ok
}

// Search does reverse lookup by KJV English word, gloss, or transliteration.
func (l *Lexicon) Search(query string, max int) []*Entry {
	if max <= 0 {
		max = 20
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	type scored struct {
		e *Entry
		s int
	}
	var hits []scored
	for _, e := range l.entries {
		if s := matchScore(q, e); s > 0 {
			hits = append(hits, scored{e, s})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].s != hits[j].s {
			return hits[i].s > hits[j].s
		}
		return numLess(hits[i].e.Number, hits[j].e.Number)
	})
	out := make([]*Entry, 0, max)
	for _, h := range hits {
		out = append(out, h.e)
		if len(out) >= max {
			break
		}
	}
	return out
}

func matchScore(q string, e *Entry) int {
	gloss := strings.ToLower(e.StepGloss)
	kjv := strings.ToLower(e.KJVDef)
	sdef := strings.ToLower(e.StrongsDef)
	translit := strings.ToLower(e.Translit)
	switch {
	case gloss == q || translit == q:
		return 100
	}
	best := 0
	bump := func(v int) {
		if v > best {
			best = v
		}
	}
	if hasWord(gloss, q) {
		bump(80)
	}
	if hasWord(kjv, q) {
		bump(70)
	}
	if hasWord(sdef, q) {
		bump(60)
	}
	if strings.Contains(translit, q) {
		bump(45)
	}
	if strings.Contains(gloss, q) {
		bump(40)
	}
	if strings.Contains(kjv, q) {
		bump(30)
	}
	if strings.Contains(strings.ToLower(e.StepDef), q) {
		bump(20)
	}
	return best
}

// hasWord reports whether w appears in s bounded by non-letters.
func hasWord(s, w string) bool {
	for idx := 0; idx < len(s); {
		j := strings.Index(s[idx:], w)
		if j < 0 {
			return false
		}
		j += idx
		beforeOK := j == 0 || !isLetter(s[j-1])
		afterOK := j+len(w) >= len(s) || !isLetter(s[j+len(w)])
		if beforeOK && afterOK {
			return true
		}
		idx = j + 1
	}
	return false
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func numLess(a, b string) bool {
	if a == "" || b == "" {
		return a < b
	}
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	na, _ := strconv.Atoi(a[1:])
	nb, _ := strconv.Atoi(b[1:])
	return na < nb
}
