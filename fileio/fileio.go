package fileio

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"github.com/asylumcs/pptext/models"
)

var BOM = string([]byte{239, 187, 191}) // UTF-8 Byte Order Mark

// Readln returns a single line (without the ending \n)
// from the input buffered reader.
// An error is returned iff there is an error with the
// buffered reader.
func Readln(r *bufio.Reader) (string, error) {
  var (isPrefix bool = true
       err error = nil
       line, ln []byte
      )
  for isPrefix && err == nil {
      line, isPrefix, err = r.ReadLine()
      ln = append(ln, line...)
  }
  return string(ln),err
}

func ReadText(infile string) []string {
	wb := []string{}
	f, err := os.Open(infile)
	if err != nil {
	    s := fmt.Sprintf("error opening file: %v\n",err)
		models.Report = append(models.Report, s)
	} else {
		r := bufio.NewReader(f)
		s, e := Readln(r)  // read first line
		for e == nil {  // continue as long as there are no errors reported
		    wb = append(wb,s)
		    s,e = Readln(r)
		}
	}
	// successfully read. remove BOM if present
	if len(wb) > 0 {
		wb[0] = strings.TrimPrefix(wb[0], BOM)
	}
	return wb
}

// saves working buffer
func SaveText(a []string, outfile string, noBOM bool, useLF bool) {
	f2, err := os.Create(outfile)
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()
	if !noBOM {  // normally provide a Byte Order Mark
		a[0] = BOM + a[0]
	}
	for _, line := range a {
		if useLF {
			fmt.Fprintf(f2, "%s\n", line)
		} else {
			s := strings.Replace(line, "\n", "\r\n", -1)
			fmt.Fprintf(f2, "%s\r\n", s)
		}
	}
}
