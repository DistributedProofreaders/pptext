pptext

Usage
-----

with all defaults: `pptext`
  will run `pptext` using the input file `book-utf8.txt`
  and the data file `goodwords.txt`. If "goodwords.txt"
  is present, it will be used. The program will generate
  log files (all starting with "log...") for each test.

to see all options: `pptext -h` in a command line.

defaults are:
  source file: book-utf8.txt (or specify with "-i filename.txt")
  data file: pptext.dat ("-d otherfile.dat")
  good words list: goodwords.txt ("-g otherlist.txt")
  use BOM: false ("-useBOM" to use BOM on report files)
  use CRLF: false ("-useCRLF" to use Windows CR/LF)
  experimental: false ("-x" for developers' only)

data file.
  the data file pptext.dat is required in the same folder as
  the pptext executable. it contains:

  1. a list of dictionary words. English, UTF-8
  2. scannos list

Tests (implemented or planned (⌻))
-----

spellcheck -> report in logspell.txt
	spell check of all words in book
	uses optional goodwords.txt file for known good words

levenshtein check -> report in loglev.txt
	compares all suspect words to all okwords in text

text check -> report in logtext.txt
	- asterisk
	- adjacent spaces
	- trailing spaces
	- letter (unusual or infrequent characters)
	- spacing
	- short lines 
	- long lines
	- repeated words
	- ellipsis check
	- dash check
	- scanno check
	- curly quote check (context checks)
	- special situations
		-- mixed straight and curly quotes
		-- mixed American and British title punctuation
		-- paragraphs start with upper-case word
	    -- mixed hyphen/dash
	    -- non-breaking space
	    -- tab character
	    -- date format
	    -- I/! check
	    -- disjointed contraction
	    -- blank page placeholder found
	    -- title abbreviation comma
	    -- spaced punctuation
	    -- comma spacing
	    -- full stop spacing
	    -- abandoned HTML tags in text

(pptext generates its own run log file: logpptext.txt)

usage examples:

  ./pptext
	  processes book-utf8.txt
	  no BOM on output files, LF line terminators

  ./pptext -i lightning-utf8.txt
	  it looks for a file pptext.dat
	    -- dictionary word list
	    -- jeebies⌻
	    -- scannos
	  it looks for a file goodwords.txt or a user-specified filename
	  it generates
	    -- report files (all of format log*.txt)
	    -- if DEBUG: suspects.txt list of words
	    -- logfile.txt report of the pptext run, parameters used, etc.





