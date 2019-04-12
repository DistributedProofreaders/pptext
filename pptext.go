/*
filename:  pptext.go
author:    Roger Frank
license:   GPL
*/

/*
developer reference:
https://golang.org/pkg/unicode/#pkg-constants
https://golang.org/pkg/regexp/syntax/
*/

/*
2019.04.10a "show word in context" incorporates Verbose flag
             adds "-----" separator in edit distance checks
2019.04.10b case-insensitive Mr/Mr. (&c.) checks
2019.04.11  hyphenation and spaced pair check boundary code
2019.04.12  hyp/non hyp blocked in edit distance checks
*/

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const VERSION string = "2019.04.11"

var sw []string      // suspect words list
var rs []string      // array of strings for local aggregation
var pptr []string    // pptext report
var puncStyle string // punctuation style American or British

// scanno list. all lower case. curly apostrophes
var scannoWordlist []string // scanno word list

// good word list. may have straight or curly apostrophes, mixed case
var goodWordlist []string // good word list specified by user

// mixed case ok words in text
var okwords []string // ok words in text

// the lwl is a slice of slices of strings, one per line. order maintained
// lwl[31] contains a slice of strings containing the words on line "31"
var lwl []([]string)

// wordListMapCount has all words in the book and their frequency of occurence
// wordListMapCount["chocolate"] -> 3 means that word occurred three times
var wordListMapCount map[string]int

// wordListMapLines has all words in the book and their frequency of occurence
// wordListMapLines["chocolate"] -> 415,1892,2295 means that word occurred on those three lines
var wordListMapLines map[string]string

// paragraph buffer
var pbuf []string

// working buffer
var wbuf []string

// heMap and beMap maps map word sequences to relative frequency of occurence
// higher values mean more frequently seen
var heMap map[string]int
var beMap map[string]int

/* ********************************************************************** */
/*                                                                        */
/* utility functions                                                      */
/*                                                                        */
/* ********************************************************************** */

// word is entirely numeric or entirely consistent case Roman numerals

var re2a *regexp.Regexp = regexp.MustCompile(`[0123456789]+`)
var re2b *regexp.Regexp = regexp.MustCompile(`[ivxlc]+`)
var re2c *regexp.Regexp = regexp.MustCompile(`[IVXLC]+`)

//re2a := regexp.MustCompile(`[0123456789]+`)
//re2b := regexp.MustCompile(`[ivxlc]+`)
//re2c := regexp.MustCompile(`[IVXLC]+`)
func isRomOrNum(s string) bool {
	t1 := re2a.ReplaceAllString(s, "")
	t2 := re2b.ReplaceAllString(s, "")
	t3 := re2c.ReplaceAllString(s, "")
	return (t1 == "" || t2 == "" || t3 == "")
}

// return true if slice contains string
//
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// return true if both straight and curly quotes detected
//
func straightCurly(lpb []string) bool {
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

// text-wrap string into string with embedded newlines
// with leader 9 spaces after first
//
// avoid very short lines
// if final newline is within 8 characters of end, use a space

func wraptext9(s string) string {
	s2 := ""
	runecount := 0
	totalrunes := utf8.RuneCountInString(s)
	// rc := utf8.RuneCountInString(s)
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s) // first
		runecount++
		// replace space with newline (break on space)
		if runecount >= 70 && runecount < totalrunes-8 && r == ' ' {
			s2 += "\n         "
			runecount = 0
		} else {
			s2 += string(r) // append single rune to string
		}
		s = s[size:] // chop off rune
	}

	return s2
}

// text-wrap string into string with embedded newlines
// left indent 2 spaces
//
func wraptext2(s string) string {
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	s2 := "  " // start with indent
	runecount := 0
	for utf8.RuneCountInString(s) > 0 {
		r, size := utf8.DecodeRuneInString(s) // first rune
		runecount++                           // how many we have collected
		// first space after rune #68 becomes newline
		if runecount >= 68 && r == ' ' {
			s2 += "\n  "
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
/* parameters                                                             */
/*                                                                        */
/* ********************************************************************** */

type params struct {
	Infile       string
	Outdir       string // target directory
	Wlang        string
	Alang        string
	GWFilename   string
	Experimental bool
	Nolev        bool
	Nosqc        bool
	Verbose      bool
	Revision	 bool
}

var p params

/* ********************************************************************** */
/*                                                                        */
/* file operations                                                        */
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
func saveHtml(a []string, outdir string) {
	f2, err := os.Create(outdir + "/report.html") // specified report file for text output
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
	// ☰ <span class='red'>
	// ☱ <span class='green'>
	// ☲ <span class='dim'>
	// ☳ <span class='black'>
	// ☷ </span>
	// ◨ <i>
	// ◧ </i>
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
		s = strings.Replace(s, "◨", "<i>", -1)
		s = strings.Replace(s, "◧", "</i>", -1)
		s = strings.Replace(s, "☷", "</span>", -1)
		s = strings.Replace(s, "☳", "", -1) // HTML-only line flag

		fmt.Fprintf(f2, "%s\r\n", s)
	}
	for _, line := range HFOOT {
		s := strings.Replace(line, "\n", "\r\n", -1)
		fmt.Fprintf(f2, "%s\r\n", s)
	}
}

// create an empty stack to hold punctuation events
type puncEvent struct {
	punc string
	lnum int
}
type stack []puncEvent

// stack methods
func (s stack) Push(v puncEvent) stack {
	s = append(s, v) // put event on stack
	return s         // return the stack
}

// returns the stack and the puncEvent
// if pop returns -1 for a puncEvent line number,
// we tried to pop from an empty stack
func (s stack) Pop() (stack, puncEvent) {
	l := len(s)
	rval := puncEvent{"", -1} // default to return if empty stack
	if len(s) > 0 {           // have something to return
		rval = s[l-1] // get last element
		if l > 1 {
			s = s[:l-1] // pop that off stack
		} else {
			s = []puncEvent{} // now empty
		}
	}
	return s, rval // return stack and event
}

func (s stack) Peek() (stack, puncEvent) {
	l := len(s)
	rval := puncEvent{"", -1} // default to return if empty stack
	if len(s) > 0 {           // have something to return
		rval = s[l-1] // get last element
	}
	return s, rval
}

func (s stack) Dump() (stack, string) {
	t := ""
	for _, pevent := range s {
		t += pevent.punc
	}
	return s, t
}

/* tokens use in text
   ▿ certain apostrophe
   ▾ very probably apostrophe
   ⴵ apostrophe on "-ing" word
   ▵ aspell reported apostrophe
   ⸢ open single quote
   ⸣ close single quote
*/

type xpuncEvent struct {
	punc string // punctuation mark
	lnum int    // line number
	lpos int    // position on line
}

var pstack []xpuncEvent

func Push(v xpuncEvent) {
	pstack = append(pstack, v) // put event on stack
}

func Pop() xpuncEvent {
	t := xpuncEvent{"", -1, -1} // default (empty) value
	if len(pstack) > 0 {
		t = pstack[len(pstack)-1]       // get last event on stack
		pstack = pstack[:len(pstack)-1] // remove it
	}
	return t
}

func Peek() xpuncEvent {
	t := xpuncEvent{"", -1, -1} // default (empty) value
	if len(pstack) > 0 {
		t = pstack[len(pstack)-1] // get last event on stack
	}
	return t
}

// aspell qualify words in map
// return map of only those recognized by aspell
func asqual(m map[string]int) map[string]int {

	// filenames we will use
	pid := os.Getpid()
	fnpidc := fmt.Sprintf("/tmp/%dc.txt", pid) // send words to test
	fnpidd := fmt.Sprintf("/tmp/%dd.txt", pid) // pick up results

	// write words to test
	f2, err := os.Create(fnpidc)
	if err != nil {
		log.Fatal(err)
	}
	// send out the words to test
	for k, _ := range m {
		fmt.Fprintf(f2, "%s\n", k)
	}
	f2.Close()

	// run aspell
	mycommand := fmt.Sprintf("cat %s | aspell --encoding='utf-8' --list | sort | uniq > %s", fnpidc, fnpidd)
	_, err = exec.Command("bash", "-c", mycommand).Output()
	if err != nil {
		log.Fatal(err)
	}

	// get results. any word remaining should not be protected
	mycommand = fmt.Sprintf("cat %s", fnpidd)
	out, err := exec.Command("bash", "-c", mycommand).Output()
	if err != nil {
		log.Fatal(err)
	}

	// we are done with the temporary files
	os.Remove(fnpidc)
	os.Remove(fnpidd)

	// any word in "out" was not recognized by aspell
	fwords := strings.Split(string(out), "\n")
	if len(fwords) > 0 {
		// some derived words not cleared by aspell
		for _, s := range fwords {
			delete(m, s) // remove from map
		}
	}
	return m
}

// puts output in scanreport.txt in same folder as results.html

func puncScan() []string {

	rs := []string{}  // returned and displayed in pptext report
	prs := []string{} // saved to scanreport.txt

	if p.Nosqc {
		rs = append(rs, "Smart Quote Checks disabled by user")
		return rs
	}
	if puncStyle == "British" {
		rs = append(rs, "Smart Quote Checks skipped (British-style punctuation)")
		return rs
	}

	// here we can run a curly quote scan
	// build the header

	prs = append(prs, BOM+"SMART QUOTE CHECKS (overlay format)")
	prs = append(prs, "suspect punctuation marked with '@' character")
	prs = append(prs, "-------------------------------------------------------------------")
	prs = append(prs, "")

	// local working buffer to obfuscate
	lwbuf := make([]string, len(wbuf))
	copy(lwbuf, wbuf) // FIXME throws off sentinel
	lwbuf = append(lwbuf, "")

	// another copy to annotate and provide to user
	dwbuf := make([]string, len(wbuf))
	copy(dwbuf, wbuf)

	// classify apostrophes/CSQ: mid-word contractions, lists
	//
	re81 := regexp.MustCompile(`(\p{L})’(\p{L})`) // mid-word contraction
	// traililng apostrophe common word list
	re72 := regexp.MustCompile(`(?i)(^|\P{L})(wher|ther|o|an|ha|t)’($|\P{L})`)
	// leading apostrophe common word list
	re73 := regexp.MustCompile(`(?i)(^|\P{L})’(way|twas|twill|twould|twere|uns|fore|most|em|ud|cos|cept|ll|less|\d+)($|\P{L})`)

	for i, _ := range lwbuf {
		lwbuf[i] = re81.ReplaceAllString(lwbuf[i], "$1▿$2")
		lwbuf[i] = re81.ReplaceAllString(lwbuf[i], "$1▿$2") // for multiple internal quotes
		lwbuf[i] = re72.ReplaceAllString(lwbuf[i], "$1$2▿$3")
		lwbuf[i] = re73.ReplaceAllString(lwbuf[i], "$1▿$2$3")
	}

	awords := make(map[string]int) // apostrophe words

	// accept contractions in good word list
	for _, word := range goodWordlist {
		if strings.Contains(word, "’") {
			rword := strings.Replace(word, "’", "▿", -1)
			re51 := regexp.MustCompile(`(?i)(?P<1W>^|\P{L})` + word + `(?P<2W>$|\P{L})`)
			for i, _ := range lwbuf {
				lwbuf[i] = re51.ReplaceAllString(lwbuf[i], "${1W}"+rword+"${2W}")
			}
		}
	}

	/*
	   // detect all words/count using trailing apostrophes in the file
	   re37 := regexp.MustCompile(`(^|\P{L})(\p{L}+’)(\P{L}|$)`) // ending in "’"
	   for _, line := range lwbuf {
	       t := re37.FindAllStringSubmatch(line, -1)
	       for _,u := range t {
	           awords[u[2]] += 1
	       }
	   }
	*/

	// detect all words/count using leading apostrophes in the file
	re38 := regexp.MustCompile(`(^|\P{L})(’\p{L}+)(\P{L}|$)`) // starting with "’"
	for _, line := range lwbuf {
		t := re38.FindAllStringSubmatch(line, -1)
		for _, u := range t {
			awords[u[2]] += 1
		}
	}

	// if word with apostrophe occurs more than once, mark the apostrophe as ▾
	for key, value := range awords {
		if value > 1 {
			rkey := strings.Replace(key, "’", "▾", -1)
			for j, _ := range lwbuf {
				lwbuf[j] = strings.Replace(lwbuf[j], key, rkey, -1)
			}
		}
	}

	// find all phrases that occur at least twice
	// mark ends as quote pairs.
	re29 := regexp.MustCompile(`(?P<1W>‘)(?P<2W>[^’]+)(?P<3W>’)`)
	i := 0
	for ; i < len(lwbuf)-1; i++ {
		// temp join two lines
		ccl1 := len(lwbuf[i])
		s := lwbuf[i] + " " + lwbuf[i+1]
		s = re29.ReplaceAllString(s, "⸢${2W}⸣")
		// now split and put it back
		lwbuf[i] = s[:ccl1]
		lwbuf[i+1] = s[ccl1+1:]
	}

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	// thinkin’ ?= thinking processing
	// if a word ends in in’ change it to -ing and see if that's a valid word
	// if it is, protect it
	re51 := regexp.MustCompile(`(?P<a1>^|\P{L})(?P<a2>\p{L}+?in)’(?P<a3>\P{L}|$)`) // ending in "in’"
	cwords := make(map[string]int)
	for i, _ := range lwbuf {
		t := re51.FindAllStringSubmatch(lwbuf[i], -1)
		for _, u := range t {
			cwords[u[2]+"g"] = 1
		}
	}
	// reduce map of candidate words to only those ok by aspell
	cwords = asqual(cwords)
	// cloak the ok ones with ▵ replacing "’"
	for k, _ := range cwords {
		tword := k[:len(k)-1] + "’"
		pword := k[:len(k)-1] + "▵"
		for i, _ := range lwbuf {
			lwbuf[i] = strings.Replace(lwbuf[i], tword, pword, -1)
		}
	}

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	// ’ouse ?= house processing
	// if a word startsd with ’, change ’ to "h" and see if that's a valid word
	// if it is, protect it
	re52 := regexp.MustCompile(`(?P<a1>^|\P{L})’(?P<a2>\p{L}+)(?P<a3>\P{L}|$)`) // start with "’"
	cwords = make(map[string]int)
	for i, _ := range lwbuf {
		t := re52.FindAllStringSubmatch(lwbuf[i], -1)
		for _, u := range t {
			cwords["h"+u[2]] = 1
		}
	}
	// reduce map of candidate words to only those ok by aspell
	cwords = asqual(cwords)
	// cloak the ok ones with ▵ replacing "’"
	for k, _ := range cwords {
		tword := "’" + k[1:]
		pword := "▵" + k[1:]
		for i, _ := range lwbuf {
			lwbuf[i] = strings.Replace(lwbuf[i], tword, pword, -1)
		}
	}

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	/*
	   perhaps should be  disabled: fails too readily by not being able to distinguish:
	       The dogs’ barking could be heard for miles. (as an apostrophe)
	       The place has ‘gone to the dogs’ very quickly. (as a close single quote)
	*/

	// minutes’ ?= minutes processing
	// minutes’ -> minute | valid ? protect it : leave it
	re53 := regexp.MustCompile(`(?P<a1>^|\P{L})(?P<a2>\p{L}+)s’(?P<a3>\P{L}|$)`) // end in "s’"
	cwords = make(map[string]int)
	for i, _ := range lwbuf {
		t := re53.FindAllStringSubmatch(lwbuf[i], -1)
		for _, u := range t {
			cwords[u[2]] = 1
		}
	}
	// reduce map of candidate words to only those ok by aspell
	cwords = asqual(cwords)
	// cloak the ok ones with ▵ replacing "’"
	for k, _ := range cwords {
		tword := k + "s’"
		pword := k + "s▵"
		for i, _ := range lwbuf {
			lwbuf[i] = strings.Replace(lwbuf[i], tword, pword, -1)
		}
	}

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	// lwbuf is fully munged. scan for possible errors

	re80 := regexp.MustCompile(`[“”‘’]`)
	sqreport := false
	dqreport := false
	anyreport := false

	for i := 0; i < len(lwbuf); i++ {

		parabreak := false

		if strings.TrimSpace(lwbuf[i]) == "" {
			parabreak = true
		}

		if parabreak {
			// paragraph break.
			// we are sitting on a blank line between paragraphs or
			// at the EOF one past the last line.
			// process what we have and reset

			// if stack has an ODQ, look ahead and see if it may be a continued quote
			r2 := Peek()
			if r2.punc == "“" {
				if i < len(lwbuf)-1 && strings.HasPrefix(strings.TrimSpace(lwbuf[i+1]), "“") {
					_ = Pop() // continued quote
				}
			}

			// anything on stack at this point is an error
			if len(pstack) > 0 {
				stacklst := ""
				for _, t61 := range pstack {
					stacklst = stacklst + " " + t61.punc
				}
				stacklst = strings.TrimSpace(stacklst)
				anyreport = true
				dwbuf[i-1] += fmt.Sprintf("[@NESK %s]", stacklst) // non-empty stack at paragraph end
			}

			pstack = []xpuncEvent{}
			sqreport = false
			dqreport = false
			continue
		}

		m := re80.FindAllStringSubmatchIndex(lwbuf[i], -1)
		for _, t := range m {
			rune, _ := utf8.DecodeRuneInString(lwbuf[i][t[0]:])
			r := string(rune)

			// if we hit an ODQ, check if it follows another ODQ
			// always push
			if r == "“" {
				r2 := Peek()
				if !dqreport && r2.punc == "“" {
					// consecutive ODQ
					anyreport = true
					dqreport = true
					dwbuf[i] = dwbuf[i][:t[0]] + "[@CODQ]" + dwbuf[i][t[0]:]
				}
				Push(xpuncEvent{r, i, t[0]})
			}

			// close double quote should be paired with an open double quote on the stack
			// if so, remove the ODQ on the stack
			// pop only if expected, else leave stack intact
			if r == "”" {
				r2 := Peek()
				if r2.punc == "“" { // expected
					_ = Pop()
				} else {
					anyreport = true
					dwbuf[i] = dwbuf[i][:t[0]] + "[@UCDQ]" + dwbuf[i][t[0]:]
				}
			}

			// open single quote, check if it follows another OSQ
			// always push
			if r == "‘" {
				r2 := Peek()
				if !sqreport && r2.punc == "‘" {
					// consecutive OSQ
					anyreport = true
					sqreport = true
					dwbuf[i] = dwbuf[i][:t[0]] + "[@COSQ]" + dwbuf[i][t[0]:]
				}
				Push(xpuncEvent{r, i, t[0]})
			}

			// close single quote should be paired with an open single quote on the stack
			// if so, remove the OSQ on the stack
			// pop only if expected, else leave stack intact
			if r == "’" {
				r2 := Peek()
				if r2.punc == "‘" { // expected
					_ = Pop()
				} else {
					anyreport = true
					dwbuf[i] = dwbuf[i][:t[0]] + "[@UCSQ]" + dwbuf[i][t[0]:]
				}
			}
		}
	}

	if !anyreport {
		prs = append(prs, "no punctuation scan suspects reported")
		rs = append(rs, "Punctuation Scan: no suspects reported")
	} else {
		f2, err := os.Create(p.Outdir + "/scanreport.txt")
		if err != nil {
			log.Fatal(err)
		}

		for _, u := range prs {
			fmt.Fprintf(f2, "%s\n", u)
		}
		for _, u := range dwbuf {
			fmt.Fprintf(f2, "%s\n", u)
		}
		f2.Close()
		rs = append(rs, "Punctuation Scan: report generated in scanreport.txt")
	}
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* spellcheck based on aspell                                             */
/*                                                                        */
/* ********************************************************************** */

func Intersection(a, b []string) (c []string) {
	m := make(map[string]bool)

	for _, item := range a {
		m[item] = true
	}

	for _, item := range b {
		if _, ok := m[item]; ok {
			c = append(c, item)
		}
	}
	return
}

// $ aspell --help  shows installed languages
// # apt install aspell  installs aspell and language "en"
// # apt install aspell-es  installs addtl. language

func aspellCheck() ([]string, []string, []string) {

	rs := []string{} // empty rs to start aggregation
	rs = append(rs, "☳<a name='spell'></a>")

	rs = append(rs, "☳"+strings.Repeat("*", 80))
	rs = append(rs, fmt.Sprintf("* %-76s *", fmt.Sprintf("SPELLCHECK SUSPECT WORDS (%s)", p.Alang)))
	rs = append(rs, strings.Repeat("*", 80))
	rs = append(rs, "")

	// previously, the wordListMapCount was populated with all words and how
	// often they occured. That map is the starting point for a list of good
	// words in the book. Start with all words, subtract suspects -> result
	// are good words that will be used later, as in the Levenshtein distance
	// checks of each suspect word against all good words.

	okwords := make(map[string]int, len(wordListMapCount))
	for k, v := range wordListMapCount {
		okwords[k] = v
	}

	// any words in the good word list need to be pulled out of the text *before* running
	// aspell because aspell will split it. Example: avec-trollop will be flagged for "avec"
	// this will replace the entire "avec-trollop" with "▷000000◁" which will not flag aspell.

	lwbuf := make([]string, len(wbuf))
	copy(lwbuf, wbuf)

	for i, line := range lwbuf {
		for n, word := range goodWordlist {
			// use the index to generate a token
			if strings.Contains(line, word) {
				lwbuf[i] = strings.Replace(lwbuf[i], word, fmt.Sprintf("▷%06d◁", n), -1)
			}
		}
	}

	// any hyphenated words will be evaluated by parts. either part or both can flag a report.
	//
	// begin successive aspell runs for each language

	pid := os.Getpid()
	fnpida := fmt.Sprintf("/tmp/%da.txt", pid)
	fnpidb := fmt.Sprintf("/tmp/%db.txt", pid)

	// make working copy
	f2, err := os.Create(fnpida) // specified report file for text output
	if err != nil {
		log.Fatal(err)
	}
	// send out the good-word protected local working buffer
	for _, line := range lwbuf {
		fmt.Fprintf(f2, "%s\r", line)
	}
	f2.Close()

	// process with each language specified by user
	uselangs := strings.Split(p.Alang, ",")
	for _, rl := range uselangs {

		// note: de-alt not available on machine:galaxy. de-1901 is alternate
		allowedLang := map[string]int{"en": 1, "en_US": 1, "en_GB": 1, "en_CA": 1,
			"es": 1, "fr": 1, "de": 1, "de-alt": 1, "it": 1}
		_, ok := allowedLang[rl]
		if !ok {
			log.Fatal(err)
		}

		// rs = append(rs, fmt.Sprintf("lang used: %s", rl))
		mycommand := fmt.Sprintf("cat %s | aspell --encoding='utf-8' --lang=%s --list | sort | uniq > %s", fnpida, rl, fnpidb)
		_, err := exec.Command("bash", "-c", mycommand).Output()
		if err != nil {
			log.Fatal(err)
		}

		mycommand = fmt.Sprintf("cp %s %s", fnpidb, fnpida)
		_, err = exec.Command("bash", "-c", mycommand).Output()
		if err != nil {
			log.Fatal(err)
		}
	}

	// get resulting suspect word list
	mycommand := fmt.Sprintf("cat %s", fnpida)
	out, err := exec.Command("bash", "-c", mycommand).Output()
	if err != nil {
		log.Fatal(err)
	}

	// we are done with the temporary files
	os.Remove(fnpida)
	os.Remove(fnpidb)

	// returns a newline separated string of words flagged by aspell
	suspect_words := strings.Split(string(out), "\n")

	// into slice
	if len(suspect_words) > 0 {
		suspect_words = suspect_words[:len(suspect_words)-1]
	}

	// reduce suspect using various rules
	i := 0
	for ; i < len(suspect_words); i++ {
		t := suspect_words[i]

		// if word occurs 5 or more times, accept it
		if wordListMapCount[strings.ToLower(t)]+
			wordListMapCount[strings.Title(strings.ToLower(t))]+
			wordListMapCount[strings.ToUpper(t)]+
			wordListMapCount[t] >= 5 {
			suspect_words = append(suspect_words[:i], suspect_words[i+1:]...)
			i--
			continue
		}
		// danger: this could hide a " th " type by accepting "4th"
		if t == "th" || t == "st" || t == "nd" {
			suspect_words = append(suspect_words[:i], suspect_words[i+1:]...)
			i--
			continue
		}
	}

	// sort the list of suspect words for report order
	sort.Slice(suspect_words, func(i, j int) bool {
		return strings.ToLower(suspect_words[i]) < strings.ToLower(suspect_words[j])
	})

	// show each word in context

	var sw []string // suspect words
	// lcreported := make(map[string]int) // map to hold downcased words reported

	// rs = append(rs, fmt.Sprintf("Suspect words:"))
	re4 := regexp.MustCompile(`☰`)

	for _, word := range suspect_words {
		/*
			// if I've reported the word in any case, don't report it again
			lcword := strings.ToLower(word)
			if _, ok := lcreported[lcword]; ok { // if it is there already, skip
				continue
			} else {
				lcreported[lcword] = 1
			}
		*/

		sw = append(sw, word)                    // simple slice of only the word
		rs = append(rs, fmt.Sprintf("%s", word)) // word we will show in context

		re := regexp.MustCompile(`(^|\P{L})(` + word + `)(\P{L}|$)`)

		// make sure there is an entry for this word in the line map

		if _, ok := wordListMapLines[word]; ok {
			theLines := strings.Split(wordListMapLines[word], ",")
			reported := 0
			for _, theline := range theLines {
				where, _ := strconv.Atoi(theline)
				line := wbuf[where-1] // 1-based in map
				if re.MatchString(line) {
					reported++
					line = re.ReplaceAllString(line, `$1☰$2☷$3`)
					loc := re4.FindStringIndex(line) // the start of highlighted word
					line = getParaSegment(line, loc[0])
					rs = append(rs, fmt.Sprintf("  %6d: %s", where, line)) // 1-based
				}
				if !p.Verbose && reported > 1 && len(theLines) > 2 {
					rs = append(rs, fmt.Sprintf("  ...%6d more", len(theLines)-2))
					break
				}
			}
		} else {
			// in line map word is burst by hyphens
			// sésame-ouvre-toi not found in map
			// [2806 1507,1658,2533,2806 1333,1371,2806,3196,3697]
			// find a number that's in all three (2806)
			pcs := strings.Split(word, "-")
			// find all lines with first word

			t55 := []string{}
			for n, t34 := range pcs {
				if n == 0 {
					t55 = strings.Split(wordListMapLines[pcs[0]], ",")
				} else {
					t55 = Intersection(t55, strings.Split(wordListMapLines[t34], ","))
				}
			}
			// if many matches, it probably should not be reported at all.
			if lnum, err := strconv.Atoi(t55[0]); err == nil {
				rs = append(rs, fmt.Sprintf("  %6s: %s", t55[0], wbuf[lnum-1])) // 1-based
			} else {
				rs = rs[:len(rs)-2] // back this one off rs
			}
		}

		rs = append(rs, "")
	}

	haveReport := false
	// remove all suspect words from allwords map
	for _, s := range sw {
		delete(okwords, s)
		haveReport = true
	}

	// convert to slice and return
	okslice := []string{}
	for s, _ := range okwords {
		okslice = append(okslice, s)
	}

	if !haveReport {
		rs = append(rs, "no spellcheck suspect words")
		rs[1] = "☲" + string([]rune(rs[1])[1:]) // switch to dim
	}
	rs = append(rs, "☷") // and close out dim or black if reports

	return sw, okslice, rs
}

/* ********************************************************************** */
/*                                                                        */
/* utilities                                                              */
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

// note: where is a byte count, not a rune count

func getParaSegment(ss string, where int) string {

	s := "" // string to return

	// convert to array of runes
	rps := make([]rp, 0)
	for i, w := 0, 0; i < len(ss); i += w {
		runeValue, width := utf8.DecodeRuneInString(ss[i:])
		rps = append(rps, rp{rpr: runeValue, rpp: i})
		w = width
	}

	// find the rune index of the "where" location
	// test for end of ss string before accessing last+1 char
	i := 0
	for ; i < len(rps) && rps[i].rpp < where; i++ {
	}

	// adjust left and right limit
	llim := i - 30
	rlim := i + 30

	if llim < 0 {
		llim = 0
		rlim = 60
	}
	if rlim >= len(rps) {
		llim = len(rps) - 60
		rlim = len(rps)
	}
	if llim < 0 {
		llim = 0
	}
	if rlim >= len(rps) {
		rlim = len(rps)
	}

	// break on spaces or start/end of ss
	for llim > 0 && rps[llim].rpr != ' ' {
		llim--
	}
	for rlim < len(rps) && rps[rlim].rpr != ' ' {
		rlim++
	}

	rpseg := rps[llim:rlim]
	for _, t := range rpseg {
		s += string(t.rpr)
	}
	s = strings.TrimSpace(s)
	s = strings.Replace(s, "  ", " ", -1)
	s = strings.Replace(s, "  ", " ", -1)
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

// check for "motor-car" and "motorcar"

func tcHypConsistency(wb []string) []string {
	rs := []string{}
	rs = append(rs, "----- hyphenation and non-hyphenated check ------------------------------------")
	rs = append(rs, "")

	count := 0
	for s, _ := range wordListMapCount {
		if strings.Contains(s, "-") {
			// hyphenated version present
			s2 := strings.Replace(s, "-", "", -1)
			// create non-hyphenated version and look for it
			reported := false
			for t, _ := range wordListMapCount {
				if reported {
					break
				}
				if t == s2 {
					// found it. report it
					count++
					rs = append(rs, fmt.Sprintf("%s (%d) ❬-❭ %s (%d)", s2, wordListMapCount[s2], s, wordListMapCount[s]))
					where_s := strings.Split(wordListMapLines[s], ",")
					where_s2 := strings.Split(wordListMapLines[s2], ",")
					re1 := regexp.MustCompile(`(\P{L}` + s + `\P{L})`)
					where_s_count := 0
					for _, n := range where_s {
						ni, _ := strconv.ParseInt(n, 10, 64)
						line := wb[ni-1]
						line = re1.ReplaceAllString(" "+line+" ", `☰$1☷`)
						rs = append(rs, fmt.Sprintf("%6d: %s", ni, strings.TrimSpace(line)))
						where_s_count++
						if where_s_count == 4 {
							break
						}
					}
					re2 := regexp.MustCompile(`(\P{L}` + s2 + `\P{L})`)
					where_s2_count := 0
					for _, n := range where_s2 {
						ni, _ := strconv.ParseInt(n, 10, 64)
						line := wb[ni-1]
						line = re2.ReplaceAllString(" "+line+" ", `☰$1☷`)
						rs = append(rs, fmt.Sprintf("%6d: %s", ni, strings.TrimSpace(line)))
						where_s2_count++
						if where_s2_count == 4 {
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

	for wstr := range wordListMapCount {
		if strings.Contains(wstr, "-") {
			// split into two words into hpair (hyphenation pair)
			hpair := strings.Split(wstr, "-")
			if len(hpair) != 2 {
				continue // only handle two words with one hyphen
			}
			if hpair[0] == "" || hpair[1] == "" {
				continue // need words in both spots.
			}
			// both words lower case for compare
			hpairlow := []string{strings.ToLower(hpair[0]), strings.ToLower(hpair[1])}
			// look for first word in text
			s := wordListMapLines[hpairlow[0]]
			s += "," + wordListMapLines[strings.Title(hpairlow[0])]
			s += "," + wordListMapLines[strings.ToUpper(hpairlow[0])]
			s = strings.Replace(s, ",", " ", -1)
			s = strings.TrimSpace(s)
			wreported := map[string]int{}

			if s != "" {
				// we have lines to check
				ssp := strings.Split(s, " ")
				for _, line := range ssp {
					iline, _ := strconv.Atoi(line)
					tmpline := ""
					if iline == 0 {
						continue
					}
					if iline < len(wb) {
						tmpline = wb[iline-1] + " " + wb[iline]
					} else {
						tmpline = wb[iline-1]
					}
					t := `(?i)(^|\P{L})` + hpairlow[0] + " " + hpairlow[1] + `(\P{L}|$)`
					re001 := regexp.MustCompile(t)
					where := re001.FindAllString(tmpline, -1)
					if where != nil {
						count1 := 0
						count2 := 0
						acc1 := []string{}
						acc2 := []string{}

						// words separated by a space
						re002 := regexp.MustCompile(`(?i)(?P<1W>^|\P{L})(?P<2W>` + hpairlow[0] + " " + hpairlow[1] + `)(?P<3W>\P{L}|$)`)
						// words separated by newline (equivalent to a space)
						re003 := regexp.MustCompile(`(?i)(?P<1W>^|\P{L})(?P<2W>` + hpairlow[0] + `)$`)
						re004 := regexp.MustCompile(`(?i)^(?P<2W>` + hpairlow[1] + `)(?P<3W>\P{L}|$)`)
						// words spearated by a hyphen
						re005 := regexp.MustCompile(`(?i)(?P<1W>^|\P{L})(?P<2W>` + hpairlow[0] + "-" + hpairlow[1]  + `)(?P<3W>\P{L}|$)`)

						for i, _ := range wb {

							// count word space word
							where := re002.FindAllString(wb[i], -1)
							if where != nil {
								count1++
								re002a := regexp.MustCompile(`(` + where[0] + `)`)
								t = re002a.ReplaceAllString(wb[i], `☰${1}☷`)								
								acc1 = append(acc1, fmt.Sprintf("%6d: %s", i+1, t))
							}

							// count word newline word
							if i < len(wb)-1 {
								// this line
								where2 := re003.FindAllString(wb[i], -1)
								// next line
								where3 := re004.FindAllString(wb[i+1], -1)
								if where2 != nil && where3 != nil {
									count1++
									re003a := regexp.MustCompile(`(` + where2[0] + `)`)
									t = re003a.ReplaceAllString(wb[i], `☰${1}☷`)
									acc1 = append(acc1, fmt.Sprintf("%6d: %s", i+1, t))
									re004a := regexp.MustCompile(`(` + where3[0] + `)`)
									t = re004a.ReplaceAllString(wb[i+1], `☰${1}☷`)
									acc1 = append(acc1, fmt.Sprintf("        %s", t))
								}
							}

							// count word hyphen word
							where4 := re005.FindAllString(wb[i], -1)
							if where4 != nil {
								count2++
								re005a := regexp.MustCompile(`(` + where4[0] + `)`)
								t = re005a.ReplaceAllString(wb[i], `☰${1}☷`)
								acc2 = append(acc2, fmt.Sprintf("%6d: %s", i+1, t))
							}
						}


						if _, ok := wreported[hpair[0]]; !ok {
							rs = append(rs, fmt.Sprintf("\"%s-%s\" (%d) ❬-❭ \"%s %s\" (%d)",
								hpair[0], hpair[1], count2, hpairlow[0], hpairlow[1], count1))
							if p.Verbose {
								rs = append(rs, acc2...)
								rs = append(rs, "        -----")
								rs = append(rs, acc1...)
							} else {
								rs = append(rs, acc2[0])
								rs = append(rs, "        -----")
								rs = append(rs, acc1[0])
							}
							wreported[hpair[0]] = 1
							count++
						}

					}
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
				rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line))) // 1=based
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
				rs = append(rs, fmt.Sprintf("%6d: %s", n+1, wraptext9(line))) // 1=based
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
// iterate each scanno word from list in scannos.txt
//   from https://www.pgdp.net/c/faq/stealth_scannos_eng_common.txt
// look for that word on each line of the book

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
						re := regexp.MustCompile(`(^|\P{L})(` + word + `)(\P{L}|$)`)
						line = re.ReplaceAllString(line, `$1☰$2☷$3`)
						re = regexp.MustCompile(`☰`)
						loc := re.FindStringIndex(line)
						line = getParaSegment(line, loc[0])
						rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
// if there are 50 "-" in a paragraph, consider it to have long lines of "-" characters
// as table separators, for example.

// rewrite of dash check 2019.03.22
// obfuscate what is legal. flag what remains (even on same line)
//

/*
These are the dash characters I qualify and protect
	- hyphen minus (keyboard "-")
		allow these between two letters \p{L}‐\p{L}
		allow 8 or more of these as a separator ‐{8,}
	‐ hyphen
		allow these between two letters \p{L}‐\p{L}
		allow 8 or more of these as a separator ‐{8,}
	‑ non-breaking hyphen
		allow these between two letters \p{L}‐\p{L}
	‒ figure dash (i.e. to connect digits in a phone number)
		allow these between two numbers \p{Nd}‒\p{Nd}
	– en dash (to show a range of numbers)
		allow these between two numbers \p{Nd}–\p{Nd}
		allow these between two numbers \p{Nd}\s–\s\p{Nd}
	— em dash
		allow patterns:
			\p{L}—\p{L} between letters with no spacing
				My favorite food—pizza—originated in Italy.
				My granddaughter—Kenzie—plays volleyball.
			\p{Ll}—\p{Pe} between lower-case letter and closing punctuation
				“What if we—”
			\p{Ll}— \p{Lu} lower-case letter, en dash, space, upper-case letter
				If you tell him— Wait, I will give you this.

These are dash characters I will flag
	- HYPHEN-MINUS
	֊ ARMENIAN HYPHEN
	־ HEBREW PUNCTUATION MAQAF
	᐀ CANADIAN SYLLABICS HYPHEN
	᠆ MONGOLIAN TODO SOFT HYPHEN
	‐ HYPHEN
	‑ NON-BREAKING HYPHEN
	‒ FIGURE DASH
	– EN DASH
	— EM DASH
	― HORIZONTAL BAR
	⸗ DOUBLE OBLIQUE HYPHEN
	⸚ HYPHEN WITH DIAERESIS
	⸺ TWO-EM DASH
	⸻ THREE-EM DASH
	⹀ DOUBLE HYPHEN
	〜 WAVE DASH
	〰 WAVY DASH
	゠ KATAKANA-HIRAGANA DOUBLE HYPHEN
	︱ PRESENTATION FORM FOR VERTICAL EM DASH
	︲ PRESENTATION FORM FOR VERTICAL EN DASH ﹘ SMALL EM DASH
	﹣ SMALL HYPHEN-MINUS
	－ FULLWIDTH HYPHEN-MINUS
*/

//
//
//
//
//
//

func tcDashCheck(wb []string, pb []string) []string {

	rs := []string{}
	rs = append(rs, "----- dash check -------------------------------------------------------------")
	rs = append(rs, "")

	// first pass: protect what is allowed

	dbuf := make([]string, len(wb))
	copy(dbuf, wb) // local writeable copy

	re00 := regexp.MustCompile(`\p{L}(-\p{L})+`)    // hyphen-minus between two letters
	re01 := regexp.MustCompile(`\p{L}(‐\p{L})+`)    // hyphen between two letters
	re02 := regexp.MustCompile(`-{8,}`)             // h-m eight or more as a separator
	re03 := regexp.MustCompile(`\p{L}(‐\p{L})+`)    // non-breaking hyphen between two letters
	re04 := regexp.MustCompile(`\p{Nd}‒\p{Nd}`)     // figure dash between two numbers
	re05 := regexp.MustCompile(`\p{Nd}–\p{Nd}`)     // en-dash between two numbers
	re06 := regexp.MustCompile(`\p{Nd}\s–\s\p{Nd}`) // with spaces
	re07 := regexp.MustCompile(`\p{L}—\p{L}`)       // em dash between letters with no spacing
	re08 := regexp.MustCompile(`[\p{Ll}I]—\p{Pf}`)  // between lower-case letter or "I" and final punctuation
	re09 := regexp.MustCompile(`\p{Ll}— \p{Lu}`)    // lower-case letter, en dash, space, upper-case letter

	re0a := regexp.MustCompile(`—$`) // dash can end a line if verbose not selected

	// special case: deleted words
	re0b := regexp.MustCompile(`\s——\s`) // as soon as Mr. —— had left the ship.

	// for this check, consider exactly two em-dashes as one
	re0c := regexp.MustCompile(`([^—])(——)([^—])`)
	re0d := regexp.MustCompile(`\p{Zs}*—`)

	for i := 0; i < len(dbuf)-1; i++ {
		if dbuf[i] == "" && re0d.MatchString(dbuf[i+1]) {
			dbuf[i+1] = strings.Replace(dbuf[i+1], "—", "", 1) // allow m-dash to start a paragraph
		}
	}

	for i := 0; i < len(dbuf); i++ {
		dbuf[i] = strings.Replace(dbuf[i], "_", "a", -1)  // obfuscate italics for this test
		dbuf[i] = re0b.ReplaceAllString(dbuf[i], " ")     // deleted words
		dbuf[i] = re0c.ReplaceAllString(dbuf[i], "$1—$3") // exactly two em-dashes become one
		dbuf[i] = re00.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re01.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re02.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re03.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re04.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re05.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re06.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re07.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re08.ReplaceAllString(dbuf[i], "")
		dbuf[i] = re09.ReplaceAllString(dbuf[i], "")
		if !p.Verbose {
			dbuf[i] = re0a.ReplaceAllString(dbuf[i], "")
		}
	}

	// second pass: flag what remains
	count := 0
	re := regexp.MustCompile(`\p{Pd}`)         // any dash
	re2 := regexp.MustCompile(`\p{Pd}\p{Pd}+`) // consecutive dashes

	a_hh := []string{}
	a_hm := []string{}
	a_hy := []string{}
	a_nb := []string{}
	a_fd := []string{}
	a_en := []string{}
	a_em := []string{}
	a_un := []string{}

	for i, line := range dbuf {
		if re.MatchString(line) {
			t2 := re2.MatchString(line)
			if t2 {
				a_hh = append(a_hh, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			if strings.Contains(line, "-") { // hyphen-minus
				a_hm = append(a_hm, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}

			if strings.Contains(line, "‐") { // hyphen
				a_hy = append(a_hy, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			if strings.Contains(line, "‐") { // non-breaking hyphen
				a_nb = append(a_nb, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			if strings.Contains(line, "‒") { // figure dash
				a_fd = append(a_fd, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			if strings.Contains(line, "–") { // en-dash
				a_en = append(a_en, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			if strings.Contains(line, "—") { // em-dash
				a_em = append(a_em, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
				continue
			}
			// if we get here, we have an unrecognized dash
			a_un = append(a_un, fmt.Sprintf("  %6d: %s", i+1, wb[i]))
		}
	}

	// if "--" detected, report only the first five.
	thisReportCount := 0

	if len(a_hh) > 0 {
		thisReportCount = 0
		rs = append(rs, "  adjacent dashes:")
		countdd := 0
		for _, s := range a_hh {
			if strings.Contains(s, "--") {
				countdd++
			}
			if countdd == 5 {
				rs = append(rs, "          [book uses \"--\" as em-dash. not reporting further]")
			}
			if countdd < 5 || !strings.Contains(s, "--") {
				thisReportCount++
				if !p.Verbose && thisReportCount == 10 {
					rs = append(rs, fmt.Sprintf("     ... %d more", len(a_hh)-10))
				}
				if p.Verbose || thisReportCount < 10 {
					rs = append(rs, s)
				}
				count++

			}
			count++
		}
	}
	if len(a_hm) > 0 {
		thisReportCount = 0
		rs = append(rs, "  hyphen-minus:")
		for _, s := range a_hm {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_hm)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++

		}
	}
	if len(a_hy) > 0 {
		rs = append(rs, "  hyphen:")
		for _, s := range a_hy {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_hy)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++

		}
	}
	if len(a_nb) > 0 {
		rs = append(rs, "  non-breaking hyphen:")
		for _, s := range a_nb {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_nb)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++

		}
	}
	if len(a_fd) > 0 {
		rs = append(rs, "  figure dash:")
		for _, s := range a_fd {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_fd)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++
		}
	}
	if len(a_en) > 0 {
		rs = append(rs, "  en-dash:")
		for _, s := range a_en {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_en)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++
		}
	}
	if len(a_em) > 0 {
		rs = append(rs, "  em-dash:")
		for _, s := range a_em {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_em)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++
		}
	}
	if len(a_un) > 0 {
		rs = append(rs, "  unrecognized dash:")
		for _, s := range a_un {
			thisReportCount++
			if !p.Verbose && thisReportCount == 10 {
				rs = append(rs, fmt.Sprintf("     ... %d more", len(a_un)-10))
			}
			if p.Verbose || thisReportCount < 10 {
				rs = append(rs, s)
			}
			count++
		}
	}

	if count == 0 {
		rs = append(rs, "  no dash suspects found in text.")
		rs[0] = "☲" + rs[0] // style dim
	} else {
		rs[0] = "☳" + rs[0] // style black
	}
	rs = append(rs, "")
	rs[len(rs)-1] += "☷" // close style

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

	re8 := regexp.MustCompile(`(^|[^\.])\.\.($|[^\.])`) // was becoming διαστειχων..

	count := 0
	for n, line := range wb {
		if re1.MatchString(line) || re2.MatchString(line) ||
			re3.MatchString(line) || re4.MatchString(line) ||
			re5.MatchString(line) || re6.MatchString(line) ||
			re7.MatchString(line) || re8.MatchString(line) {
			rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
				rs = append(rs, fmt.Sprintf("  %5d: ☰%s☷", n+1, line)) // 1=based
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

type longline struct {
	llen    int
	lnum    int
	theline string
}

// all lengths count runes
func tcLongLines(wb []string) []string {
	llst := []longline{} // slice of long line structures
	rs := []string{}
	rs = append(rs, "----- long lines check ------------------------------------------------------")
	rs = append(rs, "")

	count := 0
	for n, line := range wb {
		if utf8.RuneCountInString(line) > 72 {
			llst = append(llst, longline{utf8.RuneCountInString(line), n + 1, line}) // 1-based line #s
			// t := line[60:]
			// where := strings.Index(t, " ") // first space after byte 60 (s/b rune-based?)
			// rs = append(rs, fmt.Sprintf("  %5d: [%d] %s...", n+1, utf8.RuneCountInString(line), line[:60+where]))  // 1=based
			count++
		}
	}

	// sort in order of decreasing length
	sort.Slice(llst, func(i, j int) bool {
		return llst[i].llen > llst[j].llen
	})

	nreports := 0
	for _, lstr := range llst {
		if p.Verbose || nreports < 5 {
			rs = append(rs, fmt.Sprintf("%5d (%d) %s", lstr.lnum, lstr.llen, lstr.theline))
		}
		nreports++
	}

	if !p.Verbose && nreports > 5 {
		rs = append(rs, fmt.Sprintf("         .... %d more.", nreports-5))
		rs = append(rs, "")
	}

	if count == 0 {
		rs = append(rs, "  no long lines found in text.")
	}
	if count == 0 {
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
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
				rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
				// make exception for "&c"
				if strings.ContainsRune(line, kv.Key) && !strings.Contains(line, "&c") {
					if p.Verbose || reportcount < 2 {
						line = strings.Replace(line, string(kv.Key), "☰"+string(kv.Key)+"☷", -1)
						rs = append(rs, fmt.Sprintf("  %5d: %s", n+1, line)) // 1=based
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
		rs = append(rs, "  both straight and curly ◨single◧ quotes found in text")
		count++
	}

	if m['"'] > 0 && (m['“'] > 0 || m['”'] > 0) {
		rs = append(rs, "  both straight and curly ◨double◧ quotes found in text")
		count++
	}

	// ----- check to-day and today mixed -----

	ctoday, ctohday, ctonight, ctohnight, ctomorrow, ctohmorrow := 0, 0, 0, 0, 0, 0
	re01 := regexp.MustCompile(`(?i)today`)
	re02 := regexp.MustCompile(`(?i)to-day`)
	re03 := regexp.MustCompile(`(?i)tonight`)
	re04 := regexp.MustCompile(`(?i)to-night`)
	re05 := regexp.MustCompile(`(?i)tomorrow`)
	re06 := regexp.MustCompile(`(?i)to-morrow`)
	for _, line := range wb {
		ctoday += len(re01.FindAllString(line, -1))
		ctohday += len(re02.FindAllString(line, -1))
		ctonight += len(re03.FindAllString(line, -1))
		ctohnight += len(re04.FindAllString(line, -1))
		ctomorrow += len(re05.FindAllString(line, -1))
		ctohmorrow += len(re06.FindAllString(line, -1))
	}

	if (ctoday > 0 && ctohday > 0) || (ctonight > 0 && ctohnight > 0) || (ctomorrow > 0 && ctohmorrow > 0) {
		rs = append(rs, "")
	}
	if ctoday > 0 && ctohday > 0 {
		rs = append(rs, "  ☱both \"today\" and \"to-day\" found in text☷")
		count++
	}
	if ctonight > 0 && ctohnight > 0 {
		rs = append(rs, "  ☱both \"tonight\" and \"to-night\" found in text☷")
		count++
	}
	if ctomorrow > 0 && ctohmorrow > 0 {
		rs = append(rs, "  ☱both \"tomorrow\" and \"to-morrow\" found in text☷")
		count++
	}
	if p.Verbose {
		rs = append(rs, fmt.Sprintf("%10s: %3d %10s: %3d %10s: %3d ", "today", ctoday, "tonight", ctonight,
			"tomorrow", ctomorrow))
		rs = append(rs, fmt.Sprintf("%10s: %3d %10s: %3d %10s: %3d ", "to-day", ctohday, "to-night", ctohnight,
			"to-morrow", ctohmorrow))
		rs = append(rs, "")
	}

	// ----- check American and British title punctuation mixed -----

	re01 = regexp.MustCompile(`(?i)Mr\.`)
	re02 = regexp.MustCompile(`(?i)Mr\s`)
	re03 = regexp.MustCompile(`(?i)Mrs\.`)
	re04 = regexp.MustCompile(`(?i)Mrs\s`)
	re05 = regexp.MustCompile(`(?i)Dr\.`)
	re06 = regexp.MustCompile(`(?i)Dr\s`)

	count_mr_period, count_mr_space, count_mrs_period, count_mrs_space, count_dr_period, count_dr_space := 0, 0, 0, 0, 0, 0
	for _, line := range wb {
		count_mr_period += len(re01.FindAllString(line, -1))
		count_mr_space += len(re02.FindAllString(line, -1))
		count_mrs_period += len(re03.FindAllString(line, -1))
		count_mrs_space += len(re04.FindAllString(line, -1))
		count_dr_period += len(re05.FindAllString(line, -1))
		count_dr_space += len(re06.FindAllString(line, -1))
	}

	mabreported := false
	if count_mr_period > 0 && count_mr_space > 0 {
		rs = append(rs,
			fmt.Sprintf("  ☱both \"Mr.\" (%d) and \"Mr\" (%d) found in text☷",
				count_mr_period, count_mr_space))
		count++
		mabreported = true
	}
	if count_mrs_period > 0 && count_mrs_space > 0 {
		rs = append(rs,
			fmt.Sprintf("  ☱both \"Mrs.\" (%d) and \"Mrs\" (%d) found in text☷",
				count_mrs_period, count_mrs_space))
		count++
		mabreported = true
	}
	if count_dr_period > 0 && count_dr_space > 0 {
		rs = append(rs,
			fmt.Sprintf("  ☱both \"Dr.\" (%d) and \"Dr\" (%d) found in text☷",
				count_dr_period, count_dr_space))
		count++
		mabreported = true
	}
	// if mixed and not already reported
	showmaball := false
	if !mabreported && (count_mr_period+count_mrs_period+count_dr_period > 0) &&
		(count_mr_space+count_mrs_space+count_dr_space > 0) {
		rs = append(rs, "  ☱mixed American and British title punctuation☷")
		count++
		showmaball = true
	}

	if p.Verbose || showmaball {
		rs = append(rs, fmt.Sprintf("%10s: %3d %10s: %3d %10s: %3d ", "Mr", count_mr_space, "Mrs", count_mrs_space,
			"Dr", count_dr_space))
		rs = append(rs, fmt.Sprintf("%10s: %3d %10s: %3d %10s: %3d ", "Mr.", count_mr_period, "Mrs.", count_mrs_period,
			"Dr.", count_dr_period))
	}

	// ----- apostrophes and turned commas -----
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
			rs = append(rs, fmt.Sprintf("%8d,%d: %s", n+1, n+2, wb[n])) // 1=based
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
	//
	// full stop spacing regex (removed from gutcheck)
	// re0009a := regexp.MustCompile(`\.[a-zA-Z]`)    // the.horse
	// re0009b := regexp.MustCompile(`[^(Mr)(Mrs)(Dr)]\.\s[a-z]`)   // the. horse

	// Hit the ball.and run.
	// Hit the ball.Then run.
	// “Hit.”and run.
	// “Hit.”Then run.
	// any, any
	re_ns := regexp.MustCompile(`(\p{L}+)\.[”’]?(\p{L}+)`)

	// Hit the ball. and run.
	// “Hit.” and run.
	// lower, space, lower
	re_ws := regexp.MustCompile(`(\p{Ll}+)\.[”’]?\s+(\p{Ll}+)`)

	sscnt = 0

	re77 := regexp.MustCompile(`(^|\P{L})(\p{Lu}\.)(\p{Lu}\.)+(\P{L}|$)`)
	re78 := regexp.MustCompile(`\d+d\.`)
	re79 := regexp.MustCompile(`\d+s\.`)

	// iterate over each paragraph
	for _, para := range pbuf {

		// first, pull out any initials groups
		// We founded M.S.D. on Colorado Boulevard.
		para2 := re77.ReplaceAllString(para, "")

		// now any common abbreviations
		para2 = strings.Replace(para2, "a.m.", "", -1)
		para2 = strings.Replace(para2, "Mr.", "", -1)
		para2 = strings.Replace(para2, "Mrs.", "", -1)
		para2 = strings.Replace(para2, "Dr.", "", -1)
		para2 = strings.Replace(para2, "Rev.", "", -1)
		para2 = strings.Replace(para2, "i.e.", "", -1)
		para2 = strings.Replace(para2, "e.g.", "", -1)
		para2 = strings.Replace(para2, "per cent.", "", -1)
		para2 = strings.Replace(para2, "8vo.", "", -1)
		para2 = strings.Replace(para2, "Co.", "", -1)

		para2 = re78.ReplaceAllString(para2, "")
		para2 = re79.ReplaceAllString(para2, "")

		// look for patterns in modified paragraph
		loc_ns := re_ns.FindAllStringSubmatchIndex(para2, -1)
		loc_ws := re_ws.FindAllStringSubmatchIndex(para2, -1)

		// loc_?s will be of this form if two instances are in the paragraph:
		// [[4 15 4 8 10 15] [20 28 20 22 24 28]]
		// where  4 15 is the span of the entire first match,
		//        4  8 is the left word
		//       10 15 is the right word
		// second match is the second slice

		/*
		   re := regexp.MustCompile(`(\p{L}+)\.\s+(\p{Ll}+)`)
		   s := "eat this. lemon now or. else toast"
		   t := re.FindAllStringSubmatchIndex(s, -1)
		   fmt.Println(t)                   [[4 15 4 8 10 15] [20 28 20 22 24 28]]
		   fmt.Println(t[0])                [4 15 4 8 10 15]
		   fmt.Println(t[0][0])             4
		   fmt.Println(t[0][1])             15
		   fmt.Println(s[t[0][2]:t[0][3]])  this
		   fmt.Println(s[t[0][4]:t[0][5]])  lemon
		*/

		doreport := true

		// go across this paragraph examining each match

		// matches without spaces are always an error
		// 'word.word' 'word.”word'
		for _, lmatch := range loc_ns {
			if doreport {
				if sscnt == 0 {
					rs = append(rs, "  full stop followed by unexpected sequence")
					count++
				}
				sscnt++
				if sscnt < RLIMIT || p.Verbose {
					rs = append(rs, "    "+getParaSegment(para, lmatch[0]))
				}
			}
		}
		for _, lmatch := range loc_ws {
			// if any of the matches are not forgiven by exception, report paragraph segment

			w1 := para[lmatch[2]:lmatch[3]]
			w2 := para[lmatch[4]:lmatch[5]]

			// first word Dr. or Mrs. or Mr.
			if w1 == "Mr" || w1 == "Mrs" || w1 == "Dr" {
				doreport = false
			}

			// word2 is entirely numeric or Roman numerals
			if (len(strings.Trim(w2, "0123456789ivxlcIVXLC"))) == 0 {
				doreport = false
			}

			// other exceptions
			if w1 == "i" && w2 == "e" {
				doreport = false
			}
			if w1 == "ex" && w2 == "gr" {
				doreport = false
			}
			if w1 == "e" && w2 == "g" {
				doreport = false
			}

			if w2 == "p" {
				doreport = false
			}

			if doreport {
				if sscnt == 0 {
					rs = append(rs, "  full stop followed by unexpected sequence")
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
	// check: initials spacing
	/*
	   re_is := regexp.MustCompile(`(\p{Lu})\.(\p{Lu})`)
	   // iterate over each paragraph
	   for _, para := range pbuf {

	       loc_is := re_is.FindAllStringSubmatchIndex(para, -1)
	       for _, lmatch := range loc_is {
	           if sscnt == 0 {
	               rs = append(rs, "  initials spacing")
	               count++
	           }
	           sscnt++
	           if sscnt < RLIMIT || p.Verbose {
	               rs = append(rs, "    "+getParaSegment(para, lmatch[0]))
	           }
	       }
	   }
	   if sscnt > RLIMIT && !p.Verbose {
	       rs = append(rs, fmt.Sprintf("    ...%d more", sscnt-RLIMIT))
	   }
	*/

	// ------------------------------------------------------------------------
	// check: query missing paragraph break

	re = regexp.MustCompile(`[\.\?\!]”\s*“`)
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
	// check: common he/be, hut/but and had/bad checks
	//        jeebies is run separately with different algorithm (three word forms)

	const (
		HADBADPATTERN = `\bi bad\b|\byou bad\b|\bhe bad\b|\bshe bad\b|\bthey bad\b|\ba had\b|\bthe had\b`
		HUTBUTPATTERN = `(, hut\P{L})|(; hut\P{L})`
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

	// include thought-break line all '-'
	// include ending with a footnote reference
	const (
		// LEGALAMERICAN = `\.$|:$|\?$|!$|—$|.["”]|\?["”]$|['’]["”]$|!["”]$|—["”]|-----|\[\d+\]$`
		// LEGALBRITISH  = `\.$|:$|\?$|!$|—$|.['’]|\?['’]$|["”]['’]$|!['’]$|—['’]|-----|\[\d+\]$`
		// new: any right bracket accepts the entire paragraph
		LEGALAMERICAN = `\.$|:$|\?$|!$|—$|.["”]|\?["”]$|['’]["”]$|!["”]$|—["”]|-----|\]$`
		LEGALBRITISH  = `\.$|:$|\?$|!$|—$|.['’]|\?['’]$|["”]['’]$|!['’]$|—['’]|-----|\]$`
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
		para2 := para[:]

		// if para ends with an italic, drop it for this test
		if strings.HasSuffix(para2, "_") {
			para2 = para2[:len(para2)-1] // drop underscore (italic)
		}

		if !re_end.MatchString(para2) {
			if sscnt == 0 {
				rs = append(rs, "  query: unexpected paragraph end")
				count++
			}
			sscnt++
			if sscnt < RLIMIT || p.Verbose {
				rs = append(rs, "    ..."+getParaSegment(para, len(para)-1)) // show paragraph end
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

	re0000 := regexp.MustCompile(`\[[^IGMS\d]`)                               // allow Illustration, Greek, Music or number
	re0001 := regexp.MustCompile(`(?i)\bthe[\.\,\?\'\"\;\:\!\@\#\$\^\&\(\)]`) // punctuation after "the"
	re0002 := regexp.MustCompile(`(,\.)|(\.,)|(,,)|([^\.]\.\.[^\.])`)         // double punctuation

	re0003a1 := regexp.MustCompile(`..*?[a-z].*?`) // for mixed case check
	re0003b1 := regexp.MustCompile(`..*?[A-Z].*?`)
	re0003a2 := regexp.MustCompile(`...*?[a-z].*?`)
	re0003b2 := regexp.MustCompile(`...*?[A-Z].*?`)

	re0003c := regexp.MustCompile(`cb|gb|pb|sb|tb|wh|fr|br|qu|tw|gl|fl|sw|gr|sl|cl|iy`) // rare to end word
	re0003d := regexp.MustCompile(`hr|hl|cb|sb|tb|wb|tl|tn|rn|lt|tj`)                   // rare to start word
	re0006 := regexp.MustCompile(`^.$`)                                                 // single character line
	re0007 := regexp.MustCompile(`(\p{L}\- \p{L})|(\p{L} \-\p{L})`)                     // broken hyphenation

	// comma spacing regex
	re0008a := regexp.MustCompile(`[a-zA-Z_],[a-zA-Z_]`) // the,horse
	re0008b := regexp.MustCompile(`[a-zA-Z_],\d`)        // the,1

	re0008c := regexp.MustCompile(`\s,`) // space comma
	re0008d := regexp.MustCompile(`^,`)  // comma start of line

	re0010 := regexp.MustCompile(`,1\d\d\d`)         // Oct. 8,1948 date format
	re0011 := regexp.MustCompile(`I”`)               // “You should runI”
	re0012 := regexp.MustCompile(`\s’(m|ve|ll|t)\b`) // I' ve disjointed contraction
	re0013 := regexp.MustCompile(`Mr,|Mrs,|Dr,`)     // title abbrev.
	re0014 := regexp.MustCompile(`\s[\?!:;]`)        // spaced punctuation
	re0016 := regexp.MustCompile(`<\/?.*?>`)         // HTML tag
	// re0017 := regexp.MustCompile(`([^\.]\.\.\. )|(\.\.\.\.[^\s])|([^\.]\.\.[^\.])|(\.\.\.\.\.+)`)       // ellipsis
	re0018 := regexp.MustCompile(`([\.,;!?’‘]+[‘“])|([A-Za-z]+[“])|([A-LN-Za-z]+[‘])|(“ )|( ”)|(‘s\s)`) // quote direction (context)
	re0019 := regexp.MustCompile(`\b0\b`)                                                               // standalone 0

	re0020a := regexp.MustCompile(`(^|\P{Nd})1($|\P{Nd})`) // standalone 1
	// exceptions
	re0020b := regexp.MustCompile(`\$1\b`)                // standalone 1 allowed after dollar sign
	re0020c := regexp.MustCompile(`1,`)                   // standalone 1 allowed before comma
	re0020d := regexp.MustCompile(`1(-|‑|‒|–|—|―)\p{Nd}`) // standalone 1 allowed before dash+num
	re0020e := regexp.MustCompile(`\p{Nd}(-|‑|‒|–|—|―)1`) // standalone 1 allowed after num+dash
	re0020f := regexp.MustCompile(`(^|\P{Nd})1\.`)        // standalone 1 allowed as "1." (a numbered list)
	re0020g := regexp.MustCompile(`1st`)                  // standalone 1 allowed as "1st"

	re0022 := regexp.MustCompile(`\s$`)         // trailing spaces/whitespace on line
	re0023 := regexp.MustCompile(`&c([^\.]|$)`) // abbreviation &c without period
	re0024 := regexp.MustCompile(`^[!;:,.?]`)   // line starts with (selected) punctuation
	re0025 := regexp.MustCompile(`^-[^-]`)      // line starts with hyphen followed by non-hyphen

	// re0026 := regexp.MustCompile(`\.+[’”]*\p{L}`) // full stop followed by letter (redundant 2019.2.21)

	// some traditional gutcheck tests were for
	//   "string that contains cb", "string that ends in cl", "string that contains gbt",
	//   "string containing mcnt (s/b ment)", "string that contains rnb, rnm or rnp",
	//   "string that contains tb", "string that contains tii", "string that contains tli",
	//   "character strings that end with j (s/b semicolon)"
	//   "string containing at least 5 consonants in a row"
	//   "string starts with hl, hr, or rn"
	//   "string contains invalid 'hl' sequence"
	//   "string "uess" not preceded by a g"
	//   "string ii not at the beginning of a word"
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

	re0021 := regexp.MustCompile(`(\p{L}\p{Nd})|(\p{Nd}\p{L})`) // mixed alpha and numerals
	re0021a := regexp.MustCompile(`(^|\P{L})\p{Nd}*[02-9]?1st(\P{L}|$)`)
	re0021b := regexp.MustCompile(`(^|\P{L})\p{Nd}*[02-9]?2nd(\P{L}|$)`)
	re0021c := regexp.MustCompile(`(^|\P{L})\p{Nd}*[02-9]?3rd(\P{L}|$)`)
	re0021d := regexp.MustCompile(`(^|\P{L})\p{Nd}*[4567890]th(\P{L}|$)`)
	re0021e := regexp.MustCompile(`(^|\P{L})\p{Nd}*1\p{Nd}th(\P{L}|$)`)
	re0021f := regexp.MustCompile(`(^|\P{L})\p{Nd}*[23]d(\P{L}|$)`)

	for n, line := range wb {

		if re0021.MatchString(line) &&
			!re0021a.MatchString(line) &&
			!re0021b.MatchString(line) &&
			!re0021c.MatchString(line) &&
			!re0021d.MatchString(line) &&
			!re0021e.MatchString(line) &&
			!re0021f.MatchString(line) {
			gcreports = append(gcreports, reportln{"mixed letters and numbers in word", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}

		// if re0026.MatchString(line) {
		//   gcreports = append(gcreports, reportln{"full stop followed by letter", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		//}

		if re0000.MatchString(line) {
			gcreports = append(gcreports, reportln{"opening square bracket followed by other than I, G, M, S or number", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
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
			if wordListMapCount[word] < 2 && !inGoodWordList(word) {
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
				linetmp := strings.Replace(line, word, fmt.Sprintf("☰%s☷", word), -1)
				gcreports = append(gcreports, reportln{"mixed case within word", fmt.Sprintf("  %5d: %s", n, wraptext9(linetmp))})
			}

			if len(word) > 2 {
				last2 := word[len(word)-2:]
				if re0003c.MatchString(last2) {
					linetmp := strings.Replace(line, word, fmt.Sprintf("☰%s☷", word), -1)
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word ending with %s", last2), fmt.Sprintf("  %5d: %s", n, wraptext9(linetmp))})
				}
				first2 := word[:2]
				if re0003d.MatchString(first2) {
					linetmp := strings.Replace(line, word, fmt.Sprintf("☰%s☷", word), -1)
					gcreports = append(gcreports, reportln{fmt.Sprintf("query word starting with %s", first2), fmt.Sprintf("  %5d: %s", n, wraptext9(linetmp))})
				}
			}
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
		//      gcreports = append(gcreports, reportln{"ellipsis check", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		// }
		if re0018.MatchString(line) {
			gcreports = append(gcreports, reportln{"quote error (context)", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0019.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 0", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
		}
		if re0020a.MatchString(line) &&
			!re0020b.MatchString(line) &&
			!re0020c.MatchString(line) &&
			!re0020d.MatchString(line) &&
			!re0020e.MatchString(line) &&
			!re0020f.MatchString(line) &&
			!re0020g.MatchString(line) {
			gcreports = append(gcreports, reportln{"standalone 1", fmt.Sprintf("  %5d: %s", n, wraptext9(line))})
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
	rs = append(rs, tcDashCheck(wbuf, pbuf)...)
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
            a map of words and the line numbers where they appeared
    apostrophes (' and ’) protected
    hyphenated words are split and each part is treated separately (matching aspell)
*/

func getWordList(wb []string) (map[string]int, map[string]string) {

	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != '-'
	}

	m := make(map[string]int)     // map to hold words, counts
	ml := make(map[string]string) // map to hold words, lines

	// preserve apostrophes
	var re1 = regexp.MustCompile(`(\p{L})'(\p{L})`) // letter'letter
	var re2 = regexp.MustCompile(`(\p{L})’(\p{L})`) // letter’letter
	for n, element := range wb {
		// need this twice to handle alternates i.e. fo’c’s’le or sésame-ouvre-toi
		element = re1.ReplaceAllString(element, `${1}①${2}`)
		element = re1.ReplaceAllString(element, `${1}①${2}`)
		element = re2.ReplaceAllString(element, `${1}②${2}`)
		element = re2.ReplaceAllString(element, `${1}②${2}`)

		// if the user is using "--" as a "—", convert that
		// temporarily to a space so it can be a word separator
		element = strings.Replace(element, "--", " ", -1)

		// all words with special characters are protected
		// split into words
		t := (strings.FieldsFunc(element, f))

		for _, word := range t {
			// put the special characters back in there
			s := strings.Replace(word, "①", "'", -1)
			s = strings.Replace(s, "②", "’", -1)
			// and build the frequency map
			if _, ok := m[s]; ok { // if it is there already, increment
				m[s] = m[s] + 1
			} else {
				m[s] = 1
			}
			// and build the line number map
			if _, ok := ml[s]; ok { // if it is there already, add this line number
				ml[s] = ml[s] + fmt.Sprintf(",%d", n+1)
			} else { // else start a new entry for this word
				ml[s] = fmt.Sprintf("%d", n+1)
			}
		}
	}
	return m, ml
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
/* Levenshtein distance checks                                            */
/*                                                                        */
/* ********************************************************************** */

// calculate and return edit distance

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

//

func showWordInContext(word string) []string {
	re := regexp.MustCompile(`(^|\P{L})(` + word + `)(\P{L}|$)`)
	re4 := regexp.MustCompile(`☰`)
	rs := []string{}

	// make sure there is an entry for this word in the line map
	if _, ok := wordListMapLines[word]; ok {
		theLines := strings.Split(wordListMapLines[word], ",")
		reported := 0
		for _, theline := range theLines {
			where, _ := strconv.Atoi(theline)
			line := wbuf[where-1] // 1-based in map
			if re.MatchString(line) {
				reported++
				line = re.ReplaceAllString(line, `$1☰$2☷$3`)
				loc := re4.FindStringIndex(line) // the start of highlighted word
				line = getParaSegment(line, loc[0])
				rs = append(rs, fmt.Sprintf("  %6d: %s", where, line)) // 1-based
			}
			if !p.Verbose && reported > 1 && len(theLines) > 2 {
				rs = append(rs, fmt.Sprintf("  ...%6d more", len(theLines)-2))
				break
			}
		}
	} else {
		// no entry for word in word map.
		// do manual search
		rs = append(rs, "internal error: word not in wordListMapLines. please report")
	}
	return rs
}

// iterate over every suspect word at least six runes long
// update: use any length
// case insensitive
// looking for a any word in the text that is "near"
func levencheck(suspects []string) []string {

	rs := []string{} // local rs to start aggregation
	rs = append(rs, "☳<a name='leven'></a>")

	if p.Nolev {
		rs = append(rs, "☲"+strings.Repeat("*", 80))
		rs = append(rs, fmt.Sprintf("* %-76s *", "LEVENSHTEIN (EDIT DISTANCE) CHECKS disabled"))
		rs = append(rs, strings.Repeat("*", 80)+"☷")
		rs = append(rs, "")
		return rs
	}

	// here we can run a distance check
	// build the header
	rs = append(rs, "☳"+strings.Repeat("*", 80))
	rs = append(rs, fmt.Sprintf("* %-76s *", "LEVENSHTEIN (EDIT DISTANCE) CHECKS"))
	rs = append(rs, strings.Repeat("*", 80))
	rs = append(rs, "")

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

	// build a map to show all words that have been reported
	var reportd map[string]int
	reportd = make(map[string]int)

	re31 := regexp.MustCompile(`[a-zA-Z0-9’]`)
	var levreported map[string]int
	levreported = make(map[string]int)
	// for each suspect word, check against all words.
	for _, suspect := range suspects {
		suspectlc := strings.ToLower(suspect)

		// smoke and mirrors using "e" lookalike
		suspectlc = strings.Replace(suspectlc, "æ", "a𝚎", -1)
		suspectlc = strings.Replace(suspectlc, "œ", "o𝚎", -1)

		for testword, _ := range wordListMapCount {
			testwordlc := strings.ToLower(testword)

			// have both been already reported
			r1st, r2nd := false, false
			if _, ok := reportd[testwordlc]; ok {
				r1st = true
			}
			if _, ok := reportd[suspectlc]; ok {
				r2nd = true
			}
			if r1st && r2nd {
				continue
			}

			// must be five letters or more or contain unexpected character
			s31 := re31.ReplaceAllString(suspectlc, "")
			if utf8.RuneCountInString(suspectlc) < 5 && len(s31) == 0 {
				continue
			}

			// only differ by capitalization or are the same word
			if suspectlc == testwordlc {
				continue
			}

			// if both are entirely numerals or Roman Numerals, skip
			if len(strings.Trim(suspectlc, "0123456789ivxlc")) == 0 &&
				len(strings.Trim(testwordlc, "0123456789ivxlc")) == 0 {
				continue
			}

			// differ only by apparent plural
			if suspectlc == testwordlc+"s" || suspectlc+"s" == testwordlc {
				continue
			}

			// differ by only hyphenation
			if strings.Replace(suspectlc,"-","",-1) == strings.Replace(testwordlc,"-","",-1) {
				fmt.Println(suspectlc, testwordlc)
				continue
			}

			// calculate distance (case insensitive)
			dist := levenshtein([]rune(suspectlc), []rune(testwordlc))

			if dist < 2 {

				countsuspect := wordListMapCount[suspect]
				counttestword := wordListMapCount[testword]

				if counttestword == 0 || countsuspect == 0 {
					break
				}

				suspectlc := strings.Replace(suspectlc, "o𝚎", "œ", -1)
				suspectlc = strings.Replace(suspectlc, "a𝚎", "æ", -1)

				// report if not already reported
				if _, ok := levreported[suspect+":"+testword]; !ok {
					rs = append(rs, fmt.Sprintf("%s(%d):%s(%d)", suspectlc, countsuspect,
						testwordlc, counttestword))
					nreports++

					rs = append(rs, showWordInContext(testword)...)
					rs = append(rs, "          ----")
					rs = append(rs, showWordInContext(suspect)...)

					// remember this pair and do not report again
					// check above will be for these words in reverse
					levreported[testword+":"+suspect] = 1
					rs = append(rs, "")
				}

				reportd[testwordlc] = 1
				reportd[suspectlc] = 1
			}

		}
	}

	if nreports == 0 {
		rs = append(rs, "no Levenshtein edit distance queries reported")
		rs[1] = "☲" + string([]rune(rs[1])[1:]) // switch to dim
	}
	rs = append(rs, "☷") // and close out dim or black if reports
	return rs
}

/* ********************************************************************** */
/*                                                                        */
/* jeebies: he/be substitution checks                                     */
/*                                                                        */
/* ********************************************************************** */

var wbs []string // paragraphs as single line, case preserved
var wbl []string // paragraphs as single line, all lower case

func jeebies() []string {

	rs := []string{} // empty rs to start aggregation
	rs = append(rs, "☳<a name='jeebi'></a>")

	rs = append(rs, "☳"+strings.Repeat("*", 80))
	rs = append(rs, fmt.Sprintf("* %-76s *", "JEEBIES REPORT"))
	rs = append(rs, strings.Repeat("*", 80))
	rs = append(rs, "")

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
		rs = append(rs, "no jeebies checks reported")
		rs[1] = "☲" + string([]rune(rs[1])[1:]) // switch to dim
	}
	rs = append(rs, "☷") // and close out dim or black if reports
	return rs
}

// * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
//
// scanno word list in in scannos.txt file

func readScannos(infile string) []string {
	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	swl := []string{} // scanno word list
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		swl = append(swl, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	// remove BOM if present
	swl[0] = strings.TrimPrefix(swl[0], BOM)
	return swl
}

// * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
//
// he word list and be word list in in patterns.txt
// bracketed by *** BEGIN HE *** and *** END HE ***
// hebelist.txt is all lower case; contains many ’ apostrophes

func readHeBe(infile string) (map[string]int, map[string]int) {
	hmp := make(map[string]int)
	bmp := make(map[string]int)

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
			hmp[ttmp] = n
		}
		if scanbe {
			t := strings.Split(scanner.Text(), ":")
			ttmp := strings.Replace(t[0], "|", " ", -1)
			n, _ := strconv.Atoi(t[1])
			bmp[ttmp] = n
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return hmp, bmp
}

// read in the chosen word list (good word list)
// convert any straight quote marks to apostrophes
// if lower case, add title and upper case
// if title case, add upper case

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

	// if lower case, add title and upper case
	// if title case, add upper case
	addwd := []string{}
	for _, word := range wd {
		if strings.ToLower(word) == word { // all lower case
			addwd = append(addwd, strings.Title(word))   // title case word
			addwd = append(addwd, strings.ToUpper(word)) // upper case word
		}
		if strings.Title(word) == word { // title case
			addwd = append(addwd, strings.ToUpper(word)) // upper case word
		}
	}
	wd = append(wd, addwd...)
	return wd
}

func doparams() params {
	p := params{}
	flag.StringVar(&p.Infile, "i", "", "input file")
	flag.StringVar(&p.Outdir, "o", ".", "output report directory")
	flag.StringVar(&p.Alang, "a", "en", "aspell wordlist language")
	flag.StringVar(&p.GWFilename, "g", "", "good words file")
	flag.BoolVar(&p.Experimental, "x", false, "experimental (developer use)")
	flag.BoolVar(&p.Nolev, "d", false, "do not run Levenshtein distance tests")
	flag.BoolVar(&p.Nosqc, "q", false, "do not run smart quote checks")
	flag.BoolVar(&p.Verbose, "v", false, "Verbose operation")
	flag.BoolVar(&p.Revision, "r", false, "return Revision number")
	flag.Parse()
	return p
}

/*
  start of main program
*/
var runStartTime time.Time

func main() {

	loc, _ := time.LoadLocation("America/Denver")
	runStartTime = time.Now()
	pptr = append(pptr, strings.Repeat("*", 80))
	pptr = append(pptr, fmt.Sprintf("* %-76s *", "PPTEXT RUN REPORT"))
	pptr = append(pptr, fmt.Sprintf("* %76s *", "started "+time.Now().In(loc).Format(time.RFC850)))
	pptr = append(pptr, strings.Repeat("*", 80))

	p = doparams() // parse command line parameters

	if p.Revision {
		fmt.Println(VERSION)
		return
	}

	pptr = append(pptr, fmt.Sprintf("☲processing file: %s", path.Base(p.Infile)))
	pptr = append(pptr, fmt.Sprintf("pptext version: %s", VERSION))

	f, _ := os.Create(p.Outdir + "/runlog.txt")
	f.WriteString("started: " + time.Now().In(loc).Format(time.RFC850) + "\n")
	f.WriteString(fmt.Sprintf("command line: %s\n", os.Args))
	f.Close()

	wbuf = readText(p.Infile) // working buffer from user source file, line by line

	// location of executable and user's working directory
	execut, _ := os.Executable()
	loc_exec := filepath.Dir(execut) // i.e. /home/rfrank/go/src/pptext
	// loc_proj, _ := os.Getwd()     // i.e. /home/rfrank/projects/books/hiking-westward

	// report status of verbose flag.
	onoff := "off"
	if p.Verbose {
		onoff = "on"
	}
	pptr = append(pptr, fmt.Sprintf("verbose mode: %s", onoff))

	// load scannos from data file
	scannoWordlist = readScannos(filepath.Join(loc_exec, "scannos.txt"))

	// load he/be entries
	heMap, beMap = readHeBe(filepath.Join(loc_exec, "hebelist.txt"))

	// load good word list
	if len(p.GWFilename) > 0 { // a good word list was specified
		if _, err := os.Stat(p.GWFilename); !os.IsNotExist(err) { // it exists
			_, file := filepath.Split(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good words file: %s", file))
			goodWordlist = readWordList(p.GWFilename)
			pptr = append(pptr, fmt.Sprintf("good word count: %d words", len(goodWordlist)))
		} else { // it does not exist
			pptr = append(pptr, fmt.Sprintf("no %s found", p.GWFilename))
		}
	} else {
		pptr = append(pptr, "no good words file specified")
	}

	// line word list: slice of words on each line of text file (capitalization retained)

	for _, line := range wbuf {
		lwl = append(lwl, getWordsOnLine(line))
	}

	// word list map to frequency of occurrence and word list map to lines where it occurs
	// capitalization retained; apostrophes protected

	wordListMapCount, wordListMapLines = getWordList(wbuf)

	// paragraph buffer.  the user source file one paragraph per line

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

	// check and report punctuation style

	puncStyle = getPuncStyle()
	pptr = append(pptr, fmt.Sprintf("punctuation style: %s☷", puncStyle)) // close header info
	pptr = append(pptr, "")

	pptr = append(pptr, "☳reports: <a href='#spell'>spellcheck</a> | <a href='#leven'>edit distance</a> | <a href='#texta'>text analysis</a> | <a href='#jeebi'>jeebies</a>☷")
	pptr = append(pptr, "")

	// prettyPrint(scannoWordlist)
	// prettyPrint(goodWordlist)
	// *** prettyPrint(okwords)  // only available after spellcheck
	// prettyPrint(lwl)
	// prettyPrint(wordListMapCount)
	// prettyPrint(wordListMapLines)
	// prettyPrint(goodWordlist)

	/*************************************************************************/
	/* smart quote checks place separate report in scanreport.txt            */
	/*************************************************************************/

	t := puncScan()
	pptr = append(pptr, t...)

	/*************************************************************************/
	/* spellcheck (using aspell)                                             */
	/* returns suspect words, okwords, report                                */
	/*************************************************************************/

	sw, okwords, t = aspellCheck()
	pptr = append(pptr, t...)

	/*************************************************************************/
	/* Levenshtein check                                                     */
	/* compares all suspect words to all okwords in text                     */
	/*************************************************************************/

	t = levencheck(sw)
	pptr = append(pptr, t...)

	/*************************************************************************/
	/* individual text checks                                                */
	/*************************************************************************/

	t = textCheck()
	pptr = append(pptr, t...)

	/*************************************************************************/
	/* Jeebies check                                                         */
	/* jeebies looks for he/be errors                                        */
	/*************************************************************************/

	t = jeebies()
	pptr = append(pptr, t...)

	// note: remaining words in sw are suspects.
	// they could be used to start a user-maintained persistent good word list

	/*************************************************************************/
	/* all tests complete. save results to specified report file             */
	/*************************************************************************/

	pptr = append(pptr, strings.Repeat("-", 80))
	pptr = append(pptr, "run complete")
	t2 := time.Now()
	pptr = append(pptr, fmt.Sprintf("execution time: %.2f seconds", t2.Sub(runStartTime).Seconds()))

	saveHtml(pptr, p.Outdir)
}
