package models

type Params struct {
	Infile       string
	Outfile      string
	Datfile      string
	GWFilename   string
	Experimental bool
	Nolev		 bool
	Nosqc        bool
	UseBOM       bool
	UseCRLF      bool
}

// the Lwl is a slice of slices of strings, one per line. order maintained
// Lwl[31] contains a slice of strings containing the words on line "31"
var Lwl []([]string)

// Wlm has all words in the book and their frequency of occurence
// Wlm["chocolate"] -> 3 means that word occured three times
var Wlm map[string]int

// paragraph buffer
var Pb []string

// working buffer
var Wb []string

// working dictionary
var Wd []string // working dictionary inc. goodwords.txt

// He and Be maps map word sequences to relative frequency of occurence
// higher values mean more frequently seen
var He map[string]int
var Be map[string]int

//
var Runlog []string // log for complete pptext run
var Report []string // aggregation of individual tests' reports

var Swl []string // scanno word list
var Gwl []string // good word list specified by user

var PuncStyle string

func init() {
	He = make(map[string]int)
	Be = make(map[string]int)
}
