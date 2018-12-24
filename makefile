
run:
	go build

fmt:
	find . -name "*.go" -exec go fmt '{}' \;

clean:
	rm -f log*.txt

up:
	go build
	cp pptext pptext.dat ~/projects/websites/rfrank.io/site
