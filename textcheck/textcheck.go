package textcheck

import (
	"fmt"
	"github.com/asylumcs/pptext/models"
	"github.com/asylumcs/pptext/util"
	"github.com/asylumcs/pptext/wfreq"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var rs []string // to append to the overall runlog for all tests

func report(r string) {
	rs = append(rs, r)
}

// curly quote check (positional, not using a state machine)
func curlyQuoteCheck(wb []string) {
	report("\n----- curly quote check ------------------------------------------------------\n")
	count := 0
	ast := 0

	r0a := regexp.MustCompile(` [“”] `)
	r0b := regexp.MustCompile(`^[“”] `)
	r0c := regexp.MustCompile(` [“”]$`)

	for n, line := range wb {
		if r0a.MatchString(line) || r0b.MatchString(line) || r0c.MatchString(line) {
			if ast == 0 {
				report(fmt.Sprintf("    %s", "floating quote"))
				ast++
			}
			report(fmt.Sprintf("    %5d: %s", n, line))
			count++
		}
	}

	ast = 0
	r1a := regexp.MustCompile(`[\.,;!?]+[‘“]`)
	r1b := regexp.MustCompile(`[A-Za-z]+[‘“]`)
	for n, line := range wb {
		if r1a.MatchString(line) || r1b.MatchString(line) {
			if ast == 0 {
				report(fmt.Sprintf("    %s", "quote direction"))
				ast++
			}
			report(fmt.Sprintf("    %5d: %s", n, line))
			count++
		}
	}

	if count == 0 {
		report("  no curly quote suspects found in text.")
	}
}

// scanno check
// iterate each scanno word from pptext.dat
// look for that word on each line of the book
// scannos list in pptext.dat
//   from https://www.pgdp.net/c/faq/stealth_scannos_eng_common.txt
func scannoCheck(wb []string) {
	report("\n----- scanno check -----------------------------------------------------------\n")
	count := 0
	for _, scannoword := range models.Swl { // each scanno candidate
		ast := 0
		for n, linewords := range models.Lwl { // slice of slices of words per line
			for _, word := range linewords { // each word on line
				if word == scannoword {
					if ast == 0 {
						report(fmt.Sprintf("%s", word))
						ast++
					}
					report(fmt.Sprintf("  %5d: %s", n, wb[n]))
					count++
				}
			}
		}
	}
	if count == 0 {
		report("  no suspected scannos found in text.")
	}
}

// dash check
func dashCheck(pb []string) {
	report("\n----- dash check -------------------------------------------------------------\n")
	re := regexp.MustCompile(`(— )|( —)|(—-)|(-—)|(- -)|(— -)|(- —)`)
	count := 0
	for _, para := range pb {
		if re.MatchString(para) {
			u := re.FindStringIndex(para)
			if u != nil {
				lsen := u[0] // these are character positions, not runes
				rsen := u[1]
				lsen -= 20
				rsen += 20
				if lsen < 0 {
					lsen = 0
				}
				if rsen >= len(para) {
					rsen = len(para) - 1
				}
				// now find sentinels at rune boundaries
				for ok := true; ok; ok = !utf8.RuneStart(para[lsen]) {
					lsen++
				}
				for ok := true; ok; ok = !utf8.RuneStart(para[rsen]) {
					rsen--
				}
				// para[lsen:rsen] is a valid string
				s := para[lsen:rsen]
				// trim it to first, last space
				ltrim := strings.Index(s, " ")
				rtrim := strings.LastIndex(s, " ")
				report(fmt.Sprintf("    %s", s[ltrim:rtrim]))
				count++
			}

		}
	}
	if count == 0 {
		report("  no dash check suspects found in text.")
	}
}

// ellipsis checks
func ellipsisCheck(wb []string) {
	report("\n----- ellipsis check ---------------------------------------------------------\n")

	re1 := regexp.MustCompile(`[^.]\.\.\. `)   // give... us some pudding
	re2 := regexp.MustCompile(`\.\.\.\.[^\s]`) // give....us some pudding
	re3 := regexp.MustCompile(`[^.]\.\.[^.]`)  // give .. us some pudding
	re4 := regexp.MustCompile(`\.\.\.\.\.+`)   // give.....us some pudding
	re5 := regexp.MustCompile(`^\.`)           // ... us some pudding (start of line)
	re6 := regexp.MustCompile(`\.\.\.$`)       // give ... (end of line)

	count := 0
	for n, line := range wb {
		if re1.MatchString(line) ||
			re2.MatchString(line) ||
			re3.MatchString(line) ||
			re4.MatchString(line) ||
			re5.MatchString(line) ||
			re6.MatchString(line) {
			report(fmt.Sprintf("  %5d: %s", n, line))
			count++
		}
	}
	if count == 0 {
		report("  no ellipsis suspects found in text.")
	}
}

// repeated word check
// works against paragraph buffer
func repeatedWords(pb []string) {
	report("\n----- repeated word check ----------------------------------------------------\n")
	count := 0
	for _, line := range pb {
		t := wfreq.GetWordsOnLine(line)
		for n, _ := range t {
			if n < len(t)-1 && t[n] == t[n+1] {
				// find the repeated word on the line
				cs := fmt.Sprintf("%s %s", t[n], t[n]) // only care if space-separated
				re := regexp.MustCompile(cs)
				u := re.FindStringIndex(line)
				if u != nil {
					lsen := u[0] // these are character positions, not runes
					rsen := u[1]
					lsen -= 20
					rsen += 20
					if lsen < 0 {
						lsen = 0
					}
					if rsen >= len(line) {
						rsen = len(line) - 1
					}
					// now find sentinels at rune boundaries
					for ok := true; ok; ok = !utf8.RuneStart(line[lsen]) {
						lsen++
					}
					for ok := true; ok; ok = !utf8.RuneStart(line[rsen]) {
						rsen--
					}
					// line[lsen:rsen] is a valid string
					s := line[lsen:rsen]
					// trim it to first, last space
					ltrim := strings.Index(s, " ")
					rtrim := strings.LastIndex(s, " ")
					report(fmt.Sprintf("    [%s] %s", t[n], s[ltrim:rtrim]))
					count++
				}
			}
		}
	}
	if count == 0 {
		report("  no repeated words found in text.")
	}
}

// definitions from Project Gutenberg
const (
	SHORTEST_PG_LINE = 55
	LONGEST_PG_LINE  = 75
	WAY_TOO_LONG     = 80
)

// short line:
// this line has no leading space, has some text, length of line less than
// 55 characters, following line has some text.
// all lengths count runes
func shortLines(wb []string) {
	report("\n----- short lines check ------------------------------------------------------\n")
	count := 0
	for n, line := range wb {
		if n == len(wb)-1 {
			break // do not check last line
		}
		if !strings.HasPrefix(line, " ") &&
			utf8.RuneCountInString(line) > 0 &&
			utf8.RuneCountInString(line) <= SHORTEST_PG_LINE &&
			utf8.RuneCountInString(wb[n+1]) > 0 {
				if count <= 10 {
					report(fmt.Sprintf("  %5d: %s", n, line))
				}
				count++
		}
	}
	if count > 5 {
		report(fmt.Sprintf("  ....%5d more.", count - 5))
	}
	if count == 0 {
		report("  no short lines found in text.")
	}
}

// all lengths count runes
func longLines(wb []string) {
	report("\n----- long lines check ------------------------------------------------------\n")
	count := 0
	for n, line := range wb {
		if utf8.RuneCountInString(line) > 72 {
			t := line[60:]
			where := strings.Index(t, " ") // first space after byte 60 (s/b rune-based?)
			report(fmt.Sprintf("  %5d: [%d] %s...", n, utf8.RuneCountInString(line), line[:60+where]))
			count++
		}
	}
	if count == 0 {
		report("  no long lines found in text.")
	}
}

func asteriskCheck(wb []string) {
	report("\n----- asterisk checks --------------------------------------------------------\n")
	count := 0
	for n, line := range wb {
		if strings.Contains(line, "*") {
			report(fmt.Sprintf("  %5d: %s", n, line))
			count += 1
		}
	}
	if count == 0 {
		report("  no unexpected asterisks found in text.")
	}
}

// do not report adjacent spaces that start or end a line
func adjacentSpaces(wb []string) {
	report("\n----- adjacent spaces check --------------------------------------------------\n")
	count := 0
	for n, line := range wb {
		if strings.Contains(strings.TrimSpace(line), "  ") {
			if count < 10 {
				report(fmt.Sprintf("  %5d: %s", n, line))
			}
			if count == 10 {
				report(fmt.Sprintf("    ...more"))	
			}
			count += 1
		}
	}
	if count == 0 {
		report("  no adjacent spaces found in text.")
	}
}

//
func trailingSpaces(wb []string) {
	report("\n----- trailing spaces check ---------------------------------------------------\n")
	count := 0
	for n, line := range wb {
		if strings.TrimSuffix(line, " ") != line {
			report(fmt.Sprintf("  %5d: %s", n, line))
			count += 1
		}
	}
	if count == 0 {
		report("  no trailing spaces found in text.")
	}
}

type kv struct {
	Key   rune
	Value int
}

var m = map[rune]int{} // a map for letter frequency counts

// report infrequently-occuring characters (runes)
// threshold set to fewer than 10 occurences or fewer than #lines / 100
// do not report numbers
func letterChecks(wb []string) {
	report("\n----- character checks --------------------------------------------------------\n")
	count := 0
	for _, line := range wb {
		for _, char := range line { // this gets runes
			m[char] += 1
		}
	}
	var ss []kv           // slice of Key, Value pairs
	for k, v := range m { // load it up
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool { // sort it based on Value
		return ss[i].Value > ss[j].Value
	})
	b := []int{10, int(len(wb) / 25)}
	sort.Ints(b)
	kvthres := b[0]
	for _, kv := range ss {
		reportme := false
		if kv.Value < kvthres && (kv.Key < '0' || kv.Key > '9') {
			reportme = true
		}
		if !strings.ContainsRune(",:;—?!-_0123456789“‘’”. abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", kv.Key) {
			reportme = true
		}
		if reportme {
			reportcount := 0
			report(fmt.Sprintf("%s", strconv.QuoteRune(kv.Key)))
			// report(fmt.Sprintf("%s", kv.Key))
			count += 1
			count += 1
			for n, line := range wb {
				if strings.ContainsRune(line, kv.Key) {
					if reportcount < 10 {
						report(fmt.Sprintf("  %5d: %s", n, line))
					}
					if reportcount == 10 {
						report(fmt.Sprintf("    ...more"))
					}
					reportcount++
				}
			}
		}
	}
	if count == 0 {
		report("  no character checks reported.")
	}
}

// spacing check
// any spacing is okay until the first 4-space gap. Then
// expecting 4-1-2 or 4-2 variations only
func spacingCheck(wb []string) {
	s := ""
	count := 0
	report("\n----- spacing pattern ---------------------------------------------------------\n")

	consec := 0                        // consecutive blank lines
	re := regexp.MustCompile("11+1")   // many paragraphs separated by one line show as 1..1
	re3 := regexp.MustCompile("3")     // flag unexpected vertical size breaks of 3
	re5 := regexp.MustCompile("5")     // or 5 spaces
	re411 := regexp.MustCompile("411") // a sequence of four spaces shoud be 412 so flag 411
	pbuf := ""
	for _, line := range wb {
		if line == "" {
			consec++
		} else { // a non-blank line
			if consec >= 4 { // start of a new chapter
				s = re411.ReplaceAllString(s, "[411]")
				s = re.ReplaceAllString(s, "1..1")
				s = re3.ReplaceAllString(s, "[3]")
				s = re5.ReplaceAllString(s, "[5]")
				if len(pbuf)+len(s) > 64 {
					// it would overflow, so print what I have and
					// start a new line
					if strings.ContainsAny(pbuf, "[]") {
						pbuf += " <<"
						count++
					}
					report(pbuf)
					pbuf = s
				} else {
					// add it to print buffer
					pbuf = fmt.Sprintf("%s %s", pbuf, s)
				}
				s = "4"
			} else {
				if consec > 0 {
					s = fmt.Sprintf("%s%d", s, consec)
				}
			}
			consec = 0
		}
	}
	s = pbuf + " " + s
	s = re411.ReplaceAllString(s, "[411]")
	s = re.ReplaceAllString(s, "1..1")
	s = re3.ReplaceAllString(s, "[3]")
	s = re5.ReplaceAllString(s, "[5]")
	if strings.ContainsAny(s, "[]") {
		s += " <<" // show any line (chapter) with unexpected spacing
		count++
	}
	report(s)
	if count > 0 {
		report("  spacing anomalies reported.")
	} else {
		report("  no spacing anomalies reported.")
	}
}

// book-level checks
func bookLevel(wb []string) {
	count := 0
	report("\n----- book level checks -----------------------------------------------\n")

	// check: straight and curly quotes mixed
	if m['\''] > 0 && (m['‘'] > 0 || m['’'] > 0) {
		report("  both straight and curly single quotes found in text")
		count++
	}

	if m['"'] > 0 && (m['“'] > 0 || m['”'] > 0) {
		report("  both straight and curly double quotes found in text")
		count++
	}

	// check to-day and today mixed
	ctoday, ctohday, ctonight, ctohnight := 0, 0, 0, 0
	re01 := regexp.MustCompile(`(?i)today`)
	re02 := regexp.MustCompile(`(?i)to-day`)
	re03 := regexp.MustCompile(`(?i)tonight`)
	re04 := regexp.MustCompile(`(?i)to-night`)
	for _, line := range wb {
		ctoday += len(re01.FindAllString(line, -1))
		ctohday += len(re02.FindAllString(line, -1))
		ctonight += len(re03.FindAllString(line, -1))
		ctohnight += len(re04.FindAllString(line, -1))
	}
	if ctoday > 0 && ctohday > 0 {
		report("  both \"today\" and \"to-day\" found in text")
		count++
	}
	if ctonight > 0 && ctohnight > 0 {
		report("  both \"tonight\" and \"to-night\" found in text")
		count++
	}

	// check: American and British title punctuation mixed
	mrpc, mrc, mrspc, mrsc, drpc, drc := false, false, false, false, false, false
	re1 := regexp.MustCompile(`Mr.`)
	re2 := regexp.MustCompile(`Mr\s`)
	re3 := regexp.MustCompile(`Mrs.`)
	re4 := regexp.MustCompile(`Mrs\s`)
	re5 := regexp.MustCompile(`Dr.`)
	re6 := regexp.MustCompile(`Dr\s`)
	for _, para := range wb {
		if !mrpc {
			mrpc = re1.MatchString(para)
		}
		if !mrc {
			mrc = re2.MatchString(para)
		}
		if !mrspc {
			mrspc = re3.MatchString(para)
		}
		if !mrsc {
			mrsc = re4.MatchString(para)
		}
		if !drpc {
			drpc = re5.MatchString(para)
		}
		if !drc {
			drc = re6.MatchString(para)
		}
	}
	if mrpc && mrc {
		report("  both \"Mr.\" and \"Mr\" found in text")
		count++
	}
	if mrspc && mrsc {
		report("  both \"Mrs.\" and \"Mrs\" found in text")
		count++
	}
	if drpc && drc {
		report("  both \"Dr.\" and \"Dr\" found in text")
		count++
	}

	// apostrophes and turned commas
	countm1, countm2 := 0, 0
	for n, _ := range wb {
		// check each word separately on this line
		for _, word := range models.Lwl[n] {
			if strings.Contains(word, "M’") {
				countm1++ // with apostrophe
			}
			if strings.Contains(word, "M‘") {
				countm2++ // with turned comma
			}
		}
	}
	if countm1 > 0 && countm2 > 0 {
		report("  both apostrophes and turned commas appear in text")
	}

	// summary
	if count == 0 {
		report("  no book level checks reported.")
	}
}

// paragraph-level checks
func paraLevel() {
	count := 0
	const RLIMIT int = 5

	report("\n----- paragraph level checks -----------------------------------------------\n")

	// ------------------------------------------------------------------------
	// check: paragraph starts with upper-case word

	re := regexp.MustCompile(`^[“”]?[A-Z][A-Z].*?[a-z]`)
	sscnt := 0

	for _, para := range models.Pb { // paragraph buffer
		if re.MatchString(para) {
			if sscnt == 0 {
				report("  paragraph starts with upper-case word")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, 0))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: full stop (period) with the following word starting with
	// a lower case character.
	// allow exceptions

	re = regexp.MustCompile(`(\p{L}+)\.\s*?[a-z]`)
	exc := []string{"Mr.", "Mrs.", "Dr."}
	sscnt = 0
	for _, para := range models.Pb {
		loc := re.FindAllStringIndex(para, -1)
		doreport := true
		// go across this paragraph examining each match
		for _, lmatch := range loc {
			// if any of the matches are not forgiven by exception, report paragraph segment
			for _, ts := range exc {
				if strings.Contains(para[lmatch[0]:lmatch[1]], ts) {
					doreport = false
				}
			}
			if doreport {
				if sscnt == 0 {
					report("  full stop followed by lower case letter")
					count++
				}
				sscnt++
				if sscnt < RLIMIT || models.P.Verbose {
					report("    " + util.GetParaSeg(para, lmatch[0]))
				}
			}
		}

	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: query missing paragraph break

	re = regexp.MustCompile(`”\s*“`)
	sscnt = 0
	for _, para := range models.Pb {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				report("  query: missing paragraph break?")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: incorrectly split paragraph

	re = regexp.MustCompile(`^[a-z]`)
	sscnt = 0
	for _, para := range models.Pb {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				report("  incorrectly split paragraph")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: unconverted double-dash/em-dash, long dash error,
	//        or double-emdash broken at end of line
	re = regexp.MustCompile(`(\w--\w)|(\w--)|(\w— —)|(\w- -)|(--\w)|(—— )|(— )|( —)|(———)`)
	sscnt = 0
	for _, para := range models.Pb {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				report("  dash error")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: common he/be, hut/but and had/bad checks
	//        jeebies is run separately with different algorithm (three word forms)

	const (
		HADBADPATTERN = `\bi bad\b|\byou bad\b|\bhe bad\b|\bshe bad\b|\bthey bad\b|\ba had\b|\bthe had\b`
		HUTBUTPATTERN = `(, hut)|(; hut)`
		HEBEPATTERN   = `\bto he\b|\bis be\b|\bbe is\b|\bwas be\b|\bbe would\b|\bbe could\b`
	)

	re_hebe := regexp.MustCompile(HEBEPATTERN)
	sscnt = 0
	for _, para := range models.Pb {
		lpara := strings.ToLower(para)
		loc := re_hebe.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				report("  query: he/be. (see also jeebies report)")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	re_hadbad := regexp.MustCompile(HADBADPATTERN)
	sscnt = 0
	for _, para := range models.Pb {
		lpara := strings.ToLower(para)
		loc := re_hadbad.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				report("  query: had/bad")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	re_hutbut := regexp.MustCompile(HUTBUTPATTERN)
	sscnt = 0
	for _, para := range models.Pb {
		lpara := strings.ToLower(para)
		loc := re_hutbut.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				report("  query: hut/but")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    " + util.GetParaSeg(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: paragraph endings, punctuation-style aware

	const (
		LEGALAMERICAN = `\.$|:$|\?$|!$|—$|.”|\?”$|’”$|!”$|—”|-----` // include thought-break line all '-'
		LEGALBRITISH  = `\.$|:$|\?$|!$|—$|.’|\?’$|”’$|!’$|—’|-----`
	)

	re_end := regexp.MustCompile(LEGALAMERICAN)
	sscnt = 0
	if models.PuncStyle == "British" {
		re_end = regexp.MustCompile(LEGALBRITISH)
	}

	for _, para := range models.Pb {
		if strings.HasPrefix(para, " ") {
			continue // only a normal paragraph
		}
		if !re_end.MatchString(para) {
			if sscnt == 0 {
				report("  query: unexpected paragraph end")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || models.P.Verbose {
				report("    ..." + util.GetParaSeg(para, -1))  // show paragraph end
			}
		}
	}
	if sscnt > RLIMIT && !models.P.Verbose {
		report(fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------

	// ------------------------------------------------------------------------

	// summary
	if count == 0 {
		report("  no paragraph level checks reported.")
	}
}

// tests extracted from gutcheck that aren't already included
func gutChecks(wb []string) {
	report("\n----- special situations checks -----------------------------------------------\n")

	re0000 := regexp.MustCompile(`\[[^IG\d]`)                                 // allow Illustration, Greek, or number
	re0001 := regexp.MustCompile(`(?i)\bthe[\.\,\?\'\"\;\:\!\@\#\$\^\&\(\)]`) // punctuation after "the"
	re0002 := regexp.MustCompile(`(,\.)|(\.,)|(,,)|([^\.]\.\.[^\.])`)         // double punctuation
	re0003a := regexp.MustCompile(`[a-z]`)                                    // for mixed case check
	re0003b := regexp.MustCompile(`[A-Z]`)
	re0003c := regexp.MustCompile(`cb|gb|pb|sb|tb|wh|fr|br|qu|tw|gl|fl|sw|gr|sl|cl|iy`) // rare to end word
	re0003d := regexp.MustCompile(`hr|hl|cb|sb|tb|wb|tl|tn|rn|lt|tj`)                   // rare to start word
	re0004 := regexp.MustCompile(`([A-Z])\.([A-Z])`)                                    // initials without space
	re0006 := regexp.MustCompile(`^.$`)                                                 // single character line
	re0007 := regexp.MustCompile(`([A-Za-z]\- [A-Za-z])|([A-Za-z] \-[A-Za-z])`)         // broken hyphenation

	// comma spacing regex
	re0008a := regexp.MustCompile(`[a-zA-Z_],[a-zA-Z_]`) // the,horse
	re0008b := regexp.MustCompile(`[a-zA-Z_],\d`)        // the,1
	re0008c := regexp.MustCompile(`\s,`)                 // space comma
	re0008d := regexp.MustCompile(`^,`)                  // comma start of line

	// full stop spacing regex
	re0009a := regexp.MustCompile(`\.[a-zA-Z]`)
	re0009b := regexp.MustCompile(`[^(Mr)(Mrs)(Dr)]\.\s[a-z]`)

	re0010 := regexp.MustCompile(`,1\d\d\d`)                                                            // Oct. 8,1948 date format
	re0011 := regexp.MustCompile(`I”`)                                                                  // “You should runI”
	re0012 := regexp.MustCompile(`\s’(m|ve|ll|t)\b`)                                                    // I' ve disjointed contraction
	re0013 := regexp.MustCompile(`Mr,|Mrs,|Dr,`)                                                        // title abbrev.
	re0014 := regexp.MustCompile(`\s[\?!:;]`)                                                           // spaced punctuation
	re0016 := regexp.MustCompile(`<\/?.*?>`)                                                            // HTML tag
	re0017 := regexp.MustCompile(`([^\.]\.\.\. )|(\.\.\.\.[^\s])|([^\.]\.\.[^\.])|(\.\.\.\.\.+)`)       // ellipsis
	re0018 := regexp.MustCompile(`([\.,;!?’‘]+[‘“])|([A-Za-z]+[“])|([A-LN-Za-z]+[‘])|(“ )|( ”)|(‘s\s)`) // quote direction (context)
	re0019 := regexp.MustCompile(`\b0\b`)                                                               // standalone 0
	re0020a := regexp.MustCompile(`\b1\b`)                                                              // standalone 1
	re0020b := regexp.MustCompile(`\$1\b`)                                                              // standalone 1 allowed after dollar sign
	re0021 := regexp.MustCompile(`([A-Za-z]\d)|(\d[A-Za-z])`)                                           // mixed alpha and numerals
	re0022 := regexp.MustCompile(`\s$`)                                                                 // trailing spaces/whitespace on line
	re0023 := regexp.MustCompile(`&c([^\.]|$)`)                                                         // abbreviation &c without period
	re0024 := regexp.MustCompile(`^[!;:,.?]`)                                                           // line starts with (selected) punctuation
	re0025 := regexp.MustCompile(`^-[^-]`)                                                              // line starts with hyphen followed by non-hyphen

	// some traditional gutcheck tests were for
	//   "string that contains cb", "string that ends in cl", "string that contains gbt",
	//   "string containing mcnt (s/b ment)", "string that contains rnb, rnm or rnp",
	//   "string that contains tb", "string that contains tii", "string that contains tli",
	//   "character strings that end with j (s/b semicolon)"
	//   "string containing at least 5 consonants in a row"
	//   "string starts with hl, hr, or rn"
	//   "string contains invalid 'hl' sequence"
	//   "string "uess" not preceded by a g"
	//	 "string ii not at the beginning of a word"
	//   "a string that starts with one of c, s or w, li, then a vowel excluding 'client'"
	// these will be caught by spellcheck and are not tested here

	/*

		re0007 := regexp.MustCompile(``)  //
		re0007 := regexp.MustCompile(``)  //
		re0007 := regexp.MustCompile(``)  //
	*/

	type reportln struct {
		rpt        string
		sourceline string
	}

	gcreports := make([]reportln, 0)

	const (
		// commas should not occur after these words
		NOCOMMAPATTERN = `\b(the,|it's,|their,|an,|mrs,|a,|our,|that's,|its,|whose,|every,|i'll,|your,|my,|mr,|mrs,|mss,|mssrs,|ft,|pm,|st,|dr,|rd,|pp,|cf,|jr,|sr,|vs,|lb,|lbs,|ltd,|i'm,|during,|let,|toward,|among,)`

		// periods should not occur after these words
		NOPERIODPATTERN = `\b(every\.|i'm\.|during\.|that's\.|their\.|your\.|our\.|my\.|or\.|and\.|but\.|as\.|if\.|the\.|its\.|it's\.|until\.|than\.|whether\.|i'll\.|whose\.|who\.|because\.|when\.|let\.|till\.|very\.|an\.|among\.|those\.|into\.|whom\.|having\.|thence\.)`
	)

	re_comma := regexp.MustCompile(NOCOMMAPATTERN)
	re_period := regexp.MustCompile(NOPERIODPATTERN)
	abandonedTagCount := 0 // courtesy limit if user uploads fpgen source, etc.

	for n, line := range wb {

		if re0000.MatchString(line) {
			gcreports = append(gcreports, reportln{"opening square bracket followed by other than I, G or number", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0001.MatchString(line) {
			gcreports = append(gcreports, reportln{"punctuation after 'the'", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0002.MatchString(line) {
			gcreports = append(gcreports, reportln{"punctuation error", fmt.Sprintf("  %5d: %s", n, line)})
		}
		// check each word separately on this line
		for _, word := range models.Lwl[n] {
			// check for mixed case within word after the first character,
			// but not if the word is in the good word list or if it occurs more than once
			if models.Wlm[word] < 2 && !util.InGoodWordList(word) && re0003a.MatchString(word[1:]) && re0003b.MatchString(word[1:]) {
				gcreports = append(gcreports, reportln{"mixed case within word", fmt.Sprintf("  %5d: %s", n, line)})
			}
			if len(word) > 2 {
				last2 := word[len(word)-2:]
				if re0003c.MatchString(last2) {
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word ending with %s", last2), fmt.Sprintf("  %5d: %s", n, line)})
				}
				first2 := word[:2]
				if re0003d.MatchString(first2) {
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word starting with %s", first2), fmt.Sprintf("  %5d: %s", n, line)})
				}
			}
		}
		if re0004.MatchString(line) {
			gcreports = append(gcreports, reportln{"initials spacing", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0006.MatchString(line) {
			gcreports = append(gcreports, reportln{"single character line", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0007.MatchString(line) {
			gcreports = append(gcreports, reportln{"broken hyphenation", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0008a.MatchString(line) ||
			re0008b.MatchString(line) ||
			re0008c.MatchString(line) ||
			re0008d.MatchString(line) {
			gcreports = append(gcreports, reportln{"comma spacing", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0009a.MatchString(line) ||
			re0009b.MatchString(line) {
			gcreports = append(gcreports, reportln{"full-stop spacing", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0010.MatchString(line) {
			gcreports = append(gcreports, reportln{"date format", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0011.MatchString(line) {
			gcreports = append(gcreports, reportln{"I/! check", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0012.MatchString(line) {
			gcreports = append(gcreports, reportln{"disjointed contraction", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0013.MatchString(line) {
			gcreports = append(gcreports, reportln{"title abbreviation comma", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0014.MatchString(line) {
			gcreports = append(gcreports, reportln{"spaced punctuation", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0016.MatchString(line) {
			if abandonedTagCount < 10 {
				gcreports = append(gcreports, reportln{"abandoned HTML tag", fmt.Sprintf("  %5d: %s", n, line)})
			}	
			abandonedTagCount++
		}
		if re0017.MatchString(line) {
			gcreports = append(gcreports, reportln{"ellipsis check", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0018.MatchString(line) {
			gcreports = append(gcreports, reportln{"quote error (context)", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0019.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 0", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0020a.MatchString(line) && !re0020b.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 1", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0021.MatchString(line) {
			gcreports = append(gcreports, reportln{"mixed letters and numbers in word", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0022.MatchString(line) {
			gcreports = append(gcreports, reportln{"trailing space on line", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0023.MatchString(line) {
			gcreports = append(gcreports, reportln{"abbreviation &c without period", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0024.MatchString(line) {
			gcreports = append(gcreports, reportln{"line starts with suspect punctuation", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re0025.MatchString(line) {
			gcreports = append(gcreports, reportln{"line that starts with hyphen and then non-hyphen", fmt.Sprintf("  %5d: %s", n, line)})
		}

		// begin non-regexp based
		if strings.Contains(line, "Blank Page") {
			gcreports = append(gcreports, reportln{"Blank Page placeholder found", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if strings.Contains(line, "—-") || strings.Contains(line, "-—") {
			gcreports = append(gcreports, reportln{"mixed hyphen/dash", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if strings.Contains(line, "\u00A0") {
			gcreports = append(gcreports, reportln{"non-breaking space", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if strings.Contains(line, "\u00AD") {
			gcreports = append(gcreports, reportln{"soft hyphen", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if strings.Contains(line, "\u0009") {
			gcreports = append(gcreports, reportln{"tab character", fmt.Sprintf("  %5d: %s", n, line)})
		}
		if strings.Contains(line, "&") {
			gcreports = append(gcreports, reportln{"ampersand character", fmt.Sprintf("  %5d: %s", n, line)})
		}
		lcline := strings.ToLower(line)
		if re_comma.MatchString(lcline) {
			gcreports = append(gcreports, reportln{fmt.Sprintf("  unexpected comma after word"), fmt.Sprintf("  %5d: %s", n, line)})
		}
		if re_period.MatchString(lcline) {
			gcreports = append(gcreports, reportln{fmt.Sprintf("  unexpected period after word"), fmt.Sprintf("  %5d: %s", n, line)})
		}
	}

	sort.Slice(gcreports, func(i, j int) bool { return gcreports[i].sourceline < gcreports[j].sourceline })
	sort.SliceStable(gcreports, func(i, j int) bool { return gcreports[i].rpt < gcreports[j].rpt })

	if abandonedTagCount > 10 {
		rs = append(rs, fmt.Sprintf("note: source file not plain text. %d lines with markup", abandonedTagCount))
	}

	if len(gcreports) == 0 {
		report("  no special situation reports.")
	} else {
		rrpt_last := ""
		for _, rpt := range gcreports {
			if rpt.rpt != rrpt_last {
				rs = append(rs, fmt.Sprintf("%s\n%s", rpt.rpt, rpt.sourceline))
			} else {
				rs = append(rs, fmt.Sprintf("%s", rpt.sourceline))
			}
			rrpt_last = rpt.rpt
		}
	}
}

// text checks
// a series of tests either on the working buffer (line at a time)
// or the paragraph buffer (paragraph at a time)
func Textcheck() {
	rs = append(rs, fmt.Sprintf("\n********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "TEXT ANALYSIS REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	asteriskCheck(models.Wb)
	adjacentSpaces(models.Wb)
	trailingSpaces(models.Wb)
	letterChecks(models.Wb)
	spacingCheck(models.Wb)
	shortLines(models.Wb)
	longLines(models.Wb)
	repeatedWords(models.Pb)
	ellipsisCheck(models.Wb)
	dashCheck(models.Pb)
	scannoCheck(models.Wb)
	curlyQuoteCheck(models.Wb)
	gutChecks(models.Wb)
	bookLevel(models.Wb)
	paraLevel()

	models.Report = append(models.Report, rs...)
}

/*
Common abbreviations and other OK words not to query as typos.
okword = ("mr", "mrs", "mss", "mssrs", "ft", "pm", "st", "dr", "hmm", "h'm",
               "hmmm", "rd", "sh", "br", "pp", "hm", "cf", "jr", "sr", "vs", "lb",
               "lbs", "ltd", "pompeii", "hawaii", "hawaiian", "hotbed", "heartbeat",
               "heartbeats", "outbid", "outbids", "frostbite", "frostbitten")

Common abbreviations that cause otherwise unexplained periods.
okabbrev = ("cent", "cents", "viz", "vol", "vols", "vid", "ed", "al", "etc",
               "op", "cit", "deg", "min", "chap", "oz", "mme", "mlle", "mssrs")

Checks to do:
    m1 = re.search(r"^,", line)  # no comma starts a line
    m2 = re.search(r"\s,", line)  # no space before a comma
    m3 = re.search(r"\s,\s", line)  # no floating comma
    m4 = re.search(r",\w", line)  # always a space after a comma unless a digit

*/
