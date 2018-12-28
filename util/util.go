package util

import (
	"github.com/asylumcs/pptext/models"
	"strings"
	"unicode/utf8"
	"encoding/json"
	"fmt"
)

// compare the good word list to the submitted word
// allow variations, i.e. "Rose-Ann" in GWL will match "Rose-Ann’s"
func InGoodWordList(s string) bool {
	for _, word := range models.Gwl {
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
	rs := ""
   	if len(rps) < 60 {
   		rs = s
   	} else {
   		t := 60
    	for {
    		if rps[t].rpr == ' ' {
    			break
    		}
    		t--
    	}   		
   		rs = strings.TrimSpace(s[0:rps[t].rpp])
   	}
   	return rs
}

func useEnd (s string, rps []rp) string {
	rs := ""
   	if len(rps) < 60 {
   		rs = s
   	} else {
   		t := len(rps)-60
    	for {
    		if rps[t].rpr == ' ' {
    			break
    		}
    		t++
    	}   		
   		rs = strings.TrimSpace(s[rps[t].rpp:len(s)])
   	}
   	return rs
}

func GetParaSeg(s string, where int) string {
    // convert paragraph to slice of {runes, positions}
	rps := make([]rp, 0)
	rs := ""  // return string
	for i, w := 0, 0; i < len(s); i += w {
        runeValue, width := utf8.DecodeRuneInString(s[i:])
        // fmt.Printf("%#U starts at byte position %d\n", runeValue, i)
        rps = append(rps, rp{rpr:runeValue, rpp: i})
        w = width
    }
    if where == -1 { // show paragraph end
    	rs = useEnd(s, rps)
    }
    if where == 0 { // show paragraph start
    	rs = useStart(s, rps)
    }
    if rs == "" {  // we are given a center point as a byte position
    	crp := 0  // center run position as index into rps
    	for i := 0; i < len(rps); i++ {
    		if rps[i].rpp == where {
    			crp = i
    		}
    	}
    	if crp - 30 < 0 {
    		rs = useStart(s, rps)
    	}
    	if rs == "" && crp + 30 >= len(rps) {
    		rs = useEnd(s, rps)
    	}
    	if rs == "" {
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
    		rs = strings.TrimSpace(s[rps[sleft].rpp:rps[sright].rpp])
    	}
    }
    return rs
}

func PuncStyle() string {

	// decide is this is American or British punctuation
	cbrit, camer := 0, 0
	for _, line := range models.Wb {
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

func PrettyPrint(v interface{}) (err error) {
      b, err := json.MarshalIndent(v, "", "  ")
      if err == nil {
              fmt.Println(string(b))
      }
      return
}