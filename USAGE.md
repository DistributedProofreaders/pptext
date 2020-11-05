# pptext Usage

With all defaults `pptext` will run using the input file `book-utf8.txt`
and the data file `good_words.txt` if that file is present. The program
will generate a single report file for all tests.

Defaults:
* source file: `book-utf8.txt` (or specify with `-i filename.txt`)
* report file: `report.txt` (or specify with `-o reportname.txt`)
* good words list: `good_words.txt` (`-g otherlist.txt`)

To see all options run `pptext --help`:

    -a string
        aspell wordlist language (default "en")
    -d  Debug flag
    -g string
        good words file
    -i string
        input file
    -o string
        output report directory (default ".")
    -r  return Revision number
    -t string
        tests to run (default "a")
    -v  Verbose operation
    -x  experimental (developer use)

`pptext` uses two datafiles that must be in the same directory as the binary:
* `scannos.txt` - list of common scannos, one per line
* `hebelist.txt` -  list of he/be pattern counts
