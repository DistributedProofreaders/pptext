package dict

import (
	"bufio"
	"github.com/asylumcs/pptext/models"
	"log"
	"os"
	"strconv"
	"strings"
)

var BOM = string([]byte{239, 187, 191}) // UTF-8 specific

// dictionary word list in in pptext.dat bracketed by
// *** BEGIN DICT *** and *** END DICT ***
func ReadDict(infile string) []string {
	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	wd := []string{}
	scanner := bufio.NewScanner(file)
	keep := false
	for scanner.Scan() {
		if scanner.Text() == "*** BEGIN DICT ***" {
			keep = true
			continue
		}
		if scanner.Text() == "*** END DICT ***" {
			keep = false
			continue
		}
		if keep {
			wd = append(wd, scanner.Text())
		}
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
func ReadScannos(infile string) []string {
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
func ReadHeBe(infile string) {
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
			models.He[ttmp] = n
		}
		if scanbe {
			t := strings.Split(scanner.Text(), ":")
			ttmp := strings.Replace(t[0], "|", " ", -1)
			n, _ := strconv.Atoi(t[1])
			models.Be[ttmp] = n
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func ReadWordList(infile string) []string {
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
