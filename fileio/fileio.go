package fileio

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

var BOM = string([]byte{239, 187, 191}) // UTF-8 Byte Order Mark

func ReadText(infile string) []string {
	file, err := os.Open(infile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	wb := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		wb = append(wb, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// remove BOM if present
	wb[0] = strings.TrimPrefix(wb[0], BOM)
	return wb
}

// saves working buffer
// BOM and line ending CRLF are options. default is no
func SaveText(a []string, outfile string, useBOM bool, useCRLF bool) {
	f2, err := os.Create(outfile)
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()
	if useBOM {
		a[0] = BOM + a[0]
	}
	for _, line := range a {
		if useCRLF {
			fmt.Fprintf(f2, "%s\r\n", line)
		} else {
			fmt.Fprintf(f2, "%s\n", line)
		}
	}
}
