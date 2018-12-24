package spellcheck

import (
	"fmt"
	"github.com/asylumcs/pptext/models"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func lookup(wd []string, word string) bool {
	ip := sort.SearchStrings(wd, word)       // where it would insert
	return (ip != len(wd) && wd[ip] == word) // true if we found it
}

// spellcheck returns list of suspect words, list of ok words in text

func Spellcheck(wd []string) ([]string, []string) {
	var rs []string
	rs = append(rs, fmt.Sprintf("\n********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "SPELLCHECK REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	okwordlist := make(map[string]int) // cumulative words OK by successive tests
	var willdelete []string            // words to be deleted from wordlist

	// local wlm keeps models.Wlm intact with all words/frequencies
	// local wlm is pruned as words are approved during spellcheck
	wlm := make(map[string]int, len(models.Wlm)) // copy of all words in the book, with freq
	for k, v := range models.Wlm {
		swd := strings.Replace(k, "’", "'", -1) // apostrophes straight in pptext.dat
		wlm[swd] = v
	}

	rs = append(rs, fmt.Sprintf("  unique words in text: %d words", len(wlm)))

	for word, count := range wlm {
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
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Println(len(wlm))
	// fmt.Println(len(okwordlist))

	// typically at this point, I have taken the 8995 unique words in the book
	// and sorted it into 7691 words found in the dictionary and 1376 words unresolved

	// fmt.Printf("%+v\n", wlm)
	// try to approve words that are capitalized by testing them lower case

	// check words ok by depossessive
	re := regexp.MustCompile(`'s$`)
	for word, count := range wlm {
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
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// check: words OK by their lowercase form being in the dictionary
	lcwordlist := make(map[string]int) // all words in lower case

	for word, count := range wlm {
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
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Printf("%+v\n", wlm)
	// fmt.Println(len(wlm))
	// fmt.Println(len(lcwordlist))

	// typically at this point the 1376 unresolved words are now 638 that were approved b/c their
	// lowercase form is in the dictionary and 738 words still unresolved

	// some of these are hyphenated. Break those words on hyphens and see if all the individual parts
	// are valid words. If so, approve the hyphenated version

	hywordlist := make(map[string]int) // hyphenated words OK by all parts being words

	for word, count := range wlm {
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
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// fmt.Println(len(wlm))
	// fmt.Println(len(hywordlist))

	// of the 738 unresolved words before dehyphenation checks, now an additional
	// 235 have been approved and 503 remain unresolved

	// some "words" are entirely numerals. approve those
	for word, _ := range wlm {
		if _, err := strconv.Atoi(word); err == nil {
			okwordlist[word] = 1 // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	rs = append(rs, fmt.Sprintf("  approved pure numerics: %d words", len(willdelete)))
	// delete words that are entirely numeric
	for _, word := range willdelete {
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// the 503 unresolved words are now 381 with the removal of the all-numeral words

	frwordlist := make(map[string]int) // words ok by frequency occuring 4 or more times

	// some words occur many times. Accept them by frequency if they appear four or more times
	// spelled the same way
	for word, count := range wlm {
		if count >= 4 {
			frwordlist[word] = count
			okwordlist[word] = count // remember as good word
			willdelete = append(willdelete, word)
		}
	}

	rs = append(rs, fmt.Sprintf("  approved by frequency: %d words", len(willdelete)))
	// delete words approved by frequency
	for _, word := range willdelete {
		delete(wlm, word)
	}
	willdelete = nil // clear the list of words to delete

	// show each word in context
	var sw []string // suspect words
	rs = append(rs, "--------------------------------------------------------------------------------")
	rs = append(rs, fmt.Sprintf("Suspect words"))
	for word, _ := range wlm {
		word = strings.Replace(word, "'", "’", -1)
		sw = append(sw, word)                    // simple slice of only the word
		rs = append(rs, fmt.Sprintf("%s", word)) // word we will show in context
		// show word in text
		for n, line := range models.Wb {
			for _, t2 := range models.Lwl[n] {
				if t2 == word {
					rs = append(rs, fmt.Sprintf("  %6d: %s", n, line))
				}
			}
		}
		rs = append(rs, "")
	}

	rs = append(rs, fmt.Sprintf("  good words in text: %d words", len(okwordlist)))
	rs = append(rs, fmt.Sprintf("  suspect words in text: %d words", len(sw)))

	models.Report = append(models.Report, rs...)

	var ok []string
	for word, _ := range okwordlist {
		ok = append(ok, word)
	}

	// return sw: list of suspect words and ok: list of good words in text
	return sw, ok
}
