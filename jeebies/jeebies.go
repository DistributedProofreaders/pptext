package jeebies

import (
	"fmt"
	"github.com/asylumcs/pptext/models"
	"github.com/asylumcs/pptext/util"
	"regexp"
	"strings"
)

var wbs []string
var wbl []string
var rs []string // to append to the overall runlog for all tests

func report(r string) {
	rs = append(rs, r)
}

func Jeebies() {
	rs = append(rs, fmt.Sprintf("\n********************************************************************************"))
	rs = append(rs, fmt.Sprintf("* %-76s *", "JEEBIES REPORT"))
	rs = append(rs, fmt.Sprintf("********************************************************************************"))

	models.Wb = append(models.Wb, "") // ensure last paragraph converts
	s := ""
	// convert each paragraph in the working buffer to a string
	for _, line := range models.Wb {
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

	// looking for "be" errors
	// search for three-word pattern  "w1 be w2" in lower-case paragraphs
	p3b := regexp.MustCompile(`([a-z']+ be [a-z']+)`)
	for n, para := range wbl {
		t := p3b.FindStringIndex(para)
		if t != nil {
			for ok := true; ok; ok = (t != nil) {
				// have a match
				sstr := (para[t[0]:t[1]])
				para = strings.Replace(para, sstr, "", 1)
				// have a three word form here ("must be taken")
				// see if it is in the Be list
				b_count = 0
				if val, ok := models.Be[sstr]; ok {
					b_count = val
				}
				// change "be" to "he" and see if that is in the He list
				sstr2 := strings.Replace(sstr, "be", "he", 1)
				h_count = 0
				if val, ok := models.He[sstr2]; ok {
					h_count = val
				}
				// here I have the "be" form and how common that is in b_count
				// and the "he" form and how common that is in h_count
				// fmt.Printf("%d %s\n%d %s\n\n", b_count, sstr, h_count, sstr2)
				if h_count > 0 && (b_count == 0 || float64(h_count)/float64(b_count) > 0.5) {
					// calculate how scary it is.
					// if the "he" form is three times more likely than the "be" form,
					// then scary calculates to 3.0
					if b_count == 0 {
						scary = 2.0
					} else {
						scary = float64(h_count) / float64(b_count)
					}
					where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
					t01 := fmt.Sprintf("%s (%.1f)\n    %s", sstr, scary, util.GetParaSeg(wbs[n], where))
					report(t01)
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
				// see if it is in the He list
				h_count = 0
				if val, ok := models.He[sstr]; ok {
					h_count = val
				}
				// change "he" to "be" and see if that is in the Be list
				sstr2 := strings.Replace(sstr, "he", "be", 1)
				b_count = 0
				if val, ok := models.Be[sstr2]; ok {
					b_count = val
				}
				// here I have the "he" form and how common that is in h_count
				// and the "be" form and how common that is in b_count
				// fmt.Printf("%d %s\n%d %s\n\n", h_count, sstr, b_count, sstr2)
				if b_count > 0 && (h_count == 0 || float64(b_count)/float64(h_count) > 0.5) {
					// calculate how scary it is.
					// if the "he" form is three times more likely than the "be" form,
					// then scary calculates to 3.0
					if h_count == 0 {
						scary = 2.0
					} else {
						scary = float64(b_count) / float64(h_count)
					}
					where := strings.Index(strings.ToLower(wbs[n]), strings.ToLower(sstr))
					t01 := fmt.Sprintf("%s (%.1f)\n    %s", sstr, scary, util.GetParaSeg(wbs[n], where))
					report(t01)
				}

				// see if there is another candidate
				t = p3b.FindStringIndex(para)
			}
		}
	}

	models.Report = append(models.Report, rs...)
}