package concordance

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Word is one display word of a KJV verse and the Strong's numbers it carries.
type Word struct {
	W string   `json:"w"`
	S []string `json:"s,omitempty"`
}

// Verse is the clean KJV verse text plus its word-by-word Strong's tagging.
type Verse struct {
	Text  string `json:"text"`
	Words []Word `json:"words"`
}

// Verses holds the KJV tagging keyed abbrev -> chapter -> verse, plus the
// reference-resolution table (book name/alias -> kaiserlik abbrev).
type Verses struct {
	books map[string]map[string]map[string]*Verse
}

// LoadVerses decompresses and parses the bundled gzipped KJV tagging.
func LoadVerses(gzData []byte) (*Verses, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var m map[string]map[string]map[string]*Verse
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &Verses{books: m}, nil
}

// Count returns the total number of tagged verses loaded.
func (v *Verses) Count() int {
	n := 0
	for _, chs := range v.books {
		for _, vss := range chs {
			n += len(vss)
		}
	}
	return n
}

// ForVerse looks up a verse by kaiserlik abbrev + chapter + verse (all already
// resolved/normalized — see ResolveRef).
func (v *Verses) ForVerse(abbr, ch, vs string) (*Verse, bool) {
	if chs, ok := v.books[abbr]; ok {
		if vss, ok := chs[ch]; ok {
			if verse, ok := vss[vs]; ok {
				return verse, true
			}
		}
	}
	return nil, false
}

var refRe = regexp.MustCompile(`^(.*?)\s*(\d+)\s*[:.]\s*(\d+)$`)

// ResolveRef parses a human reference ("John 3:16", "1 John 2:1", "Gen 1:1",
// "Song of Solomon 2:1") into a kaiserlik abbrev + chapter + verse. The book
// part is matched against full names, the kaiserlik abbrevs, and common aliases.
func ResolveRef(ref string) (abbr, ch, vs string, ok bool) {
	m := refRe.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return "", "", "", false
	}
	name := normalizeBook(m[1])
	a, ok := bookAbbrev[name]
	if !ok {
		return "", "", "", false
	}
	// normalize numbers (strip leading zeros, reject non-numeric)
	cn, err1 := strconv.Atoi(m[2])
	vn, err2 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil || cn <= 0 || vn <= 0 {
		return "", "", "", false
	}
	return a, strconv.Itoa(cn), strconv.Itoa(vn), true
}

// normalizeBook lowercases, collapses whitespace, and trims trailing dots so
// "1 John", "1john", "1 John." all normalize alike.
func normalizeBook(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimRight(s, ". ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// bookAbbrev maps normalized book names, kaiserlik abbrevs, and common aliases
// to the kaiserlik abbrev used as the storage key.
var bookAbbrev = buildBookAbbrev()

func buildBookAbbrev() map[string]string {
	// canonical full name -> kaiserlik abbrev (from books.json)
	full := map[string]string{
		"genesis": "Gen", "exodus": "Exo", "leviticus": "Lev", "numbers": "Num",
		"deuteronomy": "Deu", "joshua": "Jos", "judges": "Jdg", "ruth": "Rth",
		"1 samuel": "1Sa", "2 samuel": "2Sa", "1 kings": "1Ki", "2 kings": "2Ki",
		"1 chronicles": "1Ch", "2 chronicles": "2Ch", "ezra": "Ezr",
		"nehemiah": "Neh", "esther": "Est", "job": "Job", "psalms": "Psa",
		"proverbs": "Pro", "ecclesiastes": "Ecc", "song of songs": "Sng",
		"isaiah": "Isa", "jeremiah": "Jer", "lamentations": "Lam",
		"ezekiel": "Eze", "daniel": "Dan", "hosea": "Hos", "joel": "Joe",
		"amos": "Amo", "obadiah": "Oba", "jonah": "Jon", "micah": "Mic",
		"nahum": "Nah", "habakkuk": "Hab", "zephaniah": "Zep", "haggai": "Hag",
		"zechariah": "Zec", "malachi": "Mal", "matthew": "Mat", "mark": "Mar",
		"luke": "Luk", "john": "Jhn", "acts": "Act", "romans": "Rom",
		"1 corinthians": "1Co", "2 corinthians": "2Co", "galatians": "Gal",
		"ephesians": "Eph", "philippians": "Phl", "colossians": "Col",
		"1 thessalonians": "1Th", "2 thessalonians": "2Th", "1 timothy": "1Ti",
		"2 timothy": "2Ti", "titus": "Tit", "philemon": "Phm", "hebrews": "Heb",
		"james": "Jas", "1 peter": "1Pe", "2 peter": "2Pe", "1 john": "1Jo",
		"2 john": "2Jo", "3 john": "3Jo", "jude": "Jde", "revelation": "Rev",
	}
	m := map[string]string{}
	for name, ab := range full {
		m[name] = ab
		m[strings.ToLower(ab)] = ab     // the kaiserlik abbrev itself
		m[strings.ReplaceAll(name, " ", "")] = ab // "1john" without space
	}
	// common aliases / alternate spellings people actually type
	aliases := map[string]string{
		"gen": "Gen", "ex": "Exo", "exod": "Exo", "lev": "Lev", "num": "Num",
		"deut": "Deu", "dt": "Deu", "josh": "Jos", "judg": "Jdg",
		"1 sam": "1Sa", "2 sam": "2Sa", "1sam": "1Sa", "2sam": "2Sa",
		"1 kgs": "1Ki", "2 kgs": "2Ki", "1kgs": "1Ki", "2kgs": "2Ki",
		"1 chron": "1Ch", "2 chron": "2Ch", "1chron": "1Ch", "2chron": "2Ch",
		"neh": "Neh", "esth": "Est", "ps": "Psa", "psa": "Psa", "psalm": "Psa",
		"prov": "Pro", "eccl": "Ecc", "qoheleth": "Ecc",
		"song": "Sng", "song of solomon": "Sng", "songofsolomon": "Sng",
		"canticles": "Sng", "ss": "Sng", "sos": "Sng",
		"isa": "Isa", "jer": "Jer", "lam": "Lam", "ezek": "Eze", "eze": "Eze",
		"dan": "Dan", "hos": "Hos", "obad": "Oba", "mic": "Mic", "zeph": "Zep",
		"zech": "Zec", "mal": "Mal",
		"matt": "Mat", "mt": "Mat", "mk": "Mar", "mrk": "Mar", "lk": "Luk",
		"jn": "Jhn", "joh": "Jhn", "rom": "Rom",
		"1 cor": "1Co", "2 cor": "2Co", "1cor": "1Co", "2cor": "2Co",
		"gal": "Gal", "eph": "Eph", "phil": "Phl", "php": "Phl", "phlp": "Phl",
		"col": "Col", "1 thess": "1Th", "2 thess": "2Th", "1thess": "1Th",
		"2thess": "2Th", "1 tim": "1Ti", "2 tim": "2Ti", "1tim": "1Ti",
		"2tim": "2Ti", "phlm": "Phm", "philem": "Phm", "heb": "Heb",
		"jas": "Jas", "jms": "Jas", "1 pet": "1Pe", "2 pet": "2Pe",
		"1pet": "1Pe", "2pet": "2Pe", "rev": "Rev", "revelations": "Rev",
		"apocalypse": "Rev",
	}
	for a, ab := range aliases {
		m[a] = ab
	}
	return m
}
