# pptext
consolidated text checks for Project Gutenberg texts

See [USAGE.md](USAGE.md) for how to use `pptext`.

## Building

To build the `pptext` binary, run `make`. Mac users may want to install Homebrew's coreutils package in order to make use of GNU's date command (which installs as `gdate`). The Makefile assumes GNU date's command line arguments are available.

If GNU Make is not installed, it can also be built with `go build pptext.go`.

## Dependencies

aspell needs to be installed, as well as aspell dictionaries for desired
languages.

If aspell is not installed, aspell checks may be skipped entirely with `-s`.
