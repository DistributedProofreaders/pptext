/*
filename:  pptext.go
author:    Roger Frank
license:   GPL
status:    beta
*/

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const VERSION string = "2019.02.05"

var sw []string // suspect words list

var rs []string // array of strings for local aggregation

var pptr []string // pptext report

var puncStyle string // punctuation style American or British

// return true if slice contains string
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// text-wrap string into string with embedded newlines
// with leader 9 spaces after first
func wraptext9(s string) string {
	s2 := ""
	runecount := 0
	// rc := utf8.RuneCountInString(s)
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s) // first
		runecount++
		// replace space with newline (break on space)
		if runecount >= 70 && r == ' ' {
			s2 += "\n         "
			runecount = 0
		} else {
			s2 += string(r) // append single rune to string
		}
		s = s[size:] // chop off rune
	}
	return s2
}

/* ********************************************************************** */
/*                                                                        */
/* from params.go                                                         */
/*                                                                        */
/* ********************************************************************** */

type params struct {
	Infile       string
	Outfile      string
	Outfileh     string
	Wlang        string
	GWFilename   string
	Experimental bool
	Nolev        bool
	Nosqc        bool
	NoBOM        bool
	UseLF        bool
	Verbose      bool
}

var p params

// the lwl is a slice of slices of strings, one per line. order maintained
// lwl[31] contains a slice of strings containing the words on line "31"
var lwl []([]string)

// wordListMap has all words in the book and their frequency of occurence
// wordListMap["chocolate"] -> 3 means that word occurred three times
var wordListMap map[string]int

// paragraph buffer
var pbuf []string

// working buffer
var wbuf []string

// working dictionary
var wdic []string // working dictionary inc. good words

// heMap and beMap maps map word sequences to relative frequency of occurence
// higher values mean more frequently seen
var heMap map[string]int
var beMap map[string]int

var scannoWordlist []string // scanno word list
var goodWordlist []string   // good word list specified by user

/* ********************************************************************** */
/*                                                                        */
/* from fileio.go                                                         */
/*                                                                        */
/* ********************************************************************** */

var BOM = string([]byte{239, 187, 191}) // UTF-8 Byte Order Mark

// readLn returns a single line (without the ending \n)
// from the input buffered reader.
// An error is returned if there is an error with the
// buffered reader.
func readLn(r *bufio.Reader) (string, error) {
	var (
		isPrefix bool  = true
		err      error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return string(ln), err
}

func readText(infile string) []string {
	wb := []string{}
	f, err := os.Open(infile)
	if err != nil {
		s := fmt.Sprintf("error opening file: %v\n", err)
		pptr = append(pptr, s)
	} else {
		r := bufio.NewReader(f)
		s, e := readLn(r) // read first line
		for e == nil {    // continue as long as there are no errors reported
			wb = append(wb, s)
			s, e = readLn(r)
		}
	}
	// successfully read. remove BOM if present
	if len(wb) > 0 {
		wb[0] = strings.TrimPrefix(wb[0], BOM)
	}
	return wb
}

// saves report to user-specified file
//
func saveText(a []string, outfile string, noBOM bool, useLF bool) {
	f2, err := os.Create(outfile) // specified report file for text output
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()
	if !noBOM { // normally provide a Byte Order Mark
		a[0] = BOM + a[0]
	}
	for _, line := range a {
		if useLF {
			fmt.Fprintf(f2, "%s\n", line)
		} else {
			s := strings.Replace(line, "\n", "\r\n", -1)
			// remove any HTML markup tokens
			if strings.ContainsAny(line, "☳") {
				continue
			}
			re := regexp.MustCompile(`[☰☱☲☳☴☵☶☷]`) // strip any tokens for HTML
			s = re.ReplaceAllString(s, "")
			fmt.Fprintf(f2, "%s\r\n", s)
		}
	}
}

var HHEAD = []string{
	"<html>",
	"<head>",
	"<meta charset=\"utf-8\">",
	"<meta name=viewport content=\"width=device-width, initial-scale=1\">",
	"<title>pptext report</title>",
	"<style type=\"text/css\">",
	"body { margin-left: 1em;}",
	".red { color:red; background-color: #FFFFAA; }",
	".green { color:green; background-color: #FFFFAA; }",
	".black { color:black; }",
	".dim { color:#999999; }",
	"</style>",
	"</head>",
	"<body>",
	"<pre>"}

var HFOOT = []string{
	"</pre>",
	"</body>",
	"</html>"}

// saves HTML report to report.html
//
func saveHtml(a []string, outfile string, noBOM bool, useLF bool) {
	f2, err := os.Create(outfile) // specified report file for text output
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()
	for _, line := range HHEAD {
		s := strings.Replace(line, "\n", "\r\n", -1)
		fmt.Fprintf(f2, "%s\r\n", s)
	}
	// as I emit the HTML, look for predefined tokens that generate spans
	// or other info.
	// ☰ <style class='red'>
	// ☱ <style class='green'>
	// ☲ <style class='dim'>
	// ☳ this token defines a comment in text and a literal line in HTML
	// ☴
	// ☵
	// ☶
	// ☷ </style>
	for _, line := range a {
		s := strings.Replace(line, "\n", "\r\n", -1)

		// there should not be any HTML tags in the report
		// unless they are specifically marked
		if !strings.ContainsAny(s, "☳") {
			s = strings.Replace(s, "<", "<span class='green'>❬", -1)
			s = strings.Replace(s, ">", "❭</span>", -1)
		}

		s = strings.Replace(s, "☰", "<span class='red'>", -1)
		s = strings.Replace(s, "☱", "<span class='green'>", -1)
		s = strings.Replace(s, "☲", "<span class='dim'>", -1)
		s = strings.Replace(s, "☳", "<span class='black'>", -1)
		s = strings.Replace(s, "☷", "</span>", -1)
		s = strings.Replace(s, "☳", "", -1) // HTML-only flag

		fmt.Fprintf(f2, "%s\r\n", s)
	}
	for _, line := range HFOOT {
		s := strings.Replace(line, "\n", "\r\n", -1)
		fmt.Fprintf(f2, "%s\r\n", s)
	}
}

/* ********************************************************************** */
/*                                                                        */
/* from scan.go                                                           */
/*                                                                        */
/* ********************************************************************** */

var lpb []string // local paragraph buffer
var pnm []string // proper names in this text

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
			ip := sort.SearchStrings(wdic, testwordlc)     // where it would insert
			if ip != len(wdic) && wdic[ip] == testwordlc { // true if we found it
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
			ip := sort.SearchStrings(wdic, strippedword_lc)
			if ip != len(wdic) && wdic[ip] == strippedword_lc {
				strippedisword = true
			}
			ip = sort.SearchStrings(wdic, augmentedword_lc)
			if ip != len(wdic) && wdic[ip] == augmentedword_lc {
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
	ip := sort.SearchStrings(wdic, word)
	return ip != len(wdic) && wdic[ip] == word
}

// proper names
// find proper names in this text. protect possessive
// any capitalized word in good word list is considered a proper name
//
func properNames() {
	re := regexp.MustCompile(`^\p{Lu}\p{Ll}`) // uppercase followed by lower case
	for _, word := range goodWordlist {
		if re.MatchString(word) {
			pnm = append(pnm, word)
		}
	}
	for word, freq := range wordListMap {
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
			// second attempt:
			// c3 := strings.Replace(clead, "’", "", 1) // replace only the lead char
			// re := regexp.MustCompile(fmt.Sprintf(`(^|\P{L})’(%s)(\P{L}|$])`, c3))
			// lpb[n] = re.ReplaceAllString(lpb[n], fmt.Sprintf(`◳$2$3`))
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
	for _, line := range pbuf {
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

func doScan() []string {
	nreports := 0
	rs := []string{}
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "PUNCTUATION SCAN REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	// scan each paragraph
	for n, p := range lpb {
		// fmt.Println(p)
		stk := ""                  // new stack for each line
		cpos := 0                  // current rune position
		query := make([]string, 0) // query list per paragraph
		for cpos < len(p) {        // go across the line for double, single quotes
			r, size := utf8.DecodeRuneInString(p[cpos:])

			if r == '“' {
				// open double quote. check for consecutive open DQ
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '“' {
					query = append(query, "query: consec open double quote")
				}
				stk += string('“')
			}
			if r == '”' {
				// close double quote. check for consecutive close DQ or unmatched
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '”' {
					query = append(query, "query: consec close double quote:")
				}
				if r2 != '“' {
					query = append(query, "query: unmatched close double quote:")
				}
				stk = strings.TrimSuffix(stk, "“")
			}

			if r == '‘' {
				// open single quote. check for consecutive open SQ
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '‘' {
					query = append(query, "query: consec open single quote")
				}
				stk += string('‘')
			}
			if r == '’' {
				// close single quote.
				// if there is an open single quote, pair it
				r2, _ := utf8.DecodeLastRuneInString(stk)
				if r2 == '‘' {
					stk = strings.TrimSuffix(stk, "‘")
				}
				/*
					 // if I wanted to report (a lot) of suspects:
					 if r2 == '‘' {
					    stk = strings.TrimSuffix(stk, "‘")
					 } else {
					    query = append(query, "query: unbalanced close single quote")
					}
				*/

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
				query = append(query, fmt.Sprintf("query: unmatched open double quote"))
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

			// save this query
			rs = append(rs, fmt.Sprintf("☳%s☷\n  ☱%s☷\n", query[0], s2))
			nreports++
		}
	}
	if nreports == 0 {
		rs = append(rs, "  no punctuation scan queries reported")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	return rs
}

func puncScan() []string {
	rs := []string{} // local rs to start aggregation

	// get a copy of the paragraph buffer to edit in place
	lpb = make([]string, len(pbuf))
	copy(lpb, pbuf)
	if straightCurly() { // check for mixed style
		// short-circuit report
		rs = append(rs, "☳********************************************************************************")
		rs = append(rs, "* PUNCTUATION SCAN REPORT                                                      *")
		rs = append(rs, "********************************************************************************")
		rs = append(rs, "  ☰mixed straight and curly quotes.☷ curly quote scan not done☷")
		rs = append(rs, "")
		return rs
	}
	pluralPossessive() // horses’ becomes horses◳
	internals()        // protect internal '’' characters as in contractions
	commonforms()      // protect known common forms
	ingResolve()       // resolve -ing words that are contracted
	wordQuotes()       // apparent quoted words or two-word phrases
	properNames()      // protect proper names
	t := doScan()      // scan quotes and report, returns []string
	rs = append(rs, t...)
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* from spellcheck.go                                                     */
/*                                                                        */
/* ********************************************************************** */

// lookup word. also check using lower case
func lookup(wd []string, word string) bool {
	foundit := false
	ip := sort.SearchStrings(wd, word) // where it would insert
	if ip != len(wd) && wd[ip] == word {
		foundit = true
	}
	ip = sort.SearchStrings(wd, strings.ToLower(word)) // where l/c would insert
	if ip != len(wd) && wd[ip] == strings.ToLower(word) {
		foundit = true
	}
	return foundit
}

// spellcheck returns list of suspect words, list of ok words in text

func spellCheck(wd []string) ([]string, []string, []string) {
	pptr = append(pptr, "☳<a name='spell'></a>")
	rs := []string{} // empty rs to start aggregation
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "SPELLCHECK REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	okwordlist := make(map[string]int) // cumulative words OK by successive tests
	var willdelete []string            // words to be deleted from wordlist

	// local wlmLocal keeps wordListMap intact with all words/frequencies
	// local wlmLocal is pruned as words are approved during spellcheck
	wlmLocal := make(map[string]int, len(wordListMap)) // copy of all words in the book, with freq
	for k, v := range wordListMap {
		// swd := strings.Replace(k, "’", "'", -1) // straighten apostrophes for spell check
		wlmLocal[k] = v
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  ☲unique words in text: %d words", len(wlmLocal)))
	}

	for word, count := range wlmLocal {
		ip := sort.SearchStrings(wd, word)   // where it would insert
		if ip != len(wd) && wd[ip] == word { // true if we found it
			// ok by wordlist
			okwordlist[word] = count // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by dictionary: %d words", len(willdelete)))
	}

	// delete words that have been OKd by being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Printf("%+v\n", willdelete)
	// os.Exit(1)

	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(okwordlist))

	// typically at this point, I have taken the 8995 unique words in the book
	// and sorted it into 7691 words found in the dictionary and 1376 words unresolved

	// fmt.Printf("%+v\n", wlmLocal)
	// try to approve words that are capitalized by testing them lower case

	/* =============================================================================== */

	// check words ok by depossessive
	re := regexp.MustCompile(`’s$`)
	for word, count := range wlmLocal {
		if re.MatchString(word) {
			testword := strings.Replace(word, "’s", "", -1) // drop suffix
			ip := sort.SearchStrings(wd, testword)          // where it would insert
			if ip != len(wd) && wd[ip] == testword {        // true if we found it
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by depossessive: %d words", len(willdelete)))
	}

	// delete words that have been OKd by their depossessive form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// check words ok by plural -> singular
	re = regexp.MustCompile(`s$`)
	for word, count := range wlmLocal {
		if re.MatchString(word) { // ends with s
			testword := word[:len(word)-1]           // drop the s
			ip := sort.SearchStrings(wd, testword)   // where it would insert
			if ip != len(wd) && wd[ip] == testword { // true if we found it
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by plural/singular: %d words", len(willdelete)))
	}

	// delete words that have been OKd by their singular form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// check words ok by dropping "’ll" as in that’ll or there’ll
	re = regexp.MustCompile(`’ll$`)
	for word, count := range wlmLocal {
		if re.MatchString(word) { // ends with ’ll
			testword := word[:len(word)-3]           // drop the ’ll
			ip := sort.SearchStrings(wd, testword)   // where it would insert
			if ip != len(wd) && wd[ip] == testword { // true if we found it
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by ’ll contraction: %d words", len(willdelete)))
	}

	// delete words that have been OKd by their ’ll form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// check: words OK by their lowercase form being in the dictionary
	lcwordlist := make(map[string]int) // all words in lower case

	for word, count := range wlmLocal {
		lcword := strings.ToLower(word)
		ip := sort.SearchStrings(wd, lcword)   // where it would insert
		if ip != len(wd) && wd[ip] == lcword { // true if we found it
			// ok by lowercase
			lcwordlist[word] = count // remember (uppercase versions) as good word
			okwordlist[word] = count // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by lowercase form: %d words", len(willdelete)))
	}

	// delete words that have been OKd by their lowercase form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// new 28-Jan-2019: if a word is in all upper case,
	// check if OK by their titlecase form being in the dictionary
	// will approve BRITISH since British is in dictionary
	tcwordlist := make(map[string]int) // all words in title case

	for word, count := range wlmLocal {
		if strings.ToUpper(word) == word { // it is upper case
			tcword := strings.Title(strings.ToLower(word))
			ip := sort.SearchStrings(wd, tcword)   // where it would insert
			if ip != len(wd) && wd[ip] == tcword { // true if we found it
				// ok by title case
				tcwordlist[word] = count // remember upper case version as good word
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by titlecase form: %d words", len(willdelete)))
	}

	// delete words that have been OKd by their lowercase form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// fmt.Printf("%+v\n", wlmLocal)
	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(lcwordlist))

	// some of these are hyphenated. Break those words on hyphens and see if all the individual parts
	// are valid words. If so, approve the hyphenated version

	hywordlist := make(map[string]int) // hyphenated words OK by all parts being words

	for word, count := range wlmLocal {
		t := strings.Split(word, "-")
		if len(t) > 1 { // if it split, have hyphenated word 2 of more words
			// we have a hyphenated word
			allgood := true
			for _, hpart := range t { // go over each part of hyphenated word
				if !lookup(wd, hpart) {
					allgood = false // any part can fail it
				}
			}
			if allgood { // all parts of the hyhenated word are words
				hywordlist[word] = count
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by dehyphenation: %d words", len(willdelete)))
	}

	// delete words that have been OKd by dehyphenation
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(hywordlist))

	// of the 738 unresolved words before dehyphenation checks, now an additional
	// 235 have been approved and 503 remain unresolved

	/*
		// some "words" are entirely numerals. approve those
		for word, _ := range wlmLocal {
			if _, err := strconv.Atoi(word); err == nil {
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	*/

	// some "words" are entirely numerals or numerals with common fractions. approve those
	for word, _ := range wlmLocal {
		dofrac := func(r rune) rune {
			if strings.ContainsRune("⅐⅑⅒⅓⅔⅕⅖⅗⅘⅙⅚⅛⅜⅝⅞¼½¾", r) {
				return '0'
			}
			return r
		}
		orig_word := word
		word = strings.Map(dofrac, word)
		if _, err := strconv.Atoi(word); err == nil {
			okwordlist[orig_word] = 1 // remember as good word
			willdelete = append(willdelete, orig_word)
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved pure numerics: %d words", len(willdelete)))
	}
	// delete words that are entirely numeric
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	// some "words" are numerals with "th", "nd", "rd", "st" (17th, 22nd, 3rd, 31st)
	re = regexp.MustCompile(`\d+(th|nd|rd|st)`)
	for word, _ := range wlmLocal {
		if re.MatchString(word) {
			okwordlist[word] = 1 // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved mixed numerics: %d words", len(willdelete)))
	}

	// delete words that are mixed numeric
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	for word, _ := range wlmLocal {

		if strings.HasSuffix(word, "’ll") {
			t := strings.Replace(word, "’ll", "", -1)
			ip := sort.SearchStrings(wd, t)                     // where it would insert
			iplow := sort.SearchStrings(wd, strings.ToLower(t)) // where lc would insert
			if (ip != len(wd) && wd[ip] == t) ||
				(iplow != len(wd) && wd[iplow] == strings.ToLower(t)) { // true if we found it
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}

		if strings.HasSuffix(word, "’ve") {
			t := strings.Replace(word, "’ve", "", -1)
			ip := sort.SearchStrings(wd, t)                     // where it would insert
			iplow := sort.SearchStrings(wd, strings.ToLower(t)) // where lc would insert
			if (ip != len(wd) && wd[ip] == t) ||
				(iplow != len(wd) && wd[iplow] == strings.ToLower(t)) { // true if we found it
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}

		if strings.HasSuffix(word, "’d") {
			t := strings.Replace(word, "’d", "", -1)
			ip := sort.SearchStrings(wd, t)                     // where it would insert
			iplow := sort.SearchStrings(wd, strings.ToLower(t)) // where lc would insert
			if (ip != len(wd) && wd[ip] == t) ||
				(iplow != len(wd) && wd[iplow] == strings.ToLower(t)) { // true if we found it
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}

		if strings.HasSuffix(word, "n’t") {
			t := strings.Replace(word, "n’t", "", -1)
			ip := sort.SearchStrings(wd, t)                     // where it would insert
			iplow := sort.SearchStrings(wd, strings.ToLower(t)) // where lc would insert
			if (ip != len(wd) && wd[ip] == t) ||
				(iplow != len(wd) && wd[iplow] == strings.ToLower(t)) { // true if we found it
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}

		if strings.HasSuffix(word, "’s") {
			t := strings.Replace(word, "’s", "", -1)
			ip := sort.SearchStrings(wd, t)                     // where it would insert
			iplow := sort.SearchStrings(wd, strings.ToLower(t)) // where lc would insert
			if (ip != len(wd) && wd[ip] == t) ||
				(iplow != len(wd) && wd[iplow] == strings.ToLower(t)) { // true if we found it
				okwordlist[word] = 1 // remember as good word
				willdelete = append(willdelete, word)
			}
		}

	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by removing contraction: %d words", len(willdelete)))
	}
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	/* =============================================================================== */

	frwordlist := make(map[string]int) // words ok by frequency occuring 4 or more times

	// some words occur many times. Accept them by frequency if they appear four or more times
	// spelled the same way
	for word, count := range wlmLocal {
		if count >= 4 {
			frwordlist[word] = count
			okwordlist[word] = count // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	if p.Verbose {
		rs = append(rs, fmt.Sprintf("  approved by frequency: %d words", len(willdelete)))
	}
	// delete words approved by frequency
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// sort remaining words in wlmLocal
	var keys []string
	for k := range wlmLocal {
		keys = append(keys, k)
	}
	// sort.Strings(keys)
	// case insensitive sort
	sort.Slice(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })

	// show each word in context
	var sw []string                    // suspect words
	lcreported := make(map[string]int) // map to hold downcased words reported

	if p.Verbose {
		rs = append(rs, "--------------------------------------------------------------------------------☷")
	}
	
	rs = append(rs, fmt.Sprintf("Suspect words"))
	for _, word := range keys {

		// if I've reported the word in any case, don't report it again
		lcword := strings.ToLower(word)
		if _, ok := lcreported[lcword]; ok { // if it is there already, skip
			continue
		} else {
			lcreported[lcword] = 1
		}

		// word = strings.Replace(word, "'", "’", -1)
		sw = append(sw, word)                    // simple slice of only the word
		rs = append(rs, fmt.Sprintf("%s", word)) // word we will show in context
		// show word in text
		for n, line := range wbuf { // every line
			for _, t2 := range lwl[n] { // every word on that line
				if t2 == word { // it is here
					re := regexp.MustCompile(`(?i)(^|\P{L})(` + word + `)(\P{L}|$)`)
					line = re.ReplaceAllString(line, `$1☰$2☷$3`)
					re = regexp.MustCompile(`☰`)
					loc := re.FindStringIndex(line)
					line = getParaSegment(line, loc[0])
					rs = append(rs, fmt.Sprintf("  %6d: %s", n+1, line)) // 1-based
					break
				}
			}
		}
		rs = append(rs, "")
	}

	// rs = append(rs, fmt.Sprintf("  ☲good words in text: %d words", len(okwordlist)))
	// rs = append(rs, fmt.Sprintf("  suspect words in text: %d words☷", len(sw)))

	var ok []string
	for word, _ := range okwordlist {
		ok = append(ok, word)
	}

	if len(sw) == 0 {
		rs = append(rs, "  none")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style

	// return sw: list of suspect words and ok: list of good words in text
	// and rs, the report

	return sw, ok, rs
}

/* ********************************************************************** */
/*                                                                        */
/* from util.go                                                          */
/*                                                                        */
/* ********************************************************************** */

// compare the good word list to the submitted word
// allow variations, i.e. "Rose-Ann" in GWL will match "Rose-Ann’s"
func inGoodWordList(s string) bool {
	for _, word := range goodWordlist {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

type rp struct {
	rpr rune
	rpp int
}

func getParaSegment(ss string, where int) string {

	s := "" // string to return

	// convert to array of runes
	rps := make([]rp, 0)
	for i, w := 0, 0; i < len(ss); i += w {
		runeValue, width := utf8.DecodeRuneInString(ss[i:])
		rps = append(rps, rp{rpr: runeValue, rpp: i})
		w = width
	}

	llim := where - 30
	rlim := where + 30
	// if llim = -5 we are 5 too far. tack to right
	if llim < 0 {
		rlim += -llim
		llim = 0
	}
	// if rlim > len(rps), too far. tack to left
	if rlim > len(rps) {
		llim -= rlim - len(rps)
		rlim = len(rps)
	}
	// last adjust: llim underflow
	if llim < 0 {
		llim = 0
	}

	// find spaces...
	for ; llim > 0 && rps[llim].rpr != ' '; llim-- {
	}
	for ; rlim < len(rps) && rps[rlim].rpr != ' '; rlim++ {
	}

	rpseg := rps[llim:rlim]
	for _, t := range rpseg {
		s += string(t.rpr)
	}
	s = strings.TrimSpace(s)

	return s
}

func getPuncStyle() string {

	// decide is this is American or British punctuation
	cbrit, camer := 0, 0
	for _, line := range wbuf {
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

// Pretty print variable (struct, map, array, slice) in Golang.

func prettyPrint(v interface{}) (err error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		fmt.Println(string(b))
	}
	return
}

/* ********************************************************************** */
/*                                                                        */
/* from textcheck.go                                                      */
/*                                                                        */
/* ********************************************************************** */

// check for "motor-car" and "motorcar"

func tcHypConsistency(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- hyphenation and non-hyphenated check ------------------------------------")
	rs = append(rs, "")

	count := 0
	for s, _ := range wordListMap {
		if strings.Contains(s, "-") {
			// hyphenated version present
			s2 := strings.Replace(s, "-", "", -1)
			// create non-hyphenated version and look for it
			reported := false
			for t, _ := range wordListMap {
				if reported {
					break
				}
				if t == s2 {
					// found it. report it
					count++
					rs = append(rs, fmt.Sprintf("%s (%d) ❬-❭ %s (%d)", s2, wordListMap[s2], s, wordListMap[s]))
					sdone, s2done := false, false
					for n, line := range wb {
						re1 := regexp.MustCompile(`(\P{L}` + s + `\P{L})`)
						if !sdone && re1.MatchString(" "+line+" ") {
							line = re1.ReplaceAllString(" "+line+" ", `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n+1, strings.TrimSpace(line)))  // 1-based
							sdone = true
						}
						re2 := regexp.MustCompile(`(\P{L}` + s2 + `\P{L})`)
						if !s2done && re2.MatchString(" "+line+" ") {
							line = re2.ReplaceAllString(" "+line+" ", `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n+1, strings.TrimSpace(line)))  // 1-based
							s2done = true
						}
						if sdone && s2done {
							rs = append(rs, "")
							reported = true
							break
						}
					}
				}
			}
		}
	}

	if count == 0 {
		rs = append(rs, "  no hyphenation/non-hyphenated inconsistencies found.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// “We can’t let that man get away!”
// “Get-away from here!”

func tcHypSpaceConsistency(wb []string, pb []string) []string {
	rs := []string{}
	rs = append(rs, "----- hyphenation and spaced pair check ---------------------------------------")
	rs = append(rs, "")
	count := 0

	re := regexp.MustCompile(`\p{L}\p{L}+ \p{L}\p{L}+`) // two words sep by space
	for s, _ := range wordListMap {
		if strings.Contains(s, "-") {
			s1done, s2done := false, false
			reported := false
			// split into two words into hpair (hyphenation pair)
			hpair := strings.Split(s, "-")
			if len(hpair) != 2 {
				continue // only handle two words with one hyphen
			}
			// both words lower case for compare
			hpairlow := []string{strings.ToLower(hpair[0]), strings.ToLower(hpair[1])}
			// go through each paragraph and look for those two words
			// in succession separated by a space
			for _, para := range pb { // go over each paragraph
				if reported {
					continue
				}
				start := 0 // at start of para
				// find two space separated words into spair (space pair)
				for u := re.FindStringIndex(para[start:]); u != nil; {
					pair := (para[start+u[0] : start+u[1]])
					spair := strings.Split(pair, " ")
					// both of these words also lower case also
					spairlow := []string{strings.ToLower(spair[0]), strings.ToLower(spair[1])}
					if !reported && spairlow[0] == hpairlow[0] && spairlow[1] == hpairlow[1] {
						// we have "get away" and "get-away" case insensitive
						count++

						s1a := fmt.Sprintf("(?i)(\\P{L}%s-%s\\P{L})", hpair[0], hpair[1])
						re1a := regexp.MustCompile(s1a) // two words sep by hyphen
						s2a := fmt.Sprintf("(?i)(\\P{L}%s %s\\P{L})", spair[0], spair[1])
						re2a := regexp.MustCompile(s2a) // two words sep by space

						// count of each type
						counthyp, countspc := 0, 0
						for n, line := range wb {
							if re1a.MatchString(" " + line + " ") {
								counthyp++
							}
							if re2a.MatchString(" " + line + " ") {
								countspc++
							}
							if (n < len(wb)-1) && strings.HasSuffix(line, spair[0]) && strings.HasPrefix(wb[n+1], spair[1]) {
								countspc++
							}
						}

						rs = append(rs, fmt.Sprintf("\"%s-%s\" (%d) ❬-❭ \"%s %s\" (%d)",
							hpair[0], hpair[1], counthyp, spair[0], spair[1], countspc))
						// show where they are (case insensitive)

						for n, line := range wb {
							// hyphenated
							if !s1done && re1a.MatchString(" "+line+" ") {
								line = re1a.ReplaceAllString(" "+line+" ", `☰$1☷`)
								line = strings.Replace(line, "☰ ", " ☰", -1)
								line = strings.Replace(line, " ☷", "☷ ", -1)
								rs = append(rs, fmt.Sprintf("%7d: %s", n+1, strings.TrimSpace(line)))  // 1=based
								s1done = true
							}
							// spaced (can be over two lines)
							if !s2done && re2a.MatchString(" "+line+" ") {
								line = re2a.ReplaceAllString(" "+line+" ", `☰$1☷`)
								line = strings.Replace(line, "☰ ", " ☰", -1)
								line = strings.Replace(line, " ☷", "☷ ", -1)
								rs = append(rs, fmt.Sprintf("%7d: %s", n+1, strings.TrimSpace(line)))  // 1=based
								s2done = true
							}
							if (n < len(wb)-1) && !s2done && strings.HasSuffix(line, spair[0]) && strings.HasPrefix(wb[n+1], spair[1]) {
								re3t := regexp.MustCompile("(" + spair[0] + ")")
								ltop := re3t.ReplaceAllString(wb[n], `☰$1☷`)
								rs = append(rs, fmt.Sprintf("%7d: %s", n+1, ltop))  // 1=based
								re3b := regexp.MustCompile("(" + spair[1] + ")")
								lbot := re3b.ReplaceAllString(wb[n+1], `☰$1☷`)
								rs = append(rs, fmt.Sprintf("         %s", lbot))
								s2done = true
							}
							if s1done && s2done {
								if !reported {
									rs = append(rs, "")
								}
								// we have reported once for this hyphenated word
								reported = true
								break
							}
						}
					}
					start += u[0] + len(spair[0])        // skip over first word
					u = re.FindStringIndex(para[start:]) // get next pair
				}
			}
		}
	}

	if count == 0 {
		rs = append(rs, "  no hyphenated/spaced pair inconsistencies found.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// curly quote check (positional, not using a state machine)
func tcCurlyQuoteCheck(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- curly quote check ------------------------------------------------------")
	rs = append(rs, "")

	countfq := 0 // count floating quote reports
	countqd := 0 // count fq direction reports
	ast := 0

	r0a := regexp.MustCompile(` [“”] `)
	r0b := regexp.MustCompile(`^[“”] `)
	r0c := regexp.MustCompile(` [“”]$`)

	for n, line := range wb {
		if r0a.MatchString(line) || r0b.MatchString(line) || r0c.MatchString(line) {
			if ast == 0 {
				rs = append(rs, fmt.Sprintf("%s", "floating quote"))
				ast++
			}
			if !p.Verbose && countfq < 5 {
				rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line)))  // 1=based
			}
			countfq++
		}
	}

	if !p.Verbose && countfq > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more floating quote reports", countfq-5))
	}

	ast = 0
	r1a := regexp.MustCompile(`[\.,;!?]+[‘“]`) // example.“
	r1b := regexp.MustCompile(`[A-Za-z]+[‘“]`) // example‘
	r1c := regexp.MustCompile(`”[A-Za-z]`)     // ”example
	for n, line := range wb {
		if r1a.MatchString(line) || r1b.MatchString(line) || r1c.MatchString(line) {
			if ast == 0 {
				rs = append(rs, fmt.Sprintf("%s", "quote direction"))
				ast++
			}
			if !p.Verbose && countqd < 5 {
				rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line)))  // 1=based
			}
			countqd++
		}
	}

	if !p.Verbose && countqd > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more quote direction reports", countqd-5))
	}
	if countfq == 0 && countqd == 0 {
		rs = append(rs, "  no curly quote (context) suspects found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec = tcec + countfq + countqd
	return rs
}

// scanno check
// iterate each scanno word from pptext.dat
// look for that word on each line of the book
// scannos list in pptext.dat
//   from https://www.pgdp.net/c/faq/stealth_scannos_eng_common.txt
func scannoCheck(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- scanno check -----------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for _, scannoword := range scannoWordlist { // each scanno candidate
		ast := 0
		// if user has put the word in the good word list, do not search for
		// it as a scanno
		if contains(goodWordlist, scannoword) {
			continue
		}
		for n, linewords := range lwl { // slice of slices of words per line
			for _, word := range linewords { // each word on line
				if word == scannoword {
					if ast == 0 {
						rs = append(rs, fmt.Sprintf("%s", word))
					}
					if ast < 5 || p.Verbose {
						line := wb[n]
						re := regexp.MustCompile(`(` + word + `)`)
						line = re.ReplaceAllString(line, `☰$1☷`)
						re = regexp.MustCompile(`☰`)
						loc := re.FindStringIndex(line)
						line = getParaSegment(line, loc[0])
						rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
						count++
					}
					ast++
				}
			}
		}
		if !p.Verbose && ast > 5 {
			rs = append(rs, fmt.Sprintf("         ... %d more", ast-5))
		}
	}
	if count == 0 {
		rs = append(rs, "  no suspected scannos found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// dash check
func tcDashCheck(pb []string) []string {
	rs := []string{}
	rs = append(rs, "----- dash check -------------------------------------------------------------")
	rs = append(rs, "")

	re := regexp.MustCompile(`(— )|( —)|(—-)|(-—)|(- -)|(— -)|(- —)`)
	count := 0
	for _, para := range pb {
		if re.MatchString(para) {
			u := re.FindStringIndex(para)
			if u != nil {
				if p.Verbose || count < 5 {
					s := getParaSegment(para, u[0])
					rs = append(rs, fmt.Sprintf("   [%3s] %s", para[u[0]:u[1]], s))
				}
				count++
			}
		}
	}
	if !p.Verbose && count > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more", count-5))
	}
	if count == 0 {
		rs = append(rs, "  no dash check suspects found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// ellipsis checks
func tcEllipsisCheck(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- ellipsis check ---------------------------------------------------------")
	rs = append(rs, "")

	// give... us some pudding, give...us some pudding
	re1 := regexp.MustCompile(`\p{L}\.\.\.[\s\p{L}]`)

	// give....us some pudding
	re2 := regexp.MustCompile(`\p{L}\.\.\.\.\p{L}`)

	// give.. us pudding, give ..us pudding, give .. us pudding
	re3 := regexp.MustCompile(`[\s\p{L}]\.\.[\s\p{L}]`)

	// // give .....us some pudding, // give..... us some pudding, // give ..... us some pudding
	re4 := regexp.MustCompile(`[\s\p{L}]\.\.\.\.\.+[\s\p{L}]`)

	// ... us some pudding (start of line)
	re5 := regexp.MustCompile(`^\.`)

	// give ... (end of line)
	re6 := regexp.MustCompile(`[^\.]\.\.\.$`)

	// found in the wild
	re7 := regexp.MustCompile(`\.\s\.+`) // but the cars. ...

	count := 0
	for n, line := range wb {
		if re1.MatchString(line) || re2.MatchString(line) ||
			re3.MatchString(line) || re4.MatchString(line) ||
			re5.MatchString(line) || re6.MatchString(line) ||
			re7.MatchString(line) {
			rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
			count++
		}
	}
	if count == 0 {
		rs = append(rs, "  no ellipsis suspects found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// repeated word check
// works against paragraph buffer
func tcRepeatedWords(pb []string) []string {
	rs := []string{}
	rs = append(rs, "----- repeated word check ----------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for _, para := range pb { // go over each paragraph
		// at least two letter words separated by a space
		re := regexp.MustCompile(`\p{L}\p{L}+ \p{L}\p{L}+`)
		start := 0
		for u := re.FindStringIndex(para[start:]); u != nil; {
			pair := (para[start+u[0] : start+u[1]])
			spair := strings.Split(pair, " ")
			if len(spair) == 2 && spair[0] == spair[1] {
				center := ((start + u[0]) + (start + u[1])) / 2
				t1 := para[:start+u[0]]
				t2 := para[start+u[0] : start+u[1]]
				t3 := para[start+u[1]:]
				tmppara := t1 + "☰" + t2 + "☷" + t3
				s := getParaSegment(tmppara, center)
				rs = append(rs, s)
				count++
			}
			start += u[0] + len(spair[0])
			u = re.FindStringIndex(para[start:])
		}
	}
	if count == 0 {
		rs = append(rs, "  no repeated words found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
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
func tcShortLines(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- short lines check ------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if n == len(wb)-1 {
			break // do not check last line
		}
		if !strings.HasPrefix(line, " ") &&
			utf8.RuneCountInString(line) > 0 &&
			utf8.RuneCountInString(line) <= SHORTEST_PG_LINE &&
			utf8.RuneCountInString(wb[n+1]) > 0 {
			if p.Verbose || count < 5 {
				rs = append(rs, fmt.Sprintf("  %5d: ☰%s☷", n+1, line))  // 1=based
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, wb[n+1]))
				rs = append(rs, "")
			}
			count++
		}
	}
	if !p.Verbose && count > 5 {
		rs = append(rs, fmt.Sprintf("         .... %d more.", count-5))
		rs = append(rs, "")
	}
	if count == 0 {
		rs = append(rs, "  no short lines found in text.")
		rs = append(rs, "")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// all lengths count runes
func tcLongLines(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- long lines check ------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if utf8.RuneCountInString(line) > 72 {
			t := line[60:]
			where := strings.Index(t, " ") // first space after byte 60 (s/b rune-based?)
			rs = append(rs, fmt.Sprintf("  %5d: [%d] %s...", n+1, utf8.RuneCountInString(line), line[:60+where]))  // 1=based
			count++
		}
	}

	if count > 10 {
		rs = rs[:1] // don't show any
		rs = append(rs, fmt.Sprintf("%5d long lines in text. not reporting them.", count))
	}
	if count == 0 {
		rs = append(rs, "  no long lines found in text.")
	}
	if count == 0 || count > 10 {
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

func tcAsteriskCheck(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- asterisk checks --------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if strings.Contains(line, "*") {
			if p.Verbose || count < 5 {
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
			}
			count += 1
		}
	}
	if !p.Verbose && count > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more", count-5))
	}
	if count == 0 {
		rs = append(rs, "  no unexpected asterisks found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// do not report adjacent spaces that start or end a line
func tcAdjacentSpaces(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- adjacent spaces check --------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if strings.Contains(strings.TrimSpace(line), "  ") {
			if p.Verbose || count < 5 {
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
			}
			count += 1
		}
	}
	if !p.Verbose && count > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more", count-5))
	}

	if count == 0 {
		rs = append(rs, "  no adjacent spaces found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

//
func tcTrailingSpaces(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- trailing spaces check ---------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if strings.TrimSuffix(line, " ") != line {
			if p.Verbose || count < 5 {
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
			}
			count += 1
		}
	}
	if !p.Verbose && count > 5 {
		rs = append(rs, fmt.Sprintf("         ... %d more", count-5))
	}
	if count == 0 {
		rs = append(rs, "  no trailing spaces found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

type kv struct {
	Key   rune
	Value int
}

var m = map[rune]int{} // a map for letter frequency counts

// report infrequently-occuring characters (runes)
// threshold set to fewer than 10 occurences
func tcLetterChecks(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- character checks --------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for _, line := range wb { // each line in working buffer
		for _, char := range line { // this gets runes on each line
			m[char] += 1 // for each rune, count how often it occurs
		}
	}
	var ss []kv           // slice of structures (Key, Value pairs)
	for k, v := range m { // load it up
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool { // sort it based on Key (rune)
		return ss[i].Key < ss[j].Key
	})
	// fmt.Println(ss)
	// b := []int{10, int(len(wb) / 25)}
	// sort.Ints(b)
	// kvthres := b[0]
	for _, kv := range ss {
		if strings.ContainsRune(",:;—?!-_0123456789“‘’”. abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", kv.Key) {
			continue
		}
		reportme := false
		if kv.Value < 10 {
			reportme = true
		}
		if reportme {
			reportcount := 0 // count of reports for this particular rune
			rs = append(rs, fmt.Sprintf("%s", strconv.QuoteRune(kv.Key)))
			// rs = append(rs, fmt.Sprintf("%s", kv.Key))
			count += 1
			for n, line := range wb {
				if strings.ContainsRune(line, kv.Key) {
					if p.Verbose || reportcount < 2 {
						line = strings.Replace(line, string(kv.Key), "☰"+string(kv.Key)+"☷", -1)
						rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line))  // 1=based
					}
					reportcount++
				}
			}
			if !p.Verbose && reportcount > 2 {
				rs = append(rs, fmt.Sprintf("    ... %d more", reportcount-2))
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no character checks reported.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// spacing check
// any spacing is okay until the first 4-space gap. Then
// expecting 4-1-2 or 4-2 variations only
func tcSpacingCheck(wb []string) []string {
	rs := []string{}
	s := ""
	rs = append(rs, "----- spacing pattern ---------------------------------------------------------")
	rs = append(rs, "")

	re1 := regexp.MustCompile(`11+1`)
	re2a := regexp.MustCompile(`3`)
	re2b := regexp.MustCompile(`5`)
	re2c := regexp.MustCompile(`22`)
	re2d := regexp.MustCompile(`44`)

	consec := 0 // consecutive blank lines
	lastn := 0  // line number of last paragraph start
	for n, line := range wb {
		if len(strings.TrimSpace(line)) == 0 { // all whitespace
			consec++
			continue
		}
		// a non-blank line
		// if we hit a non-blank line after having seen four or more
		// consecutive blank lines, start a new line of output
		if consec >= 4 {
			// flush any existing line
			s = re1.ReplaceAllString(s, "1..1")
			s = re2a.ReplaceAllString(s, "☰3☷")
			s = re2b.ReplaceAllString(s, "☰5☷")
			s = re2c.ReplaceAllString(s, "☰22☷")
			s = re2d.ReplaceAllString(s, "☰44☷")
			s = fmt.Sprintf("%6d %s", lastn, s)
			rs = append(rs, s)
			s = fmt.Sprintf("%d", consec)
			lastn = n
		} else {
			// we have fewer than four but at least one to report
			if consec > 0 {
				s = fmt.Sprintf("%s%d", s, consec)
			}
		}
		consec = 0 // a non-blank line seen; start count over
	}
	s = re1.ReplaceAllString(s, "1..1")
	s = fmt.Sprintf("%6d %s", lastn, s)
	rs = append(rs, s) // last line in buffer

	// always dim
	rs = append(rs, "")
	// rs[0] = "☲" + rs[0]  // style dim
	// rs[len(rs)-1] += "☷" // close style
	return rs
}

// book-level checks
func tcBookLevel(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- book level checks -----------------------------------------------")
	rs = append(rs, "")
	count := 0

	// check: straight and curly quotes mixed
	if m['\''] > 0 && (m['‘'] > 0 || m['’'] > 0) {
		rs = append(rs, "  both straight and curly single quotes found in text")
		count++
	}

	if m['"'] > 0 && (m['“'] > 0 || m['”'] > 0) {
		rs = append(rs, "  both straight and curly double quotes found in text")
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
		rs = append(rs, "  both \"today\" and \"to-day\" found in text")
		count++
	}
	if ctonight > 0 && ctohnight > 0 {
		rs = append(rs, "  both \"tonight\" and \"to-night\" found in text")
		count++
	}

	// check: American and British title punctuation mixed
	mrpc, mrc, mrspc, mrsc, drpc, drc := false, false, false, false, false, false
	re1 := regexp.MustCompile(`Mr\.`)
	re2 := regexp.MustCompile(`Mr\s`)
	re3 := regexp.MustCompile(`Mrs\.`)
	re4 := regexp.MustCompile(`Mrs\s`)
	re5 := regexp.MustCompile(`Dr\.`)
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
		rs = append(rs, "  both \"Mr.\" and \"Mr\" found in text")
		count++
	}
	if mrspc && mrsc {
		rs = append(rs, "  both \"Mrs.\" and \"Mrs\" found in text")
		count++
	}
	if drpc && drc {
		rs = append(rs, "  both \"Dr.\" and \"Dr\" found in text")
		count++
	}

	// apostrophes and turned commas
	countm1, countm2 := 0, 0
	for n, _ := range wb {
		// check each word separately on this line
		for _, word := range lwl[n] {
			if strings.Contains(word, "M’") {
				countm1++ // with apostrophe
			}
			if strings.Contains(word, "M‘") {
				countm2++ // with turned comma
			}
		}
	}
	if countm1 > 0 && countm2 > 0 {
		rs = append(rs, "  both apostrophes and turned commas appear in text")
		count++
	}

	// check for repeated lines at least 5 characters long.
	limit := len(wb) - 1
	for n, _ := range wb {
		if n == limit {
			break
		}
		if wb[n] == wb[n+1] && len(wb[n]) > 5 {
			rs = append(rs, "  repeated line:")
			rs = append(rs, fmt.Sprintf("%8d,%d: %s", n+1, n+2, wb[n]))  // 1=based
			count++
		}
	}

	// all tests complete

	if count == 0 {
		rs = append(rs, "  no book level checks reported.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// paragraph-level checks
func tcParaLevel() []string {
	rs := []string{}
	rs = append(rs, "----- paragraph level checks -----------------------------------------------")
	rs = append(rs, "")
	count := 0
	const RLIMIT int = 5

	// check: paragraph starts with upper-case word

	re := regexp.MustCompile(`^[“”]?[A-Z][A-Z].*?[a-z]`)
	sscnt := 0

	for _, para := range pbuf { // paragraph buffer
		if re.MatchString(para) {
			if sscnt == 0 {
				rs = append(rs, "  paragraph starts with upper-case word")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, 0))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: full stop (period) with the following word starting with
	// a lower case character.
	// allow exceptions

	re = regexp.MustCompile(`(\p{L}+)\.\s*?[a-z]`)
	exc := []string{"Mr.", "Mrs.", "Dr."}
	sscnt = 0
	for _, para := range pbuf {
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
					rs = append(rs, "  full stop followed by lower case letter")
					count++
				}
				sscnt++
				if sscnt < RLIMIT || p.Verbose {
					rs = append(rs, "    "+getParaSegment(para, lmatch[0]))
				}
			}
		}

	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: query missing paragraph break

	re = regexp.MustCompile(`”\s*“`)
	sscnt = 0
	for _, para := range pbuf {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				rs = append(rs, "  query: missing paragraph break?")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: incorrectly split paragraph

	re = regexp.MustCompile(`^[a-z]`)
	sscnt = 0
	for _, para := range pbuf {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				rs = append(rs, "  incorrectly split paragraph")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: unconverted double-dash/em-dash, long dash error,
	//        or double-emdash broken at end of line
	re = regexp.MustCompile(`(\p{L}--\p{L})|(\p{L}--)|(\p{L}— —)|(\p{L}- -)|(--\p{L})|(—— )|(— )|( —)|(———)`)
	sscnt = 0
	for _, para := range pbuf {
		loc := re.FindStringIndex(para)
		if loc != nil {
			if sscnt == 0 {
				rs = append(rs, "  dash error")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
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
	for _, para := range pbuf {
		lpara := strings.ToLower(para)
		loc := re_hebe.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				rs = append(rs, "  query: he/be. (see also jeebies report)")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	re_hadbad := regexp.MustCompile(HADBADPATTERN)
	sscnt = 0
	for _, para := range pbuf {
		lpara := strings.ToLower(para)
		loc := re_hadbad.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				rs = append(rs, "  query: had/bad")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	re_hutbut := regexp.MustCompile(HUTBUTPATTERN)
	sscnt = 0
	for _, para := range pbuf {
		lpara := strings.ToLower(para)
		loc := re_hutbut.FindAllStringIndex(lpara, -1)
		for _, aloc := range loc {
			if sscnt == 0 {
				rs = append(rs, "  query: hut/but")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    "+getParaSegment(para, aloc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: paragraph endings, punctuation-style aware

	const (
		LEGALAMERICAN = `\.$|:$|\?$|!$|—$|.”|\?”$|’”$|!”$|—”|-----` // include thought-break line all '-'
		LEGALBRITISH  = `\.$|:$|\?$|!$|—$|.’|\?’$|”’$|!’$|—’|-----`
	)

	re_end := regexp.MustCompile(LEGALAMERICAN)
	sscnt = 0
	if puncStyle == "British" {
		re_end = regexp.MustCompile(LEGALBRITISH)
	}

	for _, para := range pbuf {
		if strings.HasPrefix(para, " ") {
			continue // only a normal paragraph
		}
		if !re_end.MatchString(para) {
			if sscnt == 0 {
				rs = append(rs, "  query: unexpected paragraph end")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    ..."+getParaSegment(para, len(para))) // show paragraph end
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	if count == 0 {
		rs = append(rs, "  no paragraph level checks reported.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs
}

// tests extracted from gutcheck that aren't already included
//

func tcGutChecks(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- special situations checks -----------------------------------------------")
	rs = append(rs, "")

	re0000 := regexp.MustCompile(`\[[^IGM\d]`)     // allow Illustration, Greek, Music or number
	re0001 := regexp.MustCompile(`(?i)\bthe[\.\,\?\'\"\;\:\!\@\#\$\^\&\(\)]`) // punctuation after "the"
	re0002 := regexp.MustCompile(`(,\.)|(\.,)|(,,)|([^\.]\.\.[^\.])`)         // double punctuation

	re0003a1 := regexp.MustCompile(`..*?[a-z].*?`) // for mixed case check
	re0003b1 := regexp.MustCompile(`..*?[A-Z].*?`)
	re0003a2 := regexp.MustCompile(`...*?[a-z].*?`)
	re0003b2 := regexp.MustCompile(`...*?[A-Z].*?`)

	re0003c := regexp.MustCompile(`cb|gb|pb|sb|tb|wh|fr|br|qu|tw|gl|fl|sw|gr|sl|cl|iy`) // rare to end word
	re0003d := regexp.MustCompile(`hr|hl|cb|sb|tb|wb|tl|tn|rn|lt|tj`)                   // rare to start word
	re0004 := regexp.MustCompile(`([A-Z])\.([A-Z])`)                                    // initials without space
	re0006 := regexp.MustCompile(`^.$`)                                                 // single character line
	re0007 := regexp.MustCompile(`(\p{L}\- \p{L})|(\p{L} \-\p{L})`)                     // broken hyphenation

	// comma spacing regex
	re0008a := regexp.MustCompile(`[a-zA-Z_],[a-zA-Z_]`) // the,horse
	re0008b := regexp.MustCompile(`[a-zA-Z_],\d`)        // the,1
	re0008c := regexp.MustCompile(`\s,`)                 // space comma
	re0008d := regexp.MustCompile(`^,`)                  // comma start of line

	// full stop spacing regex
	re0009a := regexp.MustCompile(`\.[a-zA-Z]`)
	re0009b := regexp.MustCompile(`[^(Mr)(Mrs)(Dr)]\.\s[a-z]`)

	re0010 := regexp.MustCompile(`,1\d\d\d`)         // Oct. 8,1948 date format
	re0011 := regexp.MustCompile(`I”`)               // “You should runI”
	re0012 := regexp.MustCompile(`\s’(m|ve|ll|t)\b`) // I' ve disjointed contraction
	re0013 := regexp.MustCompile(`Mr,|Mrs,|Dr,`)     // title abbrev.
	re0014 := regexp.MustCompile(`\s[\?!:;]`)        // spaced punctuation
	re0016 := regexp.MustCompile(`<\/?.*?>`)         // HTML tag
	// re0017 := regexp.MustCompile(`([^\.]\.\.\. )|(\.\.\.\.[^\s])|([^\.]\.\.[^\.])|(\.\.\.\.\.+)`)       // ellipsis
	re0018 := regexp.MustCompile(`([\.,;!?’‘]+[‘“])|([A-Za-z]+[“])|([A-LN-Za-z]+[‘])|(“ )|( ”)|(‘s\s)`) // quote direction (context)
	re0019 := regexp.MustCompile(`\b0\b`)                                                               // standalone 0
	re0020a := regexp.MustCompile(`\b1\b`)                                                              // standalone 1
	re0020b := regexp.MustCompile(`\$1\b`)                                                              // standalone 1 allowed after dollar sign
	re0020c := regexp.MustCompile(`1,`)                                                                 // standalone 1 allowed before comma
	re0021 := regexp.MustCompile(`([A-Za-z]\d)|(\d[A-Za-z])`)                                           // mixed alpha and numerals
	re0022 := regexp.MustCompile(`\s$`)                                                                 // trailing spaces/whitespace on line
	re0023 := regexp.MustCompile(`&c([^\.]|$)`)                                                         // abbreviation &c without period
	re0024 := regexp.MustCompile(`^[!;:,.?]`)                                                           // line starts with (selected) punctuation
	re0025 := regexp.MustCompile(`^-[^-]`)                                                              // line starts with hyphen followed by non-hyphen

	re0026 := regexp.MustCompile(`\.+[’”]*\p{L}`) //

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

		if re0026.MatchString(line) {
			gcreports = append(gcreports, reportln{"sentence spacing", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}

		if re0000.MatchString(line) {
			gcreports = append(gcreports, reportln{"opening square bracket followed by other than I, G, M or number", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0001.MatchString(line) {
			gcreports = append(gcreports, reportln{"punctuation after 'the'", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0002.MatchString(line) {
			gcreports = append(gcreports, reportln{"punctuation error", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		// check each word separately on this line
		for _, word := range lwl[n] {

			// check for mixed case within word after the first character,
			// or after the second character if the first char is "’"
			// but not if the word is in the good word list or if it occurs more than once
			reportme := false
			if wordListMap[word] < 2 && !inGoodWordList(word) {
				if strings.HasPrefix(word, "’") {
					if re0003a2.MatchString(word) && re0003b2.MatchString(word) {
						reportme = true
					}
				} else {
					if re0003a1.MatchString(word) && re0003b1.MatchString(word) {
						reportme = true
					}
				}
			}
			if reportme {
				line = strings.Replace(line, word, fmt.Sprintf("☰%s☷", word), -1)
				gcreports = append(gcreports, reportln{"mixed case within word", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
			}

			if len(word) > 2 {
				last2 := word[len(word)-2:]
				if re0003c.MatchString(last2) {
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word ending with %s", last2), fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
				}
				first2 := word[:2]
				if re0003d.MatchString(first2) {
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word starting with %s", first2), fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
				}
			}
		}
		if re0004.MatchString(line) {
			gcreports = append(gcreports, reportln{"initials spacing", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0006.MatchString(line) {
			gcreports = append(gcreports, reportln{"single character line", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0007.MatchString(line) {
			gcreports = append(gcreports, reportln{"broken hyphenation", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0008a.MatchString(line) ||
			re0008b.MatchString(line) ||
			re0008c.MatchString(line) ||
			re0008d.MatchString(line) {
			gcreports = append(gcreports, reportln{"comma spacing", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0009a.MatchString(line) ||
			re0009b.MatchString(line) {
			gcreports = append(gcreports, reportln{"full-stop spacing", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0010.MatchString(line) {
			gcreports = append(gcreports, reportln{"date format", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0011.MatchString(line) {
			gcreports = append(gcreports, reportln{"I/! check", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0012.MatchString(line) {
			gcreports = append(gcreports, reportln{"disjointed contraction", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0013.MatchString(line) {
			gcreports = append(gcreports, reportln{"title abbreviation comma", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0014.MatchString(line) {
			gcreports = append(gcreports, reportln{"spaced punctuation", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0016.MatchString(line) {
			if abandonedTagCount < 10 {
				gcreports = append(gcreports, reportln{"abandoned HTML tag", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
			}
			if abandonedTagCount == 10 {
				gcreports = append(gcreports, reportln{"abandoned HTML tag", fmt.Sprintf("  %5d: %s", 99999, "...more")})
			}
			abandonedTagCount++
		}
		// if re0017.MatchString(line) {
		//  	gcreports = append(gcreports, reportln{"ellipsis check", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		// }
		if re0018.MatchString(line) {
			gcreports = append(gcreports, reportln{"quote error (context)", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0019.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 0", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0020a.MatchString(line) && !re0020b.MatchString(line) && !re0020c.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 1", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0021.MatchString(line) {
			gcreports = append(gcreports, reportln{"mixed letters and numbers in word", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0022.MatchString(line) {
			gcreports = append(gcreports, reportln{"trailing space on line", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0023.MatchString(line) {
			gcreports = append(gcreports, reportln{"abbreviation &c without period", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0024.MatchString(line) {
			gcreports = append(gcreports, reportln{"line starts with suspect punctuation", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0025.MatchString(line) {
			gcreports = append(gcreports, reportln{"line that starts with hyphen and then non-hyphen", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}

		// begin non-regexp based
		if strings.Contains(line, "Blank Page") {
			gcreports = append(gcreports, reportln{"Blank Page placeholder found", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if strings.Contains(line, "—-") || strings.Contains(line, "-—") {
			gcreports = append(gcreports, reportln{"mixed hyphen/dash", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if strings.Contains(line, "\u00A0") {
			gcreports = append(gcreports, reportln{"non-breaking space", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if strings.Contains(line, "\u00AD") {
			gcreports = append(gcreports, reportln{"soft hyphen", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if strings.Contains(line, "\u0009") {
			gcreports = append(gcreports, reportln{"tab character", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if strings.Contains(line, "&") {
			gcreports = append(gcreports, reportln{"ampersand character", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		lcline := strings.ToLower(line)
		if re_comma.MatchString(lcline) {
			gcreports = append(gcreports, reportln{fmt.Sprintf("unexpected comma after word"), fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re_period.MatchString(lcline) {
			gcreports = append(gcreports, reportln{fmt.Sprintf("unexpected period after word"), fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
	}

	sort.Slice(gcreports, func(i, j int) bool { return gcreports[i].sourceline < gcreports[j].sourceline })
	sort.SliceStable(gcreports, func(i, j int) bool { return gcreports[i].rpt < gcreports[j].rpt })

	if abandonedTagCount > 10 {
		rs = append(rs, fmt.Sprintf("note: source file not plain text. %d lines with markup", abandonedTagCount))
	}

	if len(gcreports) > 0 {
		rrpt_last := ""
		ctr := 0 // count this report
		for _, rpt := range gcreports {
			if rpt.rpt != rrpt_last {
				if !p.Verbose && ctr > 5 {
					rs = append(rs, fmt.Sprintf("         ... %d more", ctr-5))
				}
				rs = append(rs, fmt.Sprintf("%s\n%s", rpt.rpt, rpt.sourceline)) // first one gets new header line
				ctr = 1                                                         // one has been reported
			} else {
				s := strings.Replace(rpt.sourceline, "99999: ", "       ", -1)
				if p.Verbose || ctr < 5 {
					rs = append(rs, fmt.Sprintf("%s", s)) // subsequent reports
				}
				ctr++
			}
			rrpt_last = rpt.rpt
		}
	}
	if len(gcreports) == 0 {
		rs = append(rs, "   no special situation reports.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += len(gcreports)
	return rs
}

// text checks
// a series of tests either on the working buffer (line at a time)
// or the paragraph buffer (paragraph at a time)
var tcec int

func textCheck() []string {
	pptr = append(pptr, "☳<a name='texta'></a>")
	rs := []string{} // empty local rs to start aggregation
	tcec = 0
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "TEXT ANALYSIS REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, "")
	rs = append(rs, tcHypConsistency(wbuf)...)
	rs = append(rs, tcHypSpaceConsistency(wbuf, pbuf)...)
	rs = append(rs, tcAsteriskCheck(wbuf)...)
	rs = append(rs, tcAdjacentSpaces(wbuf)...)
	rs = append(rs, tcTrailingSpaces(wbuf)...)
	rs = append(rs, tcLetterChecks(wbuf)...)
	rs = append(rs, tcSpacingCheck(wbuf)...)
	rs = append(rs, tcShortLines(wbuf)...)
	rs = append(rs, tcLongLines(wbuf)...)
	rs = append(rs, tcRepeatedWords(pbuf)...)
	rs = append(rs, tcEllipsisCheck(wbuf)...)
	rs = append(rs, tcDashCheck(pbuf)...)
	rs = append(rs, scannoCheck(wbuf)...)
	rs = append(rs, tcCurlyQuoteCheck(wbuf)...)
	rs = append(rs, tcGutChecks(wbuf)...)
	rs = append(rs, tcBookLevel(wbuf)...)
	rs = append(rs, tcParaLevel()...)
	if tcec == 0 { // test check error count
		rs[0] = "☲" + rs[0] // style dim
	} else { // something was reported
		rs[0] = "☳" + rs[0] // style black
	}
	rs[len(rs)-1] += "☷" // close style
	return rs
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
*/

/*  getWordList
	input: a slice of strings that is the book
    output: a map of words and frequency of occurence of each word
*/

func getWordList(wb []string) map[string]int {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	m := make(map[string]int) // map to hold words, counts

	// hyphenated words "auburn-haired" become "auburn①haired"
	// to preserve that it is one (hyphenated) word.
	// same for single quotes within words and apostrophes starting words
	var re1 = regexp.MustCompile(`(\p{L})\-(\p{L})`)
	var re2 = regexp.MustCompile(`(\p{L})’(\p{L})`)
	var re3 = regexp.MustCompile(`(\p{L})‘(\p{L})`)
	var re4 = regexp.MustCompile(`(\P{L}|^)’(\p{L})`)
	for _, element := range wb {
		// need to preprocess each line
		// need this twice to handle alternates i.e. r-u-d-e
		element := re1.ReplaceAllString(element, `${1}①${2}`)
		element = re1.ReplaceAllString(element, `${1}①${2}`)
		// need this twice to handle alternates i.e. fo’c’s’le
		element = re2.ReplaceAllString(element, `${1}②${2}`)
		element = re2.ReplaceAllString(element, `${1}②${2}`)
		element = re3.ReplaceAllString(element, `${1}③${2}`)
		element = re3.ReplaceAllString(element, `${1}③${2}`)
		element = re4.ReplaceAllString(element, `${1}②${2}`)
		element = re4.ReplaceAllString(element, `${1}②${2}`)
		// all words with special characters are protected
		t := (strings.FieldsFunc(element, f))

		for _, word := range t {
			// put the special characters back in there
			s := strings.Replace(word, "①", "-", -1)
			s = strings.Replace(s, "②", "’", -1)
			s = strings.Replace(s, "③", "‘", -1)
			// and build the map
			if _, ok := m[s]; ok { // if it is there already, increment
				m[s] = m[s] + 1
			} else {
				m[s] = 1
			}
		}
	}
	// fmt.Println(m)
	return m
}

// protect special cases:
// high-flying, hasn't, and 'tis all stay intact
// capitalization is retained

func getWordsOnLine(s string) []string {
	var re1 = regexp.MustCompile(`(\p{L})\-(\p{L})`)
	var re2 = regexp.MustCompile(`(\p{L})’(\p{L})`)
	var re3 = regexp.MustCompile(`(\p{L})‘(\p{L})`)
	var re4 = regexp.MustCompile(`(\P{L}|^)’(\p{L})`)
	s = re1.ReplaceAllString(s, `${1}①${2}`)
	s = re1.ReplaceAllString(s, `${1}①${2}`)
	s = re2.ReplaceAllString(s, `${1}②${2}`)
	s = re2.ReplaceAllString(s, `${1}②${2}`)
	s = re3.ReplaceAllString(s, `${1}③${2}`)
	s = re3.ReplaceAllString(s, `${1}③${2}`)
	s = re4.ReplaceAllString(s, `${1}②${2}`)
	s = re4.ReplaceAllString(s, `${1}②${2}`)

	// all words with special characters are protected
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	t := (strings.FieldsFunc(s, f))

	// put back the protected characters
	for n, _ := range t {
		t[n] = strings.Replace(t[n], "①", "-", -1)
		t[n] = strings.Replace(t[n], "②", "’", -1)
		t[n] = strings.Replace(t[n], "③", "‘", -1)
	}
	return t
}

/* ********************************************************************** */
/*                                                                        */
/* from leven.go                                                          */
/*                                                                        */
/* ********************************************************************** */

func levenshtein(str1, str2 []rune) int {
	s1len := len(str1)
	s2len := len(str2)
	column := make([]int, len(str1)+1)

	for y := 1; y <= s1len; y++ {
		column[y] = y
	}
	for x := 1; x <= s2len; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= s1len; y++ {
			oldkey := column[y]
			var incr int
			if str1[y-1] != str2[x-1] {
				incr = 1
			}

			column[y] = minimum(column[y]+1, column[y-1]+1, lastkey+incr)
			lastkey = oldkey
		}
	}
	return column[s1len]
}

func minimum(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}

// iterate over every suspect word at least six runes long
// case insensitive
// looking for a good word in the text that is "near"
func levencheck(okwords []string, suspects []string) []string {
	rs := []string{} // new empty array of strings for local aggregation
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "LEVENSHTEIN (EDIT DISTANCE) CHECKS"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	// wordsonline is a slice. each "line" contains a map of words on that line
	var wordsonline []map[string]struct{}
	for n, _ := range wbuf { // on each line
		rtn := make(map[string]struct{}) // a set; empty structs take no memory
		for _, word := range lwl[n] {    // get slice of words on the line
			rtn[word] = struct{}{} // empty struct
		}
		wordsonline = append(wordsonline, rtn)
	}

	// lcwordsonline is a slice. each "line" contains a map of downcased words on that line
	var lcwordsonline []map[string]struct{}
	for n, _ := range wbuf { // on each line
		rtn := make(map[string]struct{}) // a set; empty structs take no memory
		for _, word := range lwl[n] {    // get slice of words on the line
			rtn[strings.ToLower(word)] = struct{}{} // empty struct
		}
		lcwordsonline = append(lcwordsonline, rtn)
	}

	nreports := 0
	// for each suspect word, check against all ok words.
	for _, suspect := range suspects {
		suspect_reported := false
		suspectlc := strings.ToLower(suspect)

		for _, okword := range okwords {

			okwordlc := strings.ToLower(okword)

			if suspect_reported { // stop looking if suspect already reported
				break
			}

			// must be five letters or more
			if utf8.RuneCountInString(suspect) < 5 {
				continue
			}

			// only differ by capitalization
			if suspectlc == okwordlc {
				continue
			}

			// differ only by apparent plural
			if suspectlc == okwordlc+"s" || suspectlc+"s" == okwordlc {
				continue
			}

			// calculate distance
			// dist := levenshtein([]rune(suspect), []rune(okword))  // case sensistive
			dist := levenshtein([]rune(suspectlc), []rune(okwordlc))

			if dist < 2 {
				// get counts of suspect word and ok word. case insenstitve
				suspectwordcount := 0
				okwordcount := 0
				for n, _ := range wbuf {
					lcwordsonthisline := lcwordsonline[n]
					if _, ok := lcwordsonthisline[suspectlc]; ok {
						suspectwordcount += 1
					}
					if _, ok := lcwordsonthisline[okwordlc]; ok {
						okwordcount += 1
					}
				}
				rs = append(rs, fmt.Sprintf("%s(%d):%s(%d)", suspectlc, suspectwordcount,
					okwordlc, okwordcount))
				nreports++

				// report only one line with suspect word
				count := 0
				for n, line := range wbuf {
					lcwordsonthisline := lcwordsonline[n]
					if _, ok := lcwordsonthisline[suspectlc]; ok {
						if count == 0 {
							re := regexp.MustCompile(`(?i)(` + suspectlc + `)`)
							line = re.ReplaceAllString(line, `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line)))  // 1=based
						}
						count += 1
					}
				}

				// report one line of ok word
				count = 0
				for n, line := range wbuf {
					lcwordsonthisline := lcwordsonline[n] // a set of words on this line
					if _, ok := lcwordsonthisline[okwordlc]; ok {
						if count == 0 {
							re := regexp.MustCompile(`(?i)(` + okwordlc + `)`)
							line = re.ReplaceAllString(line, `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line)))  // 1=based
						}
						count += 1
					}
				}
				rs = append(rs, "")
				suspect_reported = true // report it once
			}
		}
	}
	if nreports == 0 {
		rs = append(rs, "  no Levenshtein edit distance queries reported")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* from jeebies.go                                                        */
/*                                                                        */
/* ********************************************************************** */

var wbs []string
var wbl []string

func jeebies() []string {
	pptr = append(pptr, "☳<a name='jeebi'></a>")
	rs := []string{} // new empty array of strings for local aggregation
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "JEEBIES REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	wbuf = append(wbuf, "") // ensure last paragraph converts
	s := ""
	// convert each paragraph in the working buffer to a string
	for _, line := range wbuf {
		if line == "" { // blank line so end of paragraph
			if s != "" { // if something in the paragraph string, save it
				s = strings.TrimSpace(s)
				wbs = append(wbs, s)
				wbl = append(wbl, strings.ToLower(s)) // and lower-case version
				s = ""                                // empty string
			}
		} else { // still in the paragraph
			s = s + " " + line
		}
	}

	var scary float64
	var h_count int
	var b_count int
	var reported map[string]int

	reported = make(map[string]int)
	nreports := 0

	// looking for "be" errors
	// search for three-word pattern  "w1 be w2" in lower-case paragraphs
	paranoid_level_3words := 1.0
	p3b := regexp.MustCompile(`([a-z’]+ be [a-z’]+)`)
	for n, para := range wbl {
		t := p3b.FindStringIndex(para)
		if t != nil {
			for ok := true; ok; ok = (t != nil) {
				// have a match
				sstr := (para[t[0]:t[1]])
				para = strings.Replace(para, sstr, "", 1)
				// have a three word form here ("must be taken")
				// see if it is in the beMap map
				b_count = 0
				if val, ok := beMap[sstr]; ok {
					b_count = val
				}
				// change "be" to "he" and see if that is in the heMap map
				sstr2 := strings.Replace(sstr, "be", "he", 1)
				h_count = 0
				if val, ok := heMap[sstr2]; ok {
					h_count = val
				}
				// here I have the "be" form and how common that is in b_count
				// and the "he" form and how common that is in h_count
				// fmt.Printf("%d %s\n%d %s\n\n", b_count, sstr, h_count, sstr2)
				if h_count > 0 && (b_count == 0 || float64(h_count)/float64(b_count) > paranoid_level_3words) {
					// calculate how scary it is.
					// if the "he" form is three times more likely than the "be" form,
					// then scary calculates to 3.0
					if b_count == 0 {
						scary = -1.0
					} else {
						scary = float64(h_count) / float64(b_count)
					}
					where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
					t01 := ""
					if scary != -1 {
						t01 = fmt.Sprintf("%s (%.1f)\n    %s", sstr, scary, getParaSegment(wbs[n], where))
					} else {
						t01 = fmt.Sprintf("%s\n    %s", sstr, getParaSegment(wbs[n], where))
					}
					reported[strings.SplitAfterN(sstr, " ", 2)[1]] = 1
					rs = append(rs, t01)
					nreports++
				}

				// see if there is another candidate
				t = p3b.FindStringIndex(para)
			}

		}
	}

	// looking for "he" errors
	// search for three-word pattern  "w1 he w2" in lower-case paragraphs
	p3b = regexp.MustCompile(`([a-z’]+ he [a-z’]+)`)
	for n, para := range wbl {
		t := p3b.FindStringIndex(para)
		if t != nil {
			for ok := true; ok; ok = (t != nil) {
				// have a match
				sstr := (para[t[0]:t[1]])
				para = strings.Replace(para, sstr, "", 1)
				// have a three word form here ("must he taken")
				// see if it is in the heMap map
				h_count = 0
				if val, ok := heMap[sstr]; ok {
					h_count = val
				}
				// change "he" to "be" and see if that is in the beMap map
				sstr2 := strings.Replace(sstr, "he", "be", 1)
				b_count = 0
				if val, ok := beMap[sstr2]; ok {
					b_count = val
				}
				// here I have the "he" form and how common that is in h_count
				// and the "be" form and how common that is in b_count
				// fmt.Printf("%d %s\n%d %s\n\n", h_count, sstr, b_count, sstr2)
				// if the alternate ("be") form exists, based on paranoid_level
				// compared to the "he" form, report it and include the ratio in favor
				// of the alternate "be" form.
				// if the alternate ("be") form exists and the "he" form does not exist,
				// report it but do not show any ratio
				if b_count > 0 && (h_count == 0 || float64(b_count)/float64(h_count) >= paranoid_level_3words) {
					// calculate how scary it is.
					if h_count == 0 {
						scary = -1.0
					} else {
						scary = float64(b_count) / float64(h_count)
					}
					where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
					t01 := ""
					if scary != -1 {
						t01 = fmt.Sprintf("%s (%.1f)\n    %s", sstr, scary, getParaSegment(wbs[n], where))
					} else {
						t01 = fmt.Sprintf("%s\n    %s", sstr, getParaSegment(wbs[n], where))
					}
					reported[strings.SplitAfterN(sstr, " ", 2)[1]] = 1
					rs = append(rs, t01)
					nreports++
				}

				// see if there is another candidate
				t = p3b.FindStringIndex(para)
			}
		}
	}

	// prettyPrint(reported)

	/*
		// check two word forms.
		// ignore any that have been caught with three word forms by checking 'reported' map

		paranoid_level_2words := 300.0

		// looking for "be" errors
		// search for two-word pattern  " be word2" in lower-case paragraphs
		// "Please be happy for me."
		p3b = regexp.MustCompile(`( be [a-z']+)`) // leading space
		for n, para := range wbl {
			t := p3b.FindStringIndex(para)
			skipreport := false
			if t != nil { // found a " be word2" form
				for ok := true; ok; ok = (t != nil) {
					// have a match
					sstr := (para[t[0]:t[1]])            // " be happy" with leading space
					if _, ok := reported[sstr[1:]]; ok { // already reported?
						skipreport = true
						// fmt.Println(sstr[1:] + " already reported")
					}
					para = strings.Replace(para, sstr, "", 1) // remove this one before scan for another
					// have a two word form here ("be happy")
					// see if it is in the beMap list
					b_count = 0
					if val, ok := beMap[sstr]; ok { // searches the "beMap" map
						b_count = val
					}
					// change "be" to "he" and see if that is in the heMap list
					sstr2 := strings.Replace(sstr, "be", "he", 1) // " he happy"
					h_count = 0
					if val, ok := heMap[sstr2]; ok {
						h_count = val
					}
					// here I have the "be" form and how common that is in b_count
					// and the "he" form and how common that is in h_count
					// fmt.Printf("%d %s\n%d %s\n\n", b_count, sstr, h_count, sstr2)
					if h_count > 0 && (b_count == 0 || float64(h_count)/float64(b_count) > paranoid_level_2words) {
						// calculate how scary it is.
						// if the "he" form is three times more likely than the "be" form,
						// then scary calculates to 3.0
						if b_count == 0 {
							scary = -1.0
						} else {
							scary = float64(h_count) / float64(b_count)
						}
						where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
						t01 := ""
						if scary != -1 {
							t01 = fmt.Sprintf("%s (%.1f)\n    %s", strings.TrimSpace(sstr), scary, getParaSegment(wbs[n], where))
						} else {
							t01 = fmt.Sprintf("%s\n    %s", strings.TrimSpace(sstr), getParaSegment(wbs[n], where))
						}
						if !skipreport {
							rs = append(rs, t01)
						}
					}

					// see if there is another candidate
					t = p3b.FindStringIndex(para)
				}
			}
		}

		// looking for "he" errors
		// search for two-word pattern  " he word2" in lower-case paragraphs
		p3b = regexp.MustCompile(`( he [a-z']+)`) // leading space
		for n, para := range wbl {
			t := p3b.FindStringIndex(para)
			skipreport := false
			if t != nil {
				for ok := true; ok; ok = (t != nil) {
					sstr := (para[t[0]:t[1]])
					if _, ok := reported[sstr[1:]]; ok {
						skipreport = true
						// fmt.Println(sstr[1:] + " already reported")
					}
					para = strings.Replace(para, sstr, "", 1)
					h_count = 0
					if val, ok := heMap[sstr]; ok {
						h_count = val
					}
					// change "he" to "be" and see if that is in the beMap list
					sstr2 := strings.Replace(sstr, "he", "be", 1)
					b_count = 0
					if val, ok := heMap[sstr2]; ok {
						b_count = val
					}
					// here I have the "be" form and how common that is in b_count
					// and the "he" form and how common that is in h_count
					// fmt.Printf("%d %s\n%d %s\n\n", b_count, sstr, h_count, sstr2)
					if b_count > 0 && (h_count == 0 || float64(b_count)/float64(h_count) > paranoid_level_2words) {
						if h_count == 0 {
							scary = -1.0
						} else {
							scary = float64(b_count) / float64(h_count)
						}
						where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
						t01 := ""
						if scary != -1 {
							t01 = fmt.Sprintf("%s (%.1f)\n    %s", strings.TrimSpace(sstr), scary, getParaSegment(wbs[n], where))
						} else {
							t01 = fmt.Sprintf("%s\n    %s", strings.TrimSpace(sstr), getParaSegment(wbs[n], where))
						}
						if !skipreport {
							rs = append(rs, t01)
						}
					}

					// see if there is another candidate
					t = p3b.FindStringIndex(para)
				}
			}
		}
	*/
	if nreports == 0 {
		rs = append(rs, "  no jeebies checks reported.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* from dict.go                                                           */
/*                                                                        */
/* ********************************************************************** */

// dictionary word list
func readDict(infile string) []string {
	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	wd := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		wd = append(wd, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// remove BOM if present
	wd[0] = strings.TrimPrefix(wd[0], BOM)
	return wd
}

// scanno word list in in pptext.dat bracketed by
// *** BEGIN SCANNOS *** and *** END SCANNOS ***
func readScannos(infile string) []string {
	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	swl := []string{} // scanno word list
	scanner := bufio.NewScanner(file)
	keep := false
	for scanner.Scan() {
		if scanner.Text() == "*** BEGIN SCANNOS ***" {
			keep = true
			continue
		}
		if scanner.Text() == "*** END SCANNOS ***" {
			keep = false
			continue
		}
		if keep {
			swl = append(swl, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// remove BOM if present
	swl[0] = strings.TrimPrefix(swl[0], BOM)
	return swl
}

// he word list and be word list in in pptext.dat bracketed by
// *** BEGIN HE *** and *** END HE ***
func readHeBe(infile string) {
	heMap = make(map[string]int)
	beMap = make(map[string]int)

	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanhe := false
	scanbe := false
	for scanner.Scan() {
		if scanner.Text() == "*** BEGIN HE ***" {
			scanhe = true
			continue
		}
		if scanner.Text() == "*** END HE ***" {
			scanhe = false
			continue
		}
		if scanner.Text() == "*** BEGIN BE ***" {
			scanbe = true
			continue
		}
		if scanner.Text() == "*** END BE ***" {
			scanbe = false
			continue
		}
		if scanhe {
			t := strings.Split(scanner.Text(), ":")
			ttmp := strings.Replace(t[0], "|", " ", -1)
			n, _ := strconv.Atoi(t[1])
			heMap[ttmp] = n
		}
		if scanbe {
			t := strings.Split(scanner.Text(), ":")
			ttmp := strings.Replace(t[0], "|", " ", -1)
			n, _ := strconv.Atoi(t[1])
			beMap[ttmp] = n
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

// read in the chosen word list (dictionary)
// convert any straight quote marks to apostrophes

func readWordList(infile string) []string {
	wd := []string{}
	file, err := os.Open(infile) // try to open wordlist
	if err != nil {
		return wd // early exit if it isn't present
	}
	defer file.Close() // here if it opened
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		wd = append(wd, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	// remove BOM if present
	if len(wd) > 0 {
		wd[0] = strings.TrimPrefix(wd[0], BOM) // on first word if there is one
	}
	// "'" to apostrophes
	for i, word := range wd {
		wd[i] = strings.Replace(word, "'", "’", -1)
	}
	return wd
}

func doparams() params {
	p := params{}
	flag.StringVar(&p.Infile, "i", "book-utf8.txt", "input file")
	flag.StringVar(&p.Outfile, "o", "report.txt", "output report file")
	flag.StringVar(&p.Outfileh, "h", "report.html", "output report file (HTML)")
	flag.StringVar(&p.Wlang, "l", "master.en", "wordlist language")
	flag.StringVar(&p.GWFilename, "g", "", "good words file")
	flag.BoolVar(&p.Experimental, "x", false, "experimental (developers only)")
	flag.BoolVar(&p.Nolev, "d", false, "do not run Levenshtein distance tests")
	flag.BoolVar(&p.Nosqc, "q", false, "do not run smart quote checks")
	flag.BoolVar(&p.NoBOM, "noBOM", false, "no BOM on text report")
	flag.BoolVar(&p.UseLF, "useLF", false, "LF line endings on report")
	flag.BoolVar(&p.Verbose, "v", false, "Verbose: show all reports")
	flag.Parse()
	return p
}

func main() {
	loc, _ := time.LoadLocation("America/Denver")
	start := time.Now()
	pptr = append(pptr, fmt.Sprintf("********************************************************************************"))
	pptr = append(pptr, fmt.Sprintf("* %-76s *", "PPTEXT RUN REPORT"))
	pptr = append(pptr, fmt.Sprintf("* %76s *", "started "+time.Now().In(loc).Format(time.RFC850)))
	pptr = append(pptr, fmt.Sprintf("********************************************************************************"))
	pptr = append(pptr, fmt.Sprintf("☲pptext version: %s", VERSION))

	p = doparams() // parse command line parameters

	/*************************************************************************/
	/* working buffer (saved in models)                                      */
	/* user-supplied source file UTF-8 encoded                               */
	/*************************************************************************/

	wbuf = readText(p.Infile) // working buffer, line by line

	// location of executable and user's working directory
	execut, _ := os.Executable()
	loc_exec := filepath.Dir(execut) // i.e. /home/rfrank/go/src/pptext
	loc_proj, _ := os.Getwd()        // i.e. /home/rfrank/projects/books/hiking-westward

	// if executable is on rfrank.io server, do not expose internal directories
	if !strings.Contains(loc_exec, "www") {
		pptr = append(pptr, fmt.Sprintf("command line: %s", os.Args))
		pptr = append(pptr, fmt.Sprintf("executable is in: %s", loc_exec))
		pptr = append(pptr, fmt.Sprintf("project is in: %s", loc_proj))
	} else {
		_, file := filepath.Split(p.Infile)
		pptr = append(pptr, fmt.Sprintf("processing file: %s", file))
	}
	// report status of verbose flag.
	if p.Verbose {
		pptr = append(pptr, fmt.Sprintf("verbose mode: %s", "on"))
	} else {
		pptr = append(pptr, fmt.Sprintf("verbose mode: %s", "off"))
	}

	/*************************************************************************/
	/* working dictionary (wdic)                                             */
	/* create from words in dictionary in the language specified             */
	/* and words from optional project-specific good_words.txt file          */
	/* result are all known good words in a sorted list                      */
	/*************************************************************************/

	// language-specific wordlists are in a subdirectory of the executable

	// if the user has used the -w option, a language file has been specified
	// otherwise accept default
	where := filepath.Join(loc_exec, "/wordlists/"+p.Wlang+".txt")
	// fmt.Println(where)
	if _, err := os.Stat(where); !os.IsNotExist(err) {
		// it exists
		wdic = readDict(where)
		pptr = append(pptr, fmt.Sprintf("using wordlist: %s (%d words)", p.Wlang, len(wdic)))
	} else {
		pptr = append(pptr, fmt.Sprintf("no dictionary present"))
	}

	// require a pptext.dat file holding scannos list and jeebies he/be lists
	scannoWordlist = readScannos(filepath.Join(loc_exec, "pptext.dat"))
	readHeBe(filepath.Join(loc_exec, "pptext.dat"))

	// now the good word list.
	// good words may include curly quotes. convert to straight quotes
	// before merging into dictionary
	if len(p.GWFilename) > 0 { // a good word list was specified
		if _, err := os.Stat(p.GWFilename); !os.IsNotExist(err) { // it exists
			_, file := filepath.Split(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good words file: %s", file))
			goodWordlist = readWordList(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good word count: %d words", len(goodWordlist)))
			for i, w := range goodWordlist {
				goodWordlist[i] = strings.Replace(w, "'", "’", -1)
			}
			wdic = append(wdic, goodWordlist...) // add good_words into dictionary
		} else { // it does not exist
			pptr = append(pptr, fmt.Sprintf("no %s found", p.GWFilename))
		}
	} else {
		pptr = append(pptr, "no good words file specified")
	}

	// need the words in a sorted list for binary search later
	sort.Strings(wdic)

	/*************************************************************************/
	/* line word list                                                        */
	/* slice of words on each line of text file (capitalization retained)    */
	/*************************************************************************/

	for _, line := range wbuf {
		lwl = append(lwl, getWordsOnLine(line))
	}

	/*************************************************************************/
	/* word list map to frequency of occurrence                              */
	/* capitalization retained; hyphens, apostrophes protected               */
	/*************************************************************************/

	wordListMap = getWordList(wbuf)
	// fmt.Println(wordListMap)

	/*************************************************************************/
	/* paragraph buffer (pb)                                                 */
	/* the user source file one paragraph per line                           */
	/*************************************************************************/

	var cp string // current (in progress) paragraph
	for _, element := range wbuf {
		// if this is a blank line and there is a paragraph in progress, save it
		// if not a blank line, put it into the current paragraph
		if element == "" {
			if len(cp) > 0 {
				pbuf = append(pbuf, cp) // save this paragraph
				cp = cp[:0]             // empty the current paragraph buffer
			}
		} else {
			if len(cp) == 0 {
				cp += element
			} else {
				cp = cp + " " + element
			}
		}
	}
	// finished processing all lines in the file
	// flush possible non-empty current paragraph buffer
	if len(cp) > 0 {
		pbuf = append(pbuf, cp) // save this paragraph
	}
	pptr = append(pptr, fmt.Sprintf("paragraphs: %d", len(pbuf)))

	puncStyle = getPuncStyle()
	pptr = append(pptr, fmt.Sprintf("punctuation style: %s☷", puncStyle))
	pptr = append(pptr, "")

	pptr = append(pptr, "☳reports: <a href='#punct'>punctuation scan</a> | <a href='#spell'>spellcheck</a> | <a href='#leven'>edit distance</a> | <a href='#texta'>text analysis</a> | <a href='#jeebi'>jeebies</a>")

	/*************************************************************************/
	/* begin individual tests                                                */
	/*************************************************************************/

	// smart quote check
	pptr = append(pptr, "☳<a name='punct'></a>")

	if p.Nosqc || puncStyle == "British" {
		pptr = append(pptr, "☲********************************************************************************")
		pptr = append(pptr, "* PUNCTUATION SCAN                                                             *")
		pptr = append(pptr, "********************************************************************************")
		pptr = append(pptr, "")
		if p.Nosqc {
			pptr = append(pptr, "Curly-quote scan checks skipped☷")
		} else {
			pptr = append(pptr, "Curly-quote scan checks skipped (British-style quotes)☷")
		}
	} else {
		// t1 := time.Now()
		t := puncScan()
		// t2 := time.Now()
		pptr = append(pptr, t...)
		// pptr = append(pptr, fmt.Sprintf("smart quote check took: %.2f seconds", t2.Sub(t1).Seconds()))
	}

	// spellcheck
	// returns list of suspect words, ok words used in text
	sw, okwords, rsx := spellCheck(wdic)
	pptr = append(pptr, rsx...)

	// levenshtein check
	// compares all suspect words to all okwords in text
	pptr = append(pptr, "☳<a name='leven'></a>")
	if p.Nolev {
		pptr = append(pptr, "")
		pptr = append(pptr, "********************************************************************************")
		pptr = append(pptr, "* LEVENSHTEIN (EDIT DISTANCE) CHECKS                                           *")
		pptr = append(pptr, "********************************************************************************")
		pptr = append(pptr, "")
		pptr = append(pptr, "Levenshtein (edit-distance) checks skipped")
	} else {
		// t1 := time.Now()
		t := levencheck(okwords, sw)
		pptr = append(pptr, t...)
		// t2 := time.Now()
		// pptr = append(pptr, fmt.Sprintf("Levenshtein (edit-distance) checks took %.2f seconds", t2.Sub(t1).Seconds()))
	}

	// text check
	//
	t := textCheck()
	pptr = append(pptr, t...)

	// jeebies looks for he/be errors
	t = jeebies()
	pptr = append(pptr, t...)

	// note: remaining words in sw are suspects.
	// they could be used to start a user-maintained persistent good word list

	/*************************************************************************/
	/* all tests complete. save results to specified report file             */
	/*************************************************************************/

	pptr = append(pptr, "--------------------------------------------------------------------------------")
	pptr = append(pptr, "run complete")
	// pptr = append(pptr, fmt.Sprintf("execution time: %s", time.Since(start)))
	t2 := time.Now()
	pptr = append(pptr, fmt.Sprintf("execution time: %.2f seconds", t2.Sub(start).Seconds()))

	saveText(pptr, p.Outfile, p.NoBOM, p.UseLF)
	saveHtml(pptr, p.Outfileh, p.NoBOM, p.UseLF)
}
