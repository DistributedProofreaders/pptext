# pptext
consolidated text checks for Project Gutenberg texts

See [USAGE.md](USAGE.md) for how to use `pptext`.

## Building

To build the `pptext`  binary:

    go build pptext.go

## Dependencies

aspell needs to be installed, as well as aspell dictionaries for desired
languages.

If aspell is not installed, aspell checks may be skipped entirely with `-s`.
