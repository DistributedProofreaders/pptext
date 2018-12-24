package util

import (
	"github.com/asylumcs/pptext/models"
	"strings"
	"unicode/utf8"
)

// compare the good word list to the submitted word
// allow variations, i.e. "Rose-Ann" in GWL will match "Rose-Ann’s"
func InGoodWordList(s string) bool {
	for _, word := range models.Gwl {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

func GetParaSeg(full_s string, where int) string {
	lsen := where - 30 // these are char positions, not runes
	rsen := where + 30 // left and right sentinels
	if lsen < 0 {
		rsen -= lsen // effectively entends right if left underflowed
		lsen = 0
	}
	if rsen >= len(full_s) {
		lsen -= (rsen - len(full_s)) // extend left if right overflowed
		rsen = len(full_s) - 1
	}
	// assure in string
	if lsen < 0 {
		lsen = 0
	}
	if rsen > len(full_s)-1 {
		rsen = len(full_s) - 1
	}
	// now find sentinels at rune boundaries
	if !utf8.RuneStart(full_s[lsen]) {
		for ok := true; ok; ok = !utf8.RuneStart(full_s[lsen]) {
			lsen++
		}
	}
	if !utf8.RuneStart(full_s[rsen]) {
		for ok := true; ok; ok = !utf8.RuneStart(full_s[rsen]) {
			rsen--
		}
	}
	// full_s[lsen:rsen] is a valid string on rune boundaries
	// approximatly 60 bytes (not characters) long
	flushLeft := (lsen == 0)
	flushRight := (rsen == len(full_s)-1)
	s := full_s[lsen:rsen]

	// chop off leading runes until on a space if one is available
	// unless at the start of the string already
	if !flushLeft && strings.ContainsRune(s, ' ') {
		for len(s) > 0 {
			runeValue, width := utf8.DecodeRuneInString(s)
			if runeValue == ' ' {
				break
			}
			s = s[width:]
		}
	}

	// chop off trailing runes until on a space if one is available
	// unless at the end of the string already
	if !flushRight && strings.ContainsRune(s, ' ') {
		for len(s) > 0 {
			runeValue, width := utf8.DecodeLastRuneInString(s)
			if runeValue == ' ' {
				break
			}
			s = s[:len(s)-width]
		}
	}

	s = strings.TrimSpace(s)
	return s
}

/* NOTUSED
    // convert paragraph to slice of {runes, positions}
	type rp struct {
		rpr rune
		rpp int
	}
	rps := make([]rp, 0)
	for i, w := 0, 0; i < len(para); i += w {
        runeValue, width := utf8.DecodeRuneInString(para[i:])
        // fmt.Printf("%#U starts at byte position %d\n", runeValue, i)
        rps = append(rps, rp{rpr:runeValue, rpp: i})
        w = width
    }
*/

func PuncStyle() string {

	// decide is this is American or British punctuation
	cbrit, camer := 0, 0
	for _, line := range models.Wb {
		if strings.Contains(line, ".’") {
			cbrit += 1
		}
		if strings.Contains(line, ".”") {
			camer += 1
		}
	}
	if cbrit > camer {
		return "British"
	} else {
		return "American"
	}
}
