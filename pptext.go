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

const VERSION string = "2018.12.31"

var wlm []string // suspect words returned as list by spellcheck
var sw []string  // suspect words list

func doParams() models.Params {
	p := models.Params{}
	flag.StringVar(&p.Infile, "i", "book-utf8.txt", "input file")
	flag.StringVar(&p.Outfile, "o", "report.txt", "output report file")
	flag.StringVar(&p.Wlang, "l", "master.en", "wordlist language")
	flag.StringVar(&p.GWFilename, "g", "", "good words file")
	flag.BoolVar(&p.Experimental, "x", false, "experimental (developers only)")
	flag.BoolVar(&p.Nolev, "d", false, "do not run Levenshtein distance tests")
	flag.BoolVar(&p.Nosqc, "q", false, "do not run smart quote checks")
	flag.BoolVar(&p.UseBOM, "useBOM", true, "use BOM on text output")
	flag.BoolVar(&p.UseCRLF, "useCRLF", true, "CRLF line endings on output")
	flag.BoolVar(&p.Verbose, "v", false, "Verbose: show all reports")
	flag.Parse()
	return p
}

func main() {
	loc, _ := time.LoadLocation("America/Denver")
	start := time.Now()
	models.Report = append(models.Report, fmt.Sprintf("********************************************************************************"))
	models.Report = append(models.Report, fmt.Sprintf("* %-76s *", "PPTEXT RUN REPORT"))
	models.Report = append(models.Report, fmt.Sprintf("* %76s *", "started "+time.Now().In(loc).Format(time.RFC850)))
	models.Report = append(models.Report, fmt.Sprintf("********************************************************************************"))

	models.Report = append(models.Report, fmt.Sprintf("pptext version: %s", VERSION))

	models.P = doParams() // parse command line parameters

	/*************************************************************************/
	/* working buffer (saved in models)                                      */
	/* user-supplied source file UTF-8 encoded                               */
	/*************************************************************************/

	models.Wb = fileio.ReadText(models.P.Infile) // working buffer, line by line

	// location of executable and user's working directory
	execut, _ := os.Executable()
	loc_exec := filepath.Dir(execut) // i.e. /home/rfrank/go/src/pptext
	loc_proj, _ := os.Getwd()        // i.e. /home/rfrank/projects/books/hiking-westward

	// if executable is on rfrank.io server, do not expose internal directories
	if !strings.Contains(loc_exec, "www") {
		models.Report = append(models.Report, fmt.Sprintf("command line: %s", os.Args))
		models.Report = append(models.Report, fmt.Sprintf("executable is in: %s", loc_exec))
		models.Report = append(models.Report, fmt.Sprintf("project is in: %s", loc_proj))
	} else {
		_, file := filepath.Split(models.P.Infile)
		models.Report = append(models.Report, fmt.Sprintf("processing file: %s", file))
	}
	/*
	do not report. work to do with verbose flag. some things should always be
	"all reports" and others could be truncated.
	if models.P.Verbose {
		models.Report = append(models.Report, fmt.Sprintf("verbose flag: %s", "on"))	
	}
	*/

	/*************************************************************************/
	/* working dictionary (models.Wd)                                        */
	/* create from words in dictionary in the language specified             */
	/* and words from optional project-specific good_words.txt file          */
	/* result are all known good words in a sorted list                      */
	/*************************************************************************/

	// language-specific wordlists are in a subdirectory of the executable

	// if the user has used the -w option, a language file has been specified
	// otherwise accept default
	where := filepath.Join(loc_exec, "/wordlists/"+models.P.Wlang+".txt")
	// fmt.Println(where)
	if _, err := os.Stat(where); !os.IsNotExist(err) {
		// it exists
		models.Wd = dict.ReadDict(where)
		models.Report = append(models.Report, fmt.Sprintf("using wordlist: %s (%d words)", models.P.Wlang, len(models.Wd)))
	} else {
		models.Report = append(models.Report, fmt.Sprintf("no dictionary present"))
	}

	// require a pptext.dat file holding scannos list and jeebies he/be lists
	models.Swl = dict.ReadScannos(filepath.Join(loc_exec, "pptext.dat"))
	dict.ReadHeBe(filepath.Join(loc_exec, "pptext.dat"))

	// now the good word list.
	// by default it is named good_words.txt and is in the project folder (current working directory)
	// user can override by specifying a complete path to the -g option
	if len(models.P.GWFilename) > 0 { // a good word list was specified
		if _, err := os.Stat(models.P.GWFilename); !os.IsNotExist(err) { // it exists
			models.Gwl = dict.ReadWordList(models.P.GWFilename)
			models.Report = append(models.Report, fmt.Sprintf("good word list: %d words", len(models.Gwl)))
			models.Wd = append(models.Wd, models.Gwl...) // add good_words into dictionary
		} else { // it does not exist
			models.Report = append(models.Report, fmt.Sprintf("no %s found", models.P.GWFilename))
		}
	} else { // no full path good_words.txt file was specified. if it exists, it's in the loc_proj
		if _, err := os.Stat(filepath.Join(loc_proj, "good_words.txt")); !os.IsNotExist(err) {
			models.Gwl = dict.ReadWordList(filepath.Join(loc_proj, "good_words.txt"))
			models.Report = append(models.Report, fmt.Sprintf("good word list: %d words", len(models.Gwl)))
			models.Wd = append(models.Wd, models.Gwl...) // add good_words into dictionary
		} else { // it does not exist
			models.Report = append(models.Report, fmt.Sprintf("no %s found", "good_words.txt"))
		}
	}

	// need the words in a sorted list for binary search later
	sort.Strings(models.Wd)

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
	if models.P.Nosqc {
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "* SMART QUOTE CHECKS                                                           *")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "Smart Quote checks skipped")
	} else {
		// t1 := time.Now()
		scan.Scan()
		// t2 := time.Now()
		// models.Report = append(models.Report, fmt.Sprintf("smart quote check took: %.2f seconds", t2.Sub(t1).Seconds()))
	}

	// spellcheck
	// returns list of suspect words, ok words used in text
	sw, okwords := spellcheck.Spellcheck(models.Wd)

	// levenshtein check
	// compares all suspect words to all okwords in text
	if models.P.Nolev {
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "* LEVENSHTEIN (EDIT DISTANCE) CHECKS                                           *")
		models.Report = append(models.Report, "********************************************************************************")
		models.Report = append(models.Report, "")
		models.Report = append(models.Report, "Levenshtein (edit-distance) checks skipped")
	} else {
		// t1 := time.Now()
		leven.Levencheck(okwords, sw)
		// t2 := time.Now()
		// models.Report = append(models.Report, fmt.Sprintf("Levenshtein (edit-distance) checks took %.2f seconds", t2.Sub(t1).Seconds()))
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
	// models.Report = append(models.Report, fmt.Sprintf("execution time: %s", time.Since(start)))
	t2 := time.Now()
	models.Report = append(models.Report, fmt.Sprintf("execution time: %.2f seconds", t2.Sub(start).Seconds()))

	fileio.SaveText(models.Report, models.P.Outfile, models.P.UseBOM, models.P.UseCRLF)

	// remaining words in sw are suspects. conditionally generate a report
	var s []string
	if models.P.Experimental {
		for _, word := range sw {
			s = append(s, fmt.Sprintf("%s", word))
		}
		fileio.SaveText(s, "logsuspects.txt", models.P.UseBOM, models.P.UseCRLF)
	}
}
