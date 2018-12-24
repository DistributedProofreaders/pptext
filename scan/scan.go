package scan

import (
	"fmt"
	"github.com/asylumcs/pptext/models"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var lpb []string // local paragraph buffer
var pnm []string // proper names in this text
var rs []string  // to append to the overall runlog for all tests

func report(r string) {
	rs = append(rs, r)
}

func straightCurly() bool {
	curly := false
	straight := false
	for _, line := range lpb {
		if strings.ContainsAny(line, "\"'") {
			straight = true
		}
		if strings.ContainsAny(line, "“”‘’") {
			curly = true
		}
		if straight && curly {
			break
		}
	}
	return straight && curly
}

// logic here is to strip off the last two characters if they are s’
// if what's left is a dictionary word (l/c), then conclude it is plural possessive
// for safety, do not do this if there is an open single quote on this line.
// https://github.com/google/re2/wiki/Syntax
// "horses’" becomes "horses◳" and the apostrophe drops out of the scan

func pluralPossessive() {
	for n, line := range lpb {
		if strings.Contains(line, "‘") {
			continue // possible close quote. don't check plural-poss.
		}
		re := regexp.MustCompile(`\p{L}+(s’)`)
		words := re.FindAllString(line, -1)
		for _, w := range words {
			testword := strings.TrimSuffix(w, "s’")
			testwordlc := strings.ToLower(testword)
			ip := sort.SearchStrings(models.Wd, testwordlc)          // where it would insert
			if ip != len(models.Wd) && models.Wd[ip] == testwordlc { // true if we found it
				lpb[n] = strings.Replace(lpb[n], w, testword+"s◳", -1)
			}
		}
	}
}

// "goin’" -> strip last => "goin" not a word; add 'g' => "going"
// therefore consider this as a contraction
func ingResolve() {
	for n, line := range lpb {
		re := regexp.MustCompile(`\p{L}+(in’)`)
		words := re.FindAllString(line, -1)
		for _, w := range words {
			strippedword := strings.TrimSuffix(w, "’")
			strippedword_lc := strings.ToLower(strippedword)
			augmentedword_lc := strippedword_lc + "g"
			strippedisword := false
			augmentedisword := false
			ip := sort.SearchStrings(models.Wd, strippedword_lc)
			if ip != len(models.Wd) && models.Wd[ip] == strippedword_lc {
				strippedisword = true
			}
			ip = sort.SearchStrings(models.Wd, augmentedword_lc)
			if ip != len(models.Wd) && models.Wd[ip] == augmentedword_lc {
				augmentedisword = true
			}
			if augmentedisword && !strippedisword {
				// the word is a contraction
				lpb[n] = strings.Replace(lpb[n], w, strippedword+"◳", -1)
			}
		}
	}
}

func inDict(word string) bool {
	ip := sort.SearchStrings(models.Wd, word)
	return ip != len(models.Wd) && models.Wd[ip] == word
}

// proper names
// find proper names in this text. protect possessive
// any capitalized word in good word list is considered a proper name
//
func properNames() {
	re := regexp.MustCompile(`^\p{Lu}\p{Ll}`) // uppercase followed by lower case
	for _, word := range models.Gwl {
		if re.MatchString(word) {
			pnm = append(pnm, word)
		}
	}
	for word, freq := range models.Wlm {
		if re.MatchString(word) {
			// if this title-case word occurs frequently
			// and isn't a contraction and is not in the dictionary,
			// then consider it a proper name
			if strings.Contains(word, "’") {
				continue
			}
			if freq > 4 && !inDict(strings.ToLower(word)) {
				pnm = append(pnm, word)
			}
		}
	}
	// have proper names in pnm. protect them in text
	pnmr := make([]string, len(pnm))
	for n, word := range pnm {
		if strings.HasSuffix(word, "s") {
			pnm[n] = word + "’"
			pnmr[n] = word + "◳"
		} else {
			pnm[n] = word + "’s"
			pnmr[n] = word + "◳s"
		}
	}
	for n, _ := range lpb {
		for j, _ := range pnm {
			lpb[n] = strings.Replace(lpb[n], pnm[j], pnmr[j], -1)
		}
	}
}

// single words in single quotes are protected
// two words in single quotes are protected (TODO: if they are both valid words)
func wordQuotes() {
	for n, _ := range lpb {
		re := regexp.MustCompile(`‘(\S+)’`)
		lpb[n] = re.ReplaceAllString(lpb[n], `◰$1◳`)
		re = regexp.MustCompile(`‘(\S+) (\S+)’`)
		// verify $1 and $2 are valid words here
		lpb[n] = re.ReplaceAllString(lpb[n], `◰$1 $2◳`)
	}
}

func internals() {
	for n, _ := range lpb {
		re := regexp.MustCompile(`(\p{L})’(\p{L})`)
		lpb[n] = re.ReplaceAllString(lpb[n], `$1◳$2`)
	}
}

func commonforms() {
	// common forms that probably are apostrophes and not single quotes
	// are converted. Ideally, all remaining '‘' and '’' are quotes to be
	// scanned later.
	// limit common forms to those that do not become a word when the
	// apostrophe is removed.

	commons_lead := []string{
		"’em", "’a’", "’n’", "’twill", "’twon’t", "’twas", "’tain’t", "’taint", "’twouldn’t",
		"’twasn’t", "’twere", "’twould", "’tis", "’twarn’t", "’tisn’t", "’twixt", "’till",
		"’bout", "’casion", "’shamed", "’lowance", "’n", "’s", "’d", "’m", "’ave",
		"’cordingly", "’baccy", "’cept", "’stead", "’spose", "’chute", "’im",
		"’u’d", "’tend", "’rickshaw", "’appen", "’oo", "’urt", "’ud", "’ope", "’ow",
		"’specially",
		// higher risk follows
		"’most", "’cause", "’way"}

	commons_tail := []string{
		"especial’", "o’", "ol’", "tha’", "canna’", "an’", "d’",
		"G’-by", "ha’", "tak’", "th’", "i’", "wi’", "yo’", "ver’", "don’", "jes’",
		"aroun’", "wan’", "M◳sieu’", "nuthin’"}

	commons_both := []string{"’cordin’"}

	for n, _ := range lpb {
		for _, clead := range commons_lead {
			c3 := strings.Replace(clead, "’", "", -1)
			re := regexp.MustCompile(fmt.Sprintf(`(?i)’(%s)(\A|[^\p{L}])`, c3))
			lpb[n] = re.ReplaceAllString(lpb[n], fmt.Sprintf(`◳$1$2`))
		}
		for _, clead := range commons_tail {
			c3 := strings.Replace(clead, "’", "", -1)
			re := regexp.MustCompile(fmt.Sprintf(`(\A|[\P{L}])(%s)’`, c3))
			lpb[n] = re.ReplaceAllString(lpb[n], `$1$2◳`)
		}
		for _, clead := range commons_both {
			c3 := strings.Replace(clead, "’", "", -1)
			re := regexp.MustCompile(fmt.Sprintf(`’(%s)’`, c3))
			lpb[n] = re.ReplaceAllString(lpb[n], `◳$1◳`)
		}
	}
}

func fr(line string, cpos int) (int, rune) { // go forward one rune
	r, size := utf8.DecodeRuneInString(line[cpos:])
	cpos += size
	return cpos, r
}

func br(line string, cpos int) (int, rune) { // go backwards one rune
	r, size := utf8.DecodeLastRuneInString(line[:cpos])
	cpos -= size
	return cpos, r
}

func cr(line string, cpos int) rune { // current rune
	r, _ := utf8.DecodeRuneInString(line[cpos:])
	return r
}

/*
// test code for moving through buffer
func parawalk() {
	var cpos int // current position on line (byte offset)
	var r rune   // rune at this position
	for _, line := range models.Pb {
		cpos = 0
		for cpos < len(line) {
			cpos, r = Fr(line, cpos)
			fmt.Println(cpos, string(r))
		}
		fmt.Println("------------------")

		for cpos > 0 {
			cpos, r = Br(line, cpos)
			fmt.Println(cpos, string(r))
		}

		fmt.Printf("%s", string(Cr(line, cpos)))

		fmt.Println("BEFORE", utf8.RuneCountInString(line))

		line = strings.Replace(line, "“", "A", -1)
		line = strings.Replace(line, "”", "B", -1)

		cpos = 0
		for cpos < len(line) {
			cpos, r = Fr(line, cpos)
			fmt.Println(cpos, string(r))
		}
		fmt.Println("AFTER", utf8.RuneCountInString(line))

	}
}
*/

// limited to “, ”, ‘, ’
// doesn't handle «, ‟, ‹, ⸌, ⸜, ⸠ as in etext 58423

func doScan() {
	nreports := 0
	// scan each paragraph
	for n, p := range lpb {
		stk := ""                  // new stack for each line
		cpos := 0                  // current rune position
		query := make([]string, 0) // query list per paragraph
		for cpos < len(p) {        // go across the line for double, single quotes
			r, size := utf8.DecodeRuneInString(p[cpos:])

			if r == '“' {
				// open double quote. check for consecutive open DQ
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '“' {
					query = append(query, "[CODQ] consec open double quote")
				}
				stk += string('“')
			}
			if r == '”' {
				// close double quote. check for consecutive close DQ or unmatched
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '”' {
					query = append(query, "[CCDQ] consec close double quote:")
				}
				if r2 != '“' {
					query = append(query, "[UCDQ] unmatched close double quote:")
				}
				stk = strings.TrimSuffix(stk, "“")
			}

			if r == '‘' {
				// open single quote. check for consecutive open SQ
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '‘' {
					query = append(query, "[COSQ] consec open single quote")
				}
				stk += string('‘')
			}
			if r == '’' {
				// close single quote.
				// if there is an open single quote, pair it
				// else ignore this one.
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '‘' {
					stk = strings.TrimSuffix(stk, "‘")
				}

			}
			cpos += size
		}

		// if the stack has a single “ remaining,
		// that is considered okay if the next paragraph starts with another “
		// more rarely (as in etext 58432), the combo '“‘' remains, which
		// will trigger a false positive
		if len(stk) > 0 {
			cpara := false
			if stk == "“" {
				// check for continued paragraph. look ahead if possible
				if n < len(lpb)-2 {
					// allows for indented block of text
					r3, _ := utf8.DecodeRuneInString(strings.TrimSpace(lpb[n+1]))
					cpara = (r3 == '“')
				}
			}
			if !cpara {
				query = append(query, fmt.Sprintf("[UCPA] unclosed paragraph"))
			}
		}
		// save any query/reports, one per paragraph
		if len(query) > 0 {
			p = strings.Replace(p, "◳", "’", -1)
			p = strings.Replace(p, "◰", "‘", -1)

			// text wrap of paragraph p into s2, with inserted newlines
			s2 := ""
			runecount := 0
			// rc := utf8.RuneCountInString(s)
			for len(p) > 0 {
				r, size := utf8.DecodeRuneInString(p)
				runecount++
				if runecount >= 70 && r == ' ' {
					r = '\n'
					runecount = 0
				}
				s2 += string(r)
				p = p[size:]
			}
			
			report(fmt.Sprintf("%s\n  %s\n", query[0], s2))

			
			// report(fmt.Sprintf("%s\n  %s\n", query[0], p))
			nreports++
		}
	}
	if nreports == 0 {
		report("  no punctuation scan queries reported")
	}
}

func Scan() {
	report(fmt.Sprintf("\n********************************************************************************"))
	report(fmt.Sprintf("* %-76s *", "PUNCTUATION SCAN REPORT"))
	report(fmt.Sprintf("********************************************************************************"))

	// get a copy of the paragraph buffer to edit in place
	lpb = make([]string, len(models.Pb))
	copy(lpb, models.Pb)
	if straightCurly() { // check for mixed style
		report("  mixed straight and curly quotes. punctuation scan not done")
		models.Report = append(models.Report, rs...)
		return
	}
	pluralPossessive() // horses’ becomes horses◳
	internals()        // protect internal '’' characters as in contractions
	commonforms()      // protect known common forms
	ingResolve()       // resolve -ing words that are contracted
	wordQuotes()       // apparent quoted words or two-word phrases
	properNames()      // protect proper names
	doScan()           // scan quotes and report

	models.Report = append(models.Report, rs...)

}
