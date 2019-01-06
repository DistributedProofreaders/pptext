/*
filename:  pptext.go
author:    Roger Frank
license:   GPL
status:    development
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

const VERSION string = "2019.01.05"

var sw []string      // suspect words list

var rs []string      // array of strings for local aggregation

var pptr []string 	// pptext report

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
		r, size := utf8.DecodeRuneInString(s)  // first
		runecount++
		// replace space with newline (break on space)
		if runecount >= 70 && r == ' ' {
			s2 += "\n         "
			runecount = 0
		} else {
			s2 += string(r)  // append single rune to string
		}
		s = s[size:]  // chop off rune
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
	Outfileh	 string
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
var goodWordlist []string // good word list specified by user

/* ********************************************************************** */
/*                                                                        */
/* from fileio.go                                                         */
/*                                                                        */
/* ********************************************************************** */

var BOM = string([]byte{239, 187, 191}) // UTF-8 Byte Order Mark

// readLn returns a single line (without the ending \n)
// from the input buffered reader.
// An error is returned iff there is an error with the
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
	f2, err := os.Create(outfile)  // specified report file for text output
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
			re := regexp.MustCompile(`[☰☱☲☳☴☵☶☷]`)  // strip any tokens for HTML
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
	f2, err := os.Create(outfile)  // specified report file for text output
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
	// ☳
	// ☴
	// ☵
	// ☶
	// ☷ </style>
	for _, line := range a {
		s := strings.Replace(line, "\n", "\r\n", -1)
		s = strings.Replace(s, "☰", "<span class='red'>", -1)
		s = strings.Replace(s, "☱", "<span class='green'>", -1)
		s = strings.Replace(s, "☲", "<span class='dim'>", -1)
		s = strings.Replace(s, "☳", "<span class='black'>", -1)
		s = strings.Replace(s, "☷", "</span>", -1)
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
			ip := sort.SearchStrings(wdic, testwordlc)   // where it would insert
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
				query = append(query, fmt.Sprintf("query: unclosed paragraph"))
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
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
		rs = append(rs, "  ☰mixed straight and curly quotes.☷ punctuation scan not done☷")
		rs = append(rs, "")	
		return rs
	}
	pluralPossessive() // horses’ becomes horses◳
	internals()        // protect internal '’' characters as in contractions
	commonforms()      // protect known common forms
	ingResolve()       // resolve -ing words that are contracted
	wordQuotes()       // apparent quoted words or two-word phrases
	properNames()      // protect proper names
	t := doScan()           // scan quotes and report, returns []string
	rs = append(rs, t...)
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* from spellcheck.go                                                     */
/*                                                                        */
/* ********************************************************************** */

func lookup(wd []string, word string) bool {
	ip := sort.SearchStrings(wd, word)       // where it would insert
	return (ip != len(wd) && wd[ip] == word) // true if we found it
}

// spellcheck returns list of suspect words, list of ok words in text

func spellCheck(wd []string) ([]string, []string, []string) {
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
		swd := strings.Replace(k, "’", "'", -1) // apostrophes straight in pptext.dat
		wlmLocal[swd] = v
	}

	rs = append(rs, fmt.Sprintf("  ☲unique words in text: %d words", len(wlmLocal)))

	for word, count := range wlmLocal {
		ip := sort.SearchStrings(wd, word)   // where it would insert
		if ip != len(wd) && wd[ip] == word { // true if we found it
			// ok by wordlist
			okwordlist[word] = count // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	rs = append(rs, fmt.Sprintf("  approved by dictionary: %d words", len(willdelete)))

	// fmt.Printf("%+v\n", willdelete)
	// os.Exit(1)

	// delete words that have been OKd by being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(okwordlist))

	// typically at this point, I have taken the 8995 unique words in the book
	// and sorted it into 7691 words found in the dictionary and 1376 words unresolved

	// fmt.Printf("%+v\n", wlmLocal)
	// try to approve words that are capitalized by testing them lower case

	// check words ok by depossessive
	re := regexp.MustCompile(`'s$`)
	for word, count := range wlmLocal {
		if re.MatchString(word) {
			testword := word[:len(word)-2]
			ip := sort.SearchStrings(wd, testword)   // where it would insert
			if ip != len(wd) && wd[ip] == testword { // true if we found it
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	rs = append(rs, fmt.Sprintf("  approved by depossessive: %d words", len(willdelete)))

	// delete words that have been OKd by their lowercase form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

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

	rs = append(rs, fmt.Sprintf("  approved by lowercase form: %d words", len(willdelete)))

	// delete words that have been OKd by their lowercase form being in the dictionary
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Printf("%+v\n", wlmLocal)
	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(lcwordlist))

	// typically at this point the 1376 unresolved words are now 638 that were approved b/c their
	// lowercase form is in the dictionary and 738 words still unresolved

	// some of these are hyphenated. Break those words on hyphens and see if all the individual parts
	// are valid words. If so, approve the hyphenated version

	hywordlist := make(map[string]int) // hyphenated words OK by all parts being words

	for word, count := range wlmLocal {
		t := strings.Split(word, "-")
		if len(t) > 1 {
			// we have a hyphenated word
			allgood := true
			for _, hpart := range t {
				if !lookup(wd, hpart) {
					allgood = false
				}
			}
			if allgood { // all parts of the hyhenated word are words
				hywordlist[word] = count
				okwordlist[word] = count // remember as good word
				willdelete = append(willdelete, word)
			}
		}
	}

	rs = append(rs, fmt.Sprintf("  approved by dehyphenation: %d words", len(willdelete)))

	// delete words that have been OKd by dehyphenation
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Println(len(wlmLocal))
	// fmt.Println(len(hywordlist))

	// of the 738 unresolved words before dehyphenation checks, now an additional
	// 235 have been approved and 503 remain unresolved

	// some "words" are entirely numerals. approve those
	for word, _ := range wlmLocal {
		if _, err := strconv.Atoi(word); err == nil {
			okwordlist[word] = 1 // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	rs = append(rs, fmt.Sprintf("  approved pure numerics: %d words", len(willdelete)))
	// delete words that are entirely numeric
	for _, word := range willdelete {
		delete(wlmLocal, word)
	}
	willdelete = nil // clear the list of words to delete

	// the 503 unresolved words are now 381 with the removal of the all-numeral words

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

	rs = append(rs, fmt.Sprintf("  approved by frequency: %d words", len(willdelete)))
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
	sort.Strings(keys)

	// show each word in context
	var sw []string // suspect words
	rs = append(rs, "--------------------------------------------------------------------------------☷")
	rs = append(rs, fmt.Sprintf("Suspect words"))
	for _, word := range keys {
		word = strings.Replace(word, "'", "’", -1)
		sw = append(sw, word)                    // simple slice of only the word
		rs = append(rs, fmt.Sprintf("%s", word)) // word we will show in context
		// show word in text
		for n, line := range wbuf {  // every line
			for _, t2 := range lwl[n] {  // every word on that line
				if t2 == word {  // it is here
					re := regexp.MustCompile(`(`+word+`)`)
					line = re.ReplaceAllString(line, `☰$1☷`)
					re = regexp.MustCompile(`☰`)
					loc := re.FindStringIndex(line)
					line = getParaSegment(line, loc[0])
					rs = append(rs, fmt.Sprintf("  %6d: %s", n, line))
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
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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

func useStart(s string, rps []rp) string {
	zrs := ""
	if len(rps) <= 60 {
		zrs = s
	} else {
		t := 60
		for {
			if t == 0 || rps[t].rpr == ' ' {
				break
			}
			t--
		}
		if t == 0 { // never found a space
			t = 60 // reset to 60 runes
		}
		zrs = strings.TrimSpace(s[0:rps[t].rpp])
	}
	return zrs
}

func useEnd(s string, rps []rp) string {
	zrs := ""
	if len(rps) < 60 { // if 60 runes, keep them all
		zrs = s
	} else {
		t := len(rps) - 60 // start looking 60 runes from the end
		for {
			if rps[t].rpr == ' ' || t == len(rps)-1 {
				break
			}
			t++
		}
		if t == len(rps)-1 { // there were no spaces
			t = len(rps) - 60 // reset 60 spaces back
		}
		zrs = strings.TrimSpace(s[rps[t].rpp:len(s)])
	}
	return zrs
}

func getParaSegment(s string, where int) string {
	// convert paragraph to slice of {runes, positions}
	rps := make([]rp, 0)
	zrs := "" // return string
	for i, w := 0, 0; i < len(s); i += w {
		runeValue, width := utf8.DecodeRuneInString(s[i:])
		// fmt.Printf("%#U starts at byte position %d\n", runeValue, i)
		rps = append(rps, rp{rpr: runeValue, rpp: i})
		w = width
	}
	if where == -1 { // show paragraph end
		zrs = useEnd(s, rps)
	}
	if where == 0 { // show paragraph start
		zrs = useStart(s, rps)
	}
	if zrs == "" { // we are given a center point as a byte position
		crp := 0 // center run position as index into rps
		for i := 0; i < len(rps); i++ {
			if rps[i].rpp == where {
				crp = i
			}
		}
		if crp-30 < 0 {
			zrs = useStart(s, rps)
		}
		if zrs == "" && crp+30 >= len(rps) {
			zrs = useEnd(s, rps)
		}
		if zrs == "" {
			sleft := crp - 30
			for {
				if rps[sleft].rpr == ' ' {
					break
				}
				sleft++
			}
			sright := crp + 30
			for {
				if rps[sright].rpr == ' ' {
					break
				}
				sright--
			}
			zrs = strings.TrimSpace(s[rps[sleft].rpp:rps[sright].rpp])
		}
	}
	return zrs
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

//
func tcHypConsistency(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- hyphenation consistency check -------------------------------------------")
	rs = append(rs, "")

	count := 0
	for s, _ := range wordListMap {
		if strings.Contains(s, "-") {
			// hyphenated version present. check without
			s2 := strings.Replace(s, "-", "", -1)
			for t, _ := range wordListMap {
				if t == s2 {
					count++
					rs = append(rs, fmt.Sprintf("%s (%d) <-> %s (%d)", s2, wordListMap[s2], s, wordListMap[s]))
					sdone, s2done := false, false
					for n, line := range wb {
						re1 := regexp.MustCompile(`(\P{L}`+s+`\P{L})`)
						if !sdone && re1.MatchString(line) {
							line = re1.ReplaceAllString(line, `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n, line))
							sdone = true
						}
						re2 := regexp.MustCompile(`(\P{L}`+s2+`\P{L})`)
						if !s2done && re2.MatchString(line) {
							line = re2.ReplaceAllString(line, `☰$1☷`)							
							rs = append(rs, fmt.Sprintf("%6d: %s", n, line))
							s2done = true	
						}
						if sdone && s2done {
							break
						}
					}
				}
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no hyphenation inconsistencies found.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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

	count := 0
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
			rs = append(rs, fmt.Sprintf("%6d: %s", n, wraptext9(line)))
			count++
		}
	}

	ast = 0
	r1a := regexp.MustCompile(`[\.,;!?]+[‘“]`)
	r1b := regexp.MustCompile(`[A-Za-z]+[‘“]`)
	for n, line := range wb {
		if r1a.MatchString(line) || r1b.MatchString(line) {
			if ast == 0 {
				rs = append(rs, fmt.Sprintf("%s", "quote direction"))
				ast++
			}
			rs = append(rs, fmt.Sprintf("%6d: %s", n, wraptext9(line)))
			count++
		}
	}

	if count == 0 {
		rs = append(rs, "  no curly quote (context) suspects found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style	
	tcec += count
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
						ast++
					}

					line := wb[n]
					re := regexp.MustCompile(`(`+word+`)`)
					line = re.ReplaceAllString(line, `☰$1☷`)
					re = regexp.MustCompile(`☰`)
					loc := re.FindStringIndex(line)
					line = getParaSegment(line, loc[0])

					rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
					count++
				}
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no suspected scannos found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
				s := getParaSegment(para, u[0])
				rs = append(rs, fmt.Sprintf("   [%3s] %s", para[u[0]:u[1]], s))
				count++
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no dash check suspects found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
			rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
			count++
		}
	}
	if count == 0 {
		rs = append(rs, "  no ellipsis suspects found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
	for _, line := range pb {
		t := getWordsOnLine(line)
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
					rs = append(rs, fmt.Sprintf("    [%s] %s", t[n], s[ltrim:rtrim]))
					count++
				}
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no repeated words found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
			if count <= 10 {
				rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
			}
			count++
		}
	}
	if count > 5 {
		rs = append(rs, fmt.Sprintf("  ....%5d more.", count-5))
	}
	if count == 0 {
		rs = append(rs, "  no short lines found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
	}
	rs = append(rs, "")
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
			rs = append(rs, fmt.Sprintf("  %5d: [%d] %s...", n, utf8.RuneCountInString(line), line[:60+where]))
			count++
		}
	}

	if count > 10 {
		rs = rs[:1]  // don't show any
		rs = append(rs, fmt.Sprintf("%5d long lines in text. not reporting them.", count))
	}
	if count == 0 {
		rs = append(rs, "  no long lines found in text.")
	}
	if count == 0 || count > 10 {
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
			rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
			count += 1
		}
	}
	if count == 0 {
		rs = append(rs, "  no unexpected asterisks found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
			if count < 10 {
				rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
			}
			if count == 10 {
				rs = append(rs, fmt.Sprintf("    ...more"))
			}
			count += 1
		}
	}
	if count == 0 {
		rs = append(rs, "  no adjacent spaces found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
			rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
			count += 1
		}
	}
	if count == 0 {
		rs = append(rs, "  no trailing spaces found in text.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
// threshold set to fewer than 10 occurences or fewer than #lines / 100
// do not report numbers
func tcLetterChecks(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- character checks --------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for _, line := range wb {  // each line in working buffer
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
	b := []int{10, int(len(wb) / 25)}
	sort.Ints(b)
	kvthres := b[0]
	for _, kv := range ss {
		if strings.ContainsRune(",:;—?!-_0123456789“‘’”. abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", kv.Key) {
			continue
		}
		reportme := false
		if kv.Value < kvthres && (kv.Key < '0' || kv.Key > '9') {
			reportme = true
		}
		if reportme {
			reportcount := 0
			rs = append(rs, fmt.Sprintf("%s", strconv.QuoteRune(kv.Key)))
			// rs = append(rs, fmt.Sprintf("%s", kv.Key))
			count += 1
			count += 1
			for n, line := range wb {
				if strings.ContainsRune(line, kv.Key) {
					if reportcount < 10 {
						rs = append(rs, fmt.Sprintf("  %5d: %s", n, line))
					}
					if reportcount == 10 {
						rs = append(rs, fmt.Sprintf("    ...more"))
					}
					reportcount++
				}
			}
		}
	}
	if count == 0 {
		rs = append(rs, "  no character checks reported.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
	count := 0
	rs = append(rs, "----- spacing pattern ---------------------------------------------------------")
	rs = append(rs, "")

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
					rs = append(rs, pbuf)
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
	rs = append(rs, s)
	/*
		if count > 0 {
			rs = append(rs, "  spacing anomalies reported.")
		} else {
			rs = append(rs, "  no spacing anomalies reported.")
		}
	*/
	
	// always dim
	rs = append(rs, "")
	rs[0] = "☲" + rs[0]  // style dim
	rs[len(rs)-1] += "☷" // close style
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
	if count == 0 {
		rs = append(rs, "  no book level checks reported.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
				rs = append(rs, "    " + getParaSegment(para, 0))
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
					rs = append(rs, "    " + getParaSegment(para, lmatch[0]))
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
				rs = append(rs, "    " + getParaSegment(para, loc[0]))
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
				rs = append(rs, "    " + getParaSegment(para, loc[0]))
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	// ------------------------------------------------------------------------
	// check: unconverted double-dash/em-dash, long dash error,
	//        or double-emdash broken at end of line
	re = regexp.MustCompile(`(\w--\w)|(\w--)|(\w— —)|(\w- -)|(--\w)|(—— )|(— )|( —)|(———)`)
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
				rs = append(rs, "    " + getParaSegment(para, loc[0]))
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
				rs = append(rs, "    " + getParaSegment(para, aloc[0]))
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
				rs = append(rs, "    " + getParaSegment(para, aloc[0]))
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
				rs = append(rs, "    " + getParaSegment(para, aloc[0]))
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
				rs = append(rs, "    ..." + getParaSegment(para, -1)) // show paragraph end
			}
		}
	}
	if sscnt > RLIMIT && !p.Verbose {
		rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	}

	if count == 0 {
		rs = append(rs, "  no paragraph level checks reported.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style
	tcec += count
	return rs	
}

// tests extracted from gutcheck that aren't already included
func tcGutChecks(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- special situations checks -----------------------------------------------")
	rs = append(rs, "")

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
			gcreports = append(gcreports, reportln{"opening square bracket followed by other than I, G or number", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
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
			// but not if the word is in the good word list or if it occurs more than once
			if wordListMap[word] < 2 && !inGoodWordList(word) && re0003a.MatchString(word[1:]) && re0003b.MatchString(word[1:]) {
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
			abandonedTagCount++
		}
		if re0017.MatchString(line) {
			gcreports = append(gcreports, reportln{"ellipsis check", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0018.MatchString(line) {
			gcreports = append(gcreports, reportln{"quote error (context)", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0019.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 0", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0020a.MatchString(line) && !re0020b.MatchString(line) {
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
		for _, rpt := range gcreports {
			if rpt.rpt != rrpt_last {
				rs = append(rs, fmt.Sprintf("%s\n%s", rpt.rpt, rpt.sourceline))
			} else {
				rs = append(rs, fmt.Sprintf("%s", rpt.sourceline))
			}
			rrpt_last = rpt.rpt
		}
	}
	if len(gcreports) == 0 {
		rs = append(rs, "   no special situation reports.")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
	rs := []string{} // empty local rs to start aggregation
	tcec = 0
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "TEXT ANALYSIS REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))
	rs = append(rs, "")
	rs = append(rs, tcHypConsistency(wbuf)...)
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
	if tcec == 0 {  // test check error count
		rs[0] = "☲" + rs[0]  // style dim
	} else { // something was reported
		rs[0] = "☳" + rs[0]  // style black
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

Checks to do:
    m1 = re.search(r"^,", line)  # no comma starts a line
    m2 = re.search(r"\s,", line)  # no space before a comma
    m3 = re.search(r"\s,\s", line)  # no floating comma
    m4 = re.search(r",\w", line)  # always a space after a comma unless a digit

*/

/* ********************************************************************** */
/*                                                                        */
/* from wfreq.go                                                          */
/*                                                                        */
/* ********************************************************************** */

/*  input: a slice of strings that is the book
    output:
        1. a map of words and frequency of occurence of each word
        2. a slice with each element 1:1 with the lines of the text
           containing a set, per line, of words on that line
*/

func getWordList(wb []string) map[string]int {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	m := make(map[string]int) // map to hold words, counts

	// hyphenated words "auburn-haired" become "auburn①haired"
	// to preserve that it is one (hyphenated) word.
	// same for single quotes within words
	var re1 = regexp.MustCompile(`(\w)\-(\w)`)
	var re2 = regexp.MustCompile(`(\w)’(\w)`)
	var re3 = regexp.MustCompile(`(\w)‘(\w)`) // rare: example "M‘Donnell"
	for _, element := range wb {
		// need to preprocess each line
		// retain [-‘’] between letters
		// need this twice to handle alternates i.e. r-u-d-e
		element := re1.ReplaceAllString(element, `${1}①${2}`)
		element = re1.ReplaceAllString(element, `${1}①${2}`)
		// need this twice to handle alternates i.e. fo’c’s’le
		element = re2.ReplaceAllString(element, `${1}②${2}`)
		element = re2.ReplaceAllString(element, `${1}②${2}`)
		element = re3.ReplaceAllString(element, `${1}③${2}`)
		element = re3.ReplaceAllString(element, `${1}③${2}`)
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
	return m
}

func getWordsOnLine(s string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	var re1 = regexp.MustCompile(`(\w)\-(\w)`)
	var re2 = regexp.MustCompile(`(\w)’(\w)`)
	var re3 = regexp.MustCompile(`(\w)‘(\w)`)
	s = re1.ReplaceAllString(s, `${1}①${2}`)
	s = re1.ReplaceAllString(s, `${1}①${2}`)
	s = re2.ReplaceAllString(s, `${1}②${2}`)
	s = re2.ReplaceAllString(s, `${1}②${2}`)
	s = re3.ReplaceAllString(s, `${1}③${2}`)
	s = re3.ReplaceAllString(s, `${1}③${2}`)
	// all words with special characters are protected
	t := (strings.FieldsFunc(s, f))
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

	// m2 is a slice. each "line" contains a map of words on that line
	var m2 []map[string]struct{}
	for n, _ := range wbuf { // on each line
		rtn := make(map[string]struct{}) // a set; empty structs take no memory
		for _, word := range lwl[n] {    // get slice of words on the line
			rtn[word] = struct{}{} // empty struct
		}
		m2 = append(m2, rtn)
	}

	nreports := 0
	for _, suspect := range suspects {
		for _, okword := range okwords {
			if utf8.RuneCountInString(suspect) < 5 {
				continue
			}
			if strings.ToLower(suspect) == strings.ToLower(okword) {
				continue
			}
			// differ only by apparent plural
			if strings.ToLower(suspect) == strings.ToLower(okword+"s") {
				continue
			}
			if strings.ToLower(suspect+"s") == strings.ToLower(okword) {
				continue
			}
			dist := levenshtein([]rune(suspect), []rune(okword))
			if dist < 2 {
				// get counts
				suspectwordcount := 0
				okwordcount := 0
				for n, _ := range wbuf {
					wordsonthisline := m2[n] // a set of words on this line
					if _, ok := wordsonthisline[suspect]; ok {
						suspectwordcount += 1
					}
					if _, ok := wordsonthisline[okword]; ok {
						okwordcount += 1
					}
				}
				rs = append(rs, fmt.Sprintf("%s(%d):%s(%d)", suspect, suspectwordcount, okword, okwordcount))
				nreports++

				// show one line in context
				count := 0
				for n, line := range wbuf {
					wordsonthisline := m2[n] // a set of words on this line
					if _, ok := wordsonthisline[suspect]; ok {
						if count == 0 {
							re := regexp.MustCompile(`(`+suspect+`)`)
							line = re.ReplaceAllString(line, `☰$1☷`)
							rs = append(rs, fmt.Sprintf("%6d: %s", n, wraptext9(line)))
						}
						count += 1
					}
				}
				count = 0
				for n, line := range wbuf {
					wordsonthisline := m2[n] // a set of words on this line
					if _, ok := wordsonthisline[okword]; ok {
						if count == 0 {
							re := regexp.MustCompile(`(`+okword+`)`)
							line = re.ReplaceAllString(line, `☰$1☷`)							
							rs = append(rs, fmt.Sprintf("%6d: %s", n, wraptext9(line)))
						}
						count += 1
					}
				}
			}
		}
	}
	if nreports == 0 {
		rs = append(rs, "  no Levenshtein edit distance queries reported")
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
	p3b := regexp.MustCompile(`([a-z']+ be [a-z']+)`)
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
	p3b = regexp.MustCompile(`([a-z']+ he [a-z']+)`)
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
		rs[0] = "☲" + rs[0]  // style dim
	} else {
		rs[0] = "☳" + rs[0]  // style black
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
	/*
		do not report. work to do with verbose flag. some things should always be
		"all reports" and others could be truncated.
		if p.Verbose {
			pptr = append(pptr, fmt.Sprintf("verbose flag: %s", "on"))
		}
	*/

	/*************************************************************************/
	/* working dictionary (wdic)                                        */
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
	// by default it is named good_words.txt and is in the project folder (current working directory)
	// user can override by specifying a complete path to the -g option
	if len(p.GWFilename) > 0 { // a good word list was specified
		if _, err := os.Stat(p.GWFilename); !os.IsNotExist(err) { // it exists
			_, file := filepath.Split(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good words file: %s", file))
			goodWordlist = readWordList(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good word count: %d words", len(goodWordlist)))
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
	/* slice of words on each line of text file                              */
	/*************************************************************************/

	for _, line := range wbuf {
		lwl = append(lwl, getWordsOnLine(line))
	}

	/*************************************************************************/
	/* word list map to frequency of occurrence                              */
	/*************************************************************************/

	wordListMap = getWordList(wbuf)

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
				cp = cp[:0]         // empty the current paragraph buffer
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

	/*************************************************************************/
	/* begin individual tests                                                */
	/*************************************************************************/

	// smart quote check
	if p.Nosqc || puncStyle == "British" {
		pptr = append(pptr, "☲********************************************************************************")
		pptr = append(pptr, "* PUNCTUATION SCAN                                                             *")
		pptr = append(pptr, "********************************************************************************")
		pptr = append(pptr, "")
		if p.Nosqc {
			pptr = append(pptr, "Punctuation Scan checks skipped☷")
		} else {
			pptr = append(pptr, "Punctuation Scan checks skipped (British-style quotes)☷")
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
