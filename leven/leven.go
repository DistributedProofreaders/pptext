package leven

import (
	"fmt"
	"github.com/asylumcs/pptext/models"
	"strings"
	"unicode/utf8"
)

func Levenshtein(str1, str2 []rune) int {
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
func Levencheck(okwords []string, suspects []string) {
	var rs []string
	rs = append(rs, fmt.Sprintf("\n********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "LEVENSHTEIN (EDIT DISTANCE) CHECKS"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	// m2 is a slice. each "line" contains a map of words on that line
	var m2 []map[string]struct{}
	for n, _ := range models.Wb { // on each line
		rtn := make(map[string]struct{})     // a set; empty structs take no memory
		for _, word := range models.Lwl[n] { // get slice of words on the line
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
			dist := Levenshtein([]rune(suspect), []rune(okword))
			if dist < 2 {
				// get counts
				suspectwordcount := 0
				okwordcount := 0
				for n, _ := range models.Wb {
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
				for n, line := range models.Wb {
					wordsonthisline := m2[n] // a set of words on this line
					if _, ok := wordsonthisline[suspect]; ok {
						if count == 0 {
							rs = append(rs, fmt.Sprintf("  %6d: %s", n, line))
						}
						count += 1
					}
				}
				count = 0
				for n, line := range models.Wb {
					wordsonthisline := m2[n] // a set of words on this line
					if _, ok := wordsonthisline[okword]; ok {
						if count == 0 {
							rs = append(rs, fmt.Sprintf("  %6d: %s", n, line))
						}
						count += 1
					}
				}
			}
		}
	}

	models.Report = append(models.Report, rs...) // text header
	if nreports == 0 {
		models.Report = append(models.Report, "  no Levenshtein edit distance queries reported")
	}

}
