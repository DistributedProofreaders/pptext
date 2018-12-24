/*
filename:  pptext.go
author:    Roger Frank
license:   GPL
status:    development
*/

package main

import (
	"flag"
	"fmt"
	"github.com/asylumcs/pptext/dict"
	"github.com/asylumcs/pptext/fileio"
	"github.com/asylumcs/pptext/jeebies"
	"github.com/asylumcs/pptext/leven"
	"github.com/asylumcs/pptext/models"
	"github.com/asylumcs/pptext/scan"
	"github.com/asylumcs/pptext/spellcheck"
	"github.com/asylumcs/pptext/textcheck"
	"github.com/asylumcs/pptext/util"
	"github.com/asylumcs/pptext/wfreq"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// const VERSION string = "0.90"

var p models.Params
var wlm []string // suspect words returned as list by spellcheck
var sw []string  // suspect words list

func doParams() models.Params {
	p := models.Params{}
	flag.StringVar(&p.Infile, "i", "book-utf8.txt", "input file")
	flag.StringVar(&p.Outfile, "o", "report.txt", "output report file")
	flag.StringVar(&p.Datfile, "d", "", "data file")
	flag.StringVar(&p.GWFilename, "g", "", "good words file")
	flag.BoolVar(&p.Experimental, "x", false, "experimental (developers only)")
	flag.BoolVar(&p.Nolev, "l", false, "do not run Levenshtein tests")
	flag.BoolVar(&p.Nosqc, "q", false, "do not run smart quote checks")
	flag.BoolVar(&p.UseBOM, "useBOM", true, "use BOM on text output")
	flag.BoolVar(&p.UseCRLF, "useCRLF", true, "CRLF line endings on output")
	flag.Parse()
	return p
}

func main() {
	start := time.Now()
	models.Report = append(models.Report, fmt.Sprintf("********************************************************************************"))
	models.Report = append(models.Report, fmt.Sprintf("* %-76s *", "PPTEXT RUN REPORT"))
	models.Report = append(models.Report, fmt.Sprintf("* %76s *", "started "+time.Now().Format(time.RFC850)))
	models.Report = append(models.Report, fmt.Sprintf("********************************************************************************"))

	p = doParams() // parse command line parameters

	/*************************************************************************/
	/* working buffer (saved in models)                                      */
	/* user-supplied source file UTF-8 encoded                               */
	/*************************************************************************/

	models.Wb = fileio.ReadText(p.Infile) // working buffer, line by line

	// location of executable and user's working directory
	execut, _ := os.Executable()
	loc_exec := filepath.Dir(execut) // i.e. /home/rfrank/go/src/pptext
	loc_proj, _ := os.Getwd()        // i.e. /home/rfrank/projects/books/hiking-westward

	// if executable is on rfrank.io server, do not expose internal directories
	if !strings.Contains(loc_exec, "www") {
		models.Report = append(models.Report, fmt.Sprintf("command line: %s", os.Args))
		models.Report = append(models.Report, fmt.Sprintf("executable is in: %s", loc_exec))
		models.Report = append(models.Report, fmt.Sprintf("project is in: %s", loc_proj))
	}

	/*************************************************************************/
	/* working dictionary (models.Wd)                                        */
	/* create from words in dictionary (in pptext.dat file)                  */
	/* and words from optional project-specific goodwords.txt file           */
	/* result are all known good words in a sorted list                      */
	/*************************************************************************/

	// default dictionary is in a pptext.dat file in either the program's
	// directory or the current working directory (typ. the project directory)

	haveDict := false
	// if the user has used the -d option, it must be a complete path
	if _, err := os.Stat(p.Datfile); !os.IsNotExist(err) {
		// it exists with full path
		models.Wd = dict.ReadDict(p.Datfile)
		models.Swl = dict.ReadScannos(p.Datfile)
		dict.ReadHeBe(p.Datfile)
		if !strings.Contains(loc_exec, "www") {
			models.Report = append(models.Report, fmt.Sprintf("datafile: %s", p.Datfile))
		}
		haveDict = true
	}
	// search in same folder as executable; if not there, search project folder
	if !haveDict {
		if _, err := os.Stat(filepath.Join(loc_exec, "pptext.dat")); !os.IsNotExist(err) {
			// it exists
			models.Wd = dict.ReadDict(filepath.Join(loc_exec, "pptext.dat"))
			models.Swl = dict.ReadScannos(filepath.Join(loc_exec, "pptext.dat"))
			dict.ReadHeBe(filepath.Join(loc_exec, "pptext.dat"))
			if !strings.Contains(loc_exec, "www") {
				models.Report = append(models.Report, fmt.Sprintf("datafile: %s", filepath.Join(loc_exec, "pptext.dat")))
			}
			haveDict = true
		}
	}
	// search project folder for pptext.dat
	if !haveDict {
		if len(models.Wd) == 0 {
			if _, err := os.Stat(filepath.Join(loc_proj, "pptext.dat")); !os.IsNotExist(err) {
				// it exists
				models.Wd = dict.ReadDict(filepath.Join(loc_proj, "pptext.dat"))
				models.Swl = dict.ReadScannos(filepath.Join(loc_proj, "pptext.dat"))
				dict.ReadHeBe(filepath.Join(loc_proj, "pptext.dat"))
				if !strings.Contains(loc_exec, "www") {
					models.Report = append(models.Report,
						fmt.Sprintf("datafile: %s", filepath.Join(loc_proj, "pptext.dat")))
				}
			}
		}
	}
	if len(models.Wd) == 0 {
		models.Report = append(models.Report, fmt.Sprintf("no dictionary present"))
	} else {
		models.Report = append(models.Report, fmt.Sprintf("dictionary present: %d words", len(models.Wd)))
	}

	// now the good word list.
	// by default it is named goodwords.txt and is in the project folder (current working directory)
	// user can override by specifying a complete path to the -g option
	if len(p.GWFilename) > 0 { // a good word list was specified
		if _, err := os.Stat(p.GWFilename); !os.IsNotExist(err) { // it exists
			models.Gwl = dict.ReadWordList(p.GWFilename)
			models.Report = append(models.Report, fmt.Sprintf("good word list: %d words", len(models.Gwl)))
			models.Wd = append(models.Wd, models.Gwl...) // add goodwords into dictionary
		} else { // it does not exist
			models.Report = append(models.Report, fmt.Sprintf("no %s found", p.GWFilename))
		}
	} else { // no full path goodwords.txt file was specified. if it exists, it's in the loc_proj
		if _, err := os.Stat(filepath.Join(loc_proj, "goodwords.txt")); !os.IsNotExist(err) {
			models.Gwl = dict.ReadWordList(filepath.Join(loc_proj, "goodwords.txt"))
			models.Report = append(models.Report, fmt.Sprintf("good word list: %d words", len(models.Gwl)))
			models.Wd = append(models.Wd, models.Gwl...) // add goodwords into dictionary
		} else { // it does not exist
			models.Report = append(models.Report, fmt.Sprintf("no %s found in project directory", "goodwords.txt"))
		}
	}

	// need the words in a sorted list for binary search later
	if len(models.Gwl) > 0 {
		sort.Strings(models.Wd) // appended wordlist needs sorting
	}

	/*************************************************************************/
	/* line word list                                                        */
	/* slice of words on each line of text file                              */
	/*************************************************************************/

	for _, line := range models.Wb {
		models.Lwl = append(models.Lwl, wfreq.GetWordsOnLine(line))
	}

	/*************************************************************************/
	/* word list map to frequency of occurrence                              */
	/*************************************************************************/

	models.Wlm = wfreq.GetWordList(models.Wb)

	/*************************************************************************/
	/* paragraph buffer (pb)                                                 */
	/* the user source file one paragraph per line                           */
	/*************************************************************************/

	var cp string // current (in progress) paragraph
	for _, element := range models.Wb {
		// if this is a blank line and there is a paragraph in progress, save it
		// if not a blank line, put it into the current paragraph
		if element == "" {
			if len(cp) > 0 {
				models.Pb = append(models.Pb, cp) // save this paragraph
				cp = cp[:0]                       // empty the current paragraph buffer
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
		models.Pb = append(models.Pb, cp) // save this paragraph
	}
	models.Report = append(models.Report, fmt.Sprintf("paragraphs: %d", len(models.Pb)))

	models.PuncStyle = util.PuncStyle()
	models.Report = append(models.Report, fmt.Sprintf("punctuation style: %s", models.PuncStyle))

	/*************************************************************************/
	/* begin individual tests                                                */
	/*************************************************************************/

	// smart quote check
	if p.Nosqc {
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "* SMART QUOTE CHECKS                                                           *")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "Smart Quote checks skipped")
	} else {
		t1 := time.Now()
		scan.Scan()
		t2 := time.Now()
		models.Report = append(models.Report, fmt.Sprintf("smart quote check took: %.2f seconds", t2.Sub(t1).Seconds()))
	}

	// spellcheck
	// returns list of suspect words, ok words used in text
	sw, okwords := spellcheck.Spellcheck(models.Wd)

	// levenshtein check
	// compares all suspect words to all okwords in text
	if p.Nolev {
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "* LEVENSHTEIN (EDIT DISTANCE) CHECKS                                           *")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "Levenshtein (edit-distance) checks skipped")
	} else {
		t1 := time.Now()
		leven.Levencheck(okwords, sw)
		t2 := time.Now()
		models.Report = append(models.Report, fmt.Sprintf("Levenshtein (edit-distance) checks took %.2f seconds", t2.Sub(t1).Seconds()))
	}

	// text check
	//
	textcheck.Textcheck()

	// jeebies looks for he/be errors
	jeebies.Jeebies()

	/*************************************************************************/
	/* all tests complete. save results to specified report file and logfile */
	/*************************************************************************/

	models.Report = append(models.Report, "--------------------------------------------------------------------------------")
	models.Report = append(models.Report, "run complete")
	models.Report = append(models.Report, fmt.Sprintf("execution time: %s", time.Since(start)))
	fileio.SaveText(models.Report, p.Outfile, p.UseBOM, p.UseCRLF)

	// remaining words in sw are suspects. conditionally generate a report
	var s []string
	if p.Experimental {
		for _, word := range sw {
			s = append(s, fmt.Sprintf("%s", word))
		}
		fileio.SaveText(s, "logsuspects.txt", p.UseBOM, p.UseCRLF)
	}
}