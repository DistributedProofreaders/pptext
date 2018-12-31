pptext

Usage (for standalone version)
------------------------------

with all defaults: `pptext`
  will run `pptext` using the input file `book-utf8.txt`
  and the data file `good_words.txt` if that file
  is present. The program will generate
  a single report file for all tests.

to see all options: `pptext -h` in a command line.

defaults are:
  source file: book-utf8.txt (or specify with "-i filename.txt")
  report file: report.txt (or specify with "-o reportname.txt")
  good words list: good_words.txt ("-g otherlist.txt")
  language file: master.en (or specify with "-l otherwordlist")

flags are:
    -d do not run Levenshtein distance tests
    -q do not run smart quote checks
    -v verbose; show all reports
    -b do not use BOM on report
    -u unix line endings (default Windows™ line endings)
    -x experimental, for developers only

data file.
  the data file pptext.dat is required in the same folder as
  the pptext executable. it contains:

	1. he/be phrase-frequency master lists
 	2. scannos list

To use the standalone version, have these in the same folder:
	1. pptext binary
	2. pptext.dat
	3. /wordlists folder containing language-specific wordlists
	4. the text you want to check (default is book-utf8.txt)

