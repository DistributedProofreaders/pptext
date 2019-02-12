# pptext

Usage
-----

with all defaults: `pptext`
  will run `pptext` using the input file `book-utf8.txt`
  and the data file `good_words.txt` if that file
  is present. The program will generate
  a single report file for all tests.

to see all options: `pptext --help` in a command line.

defaults are:
  source file: book-utf8.txt (or specify with "-i filename.txt")
  report file: report.txt (or specify with "-o reportname.txt")
  good words list: good_words.txt ("-g otherlist.txt")

flags are:
  -a string
    	aspell wordlist language (default "en")
  -d	do not run Levenshtein distance tests
  -g string
    	good words file
  -h string
    	output report file (HTML) (default "report.html")
  -i string
    	input file (default "book-utf8.txt")
  -noBOM
    	no BOM on text report
  -o string
    	output report file (default "report.txt")
  -q	do not run smart quote checks
  -useLF
    	LF line endings on report
  -v	Verbose: show all reports
  -x	experimental (developers only)

data files:
    scannos.txt  list of common scannos, one per line
    hebelist.txt list of he/be pattern counts


