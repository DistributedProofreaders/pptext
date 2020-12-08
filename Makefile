.PHONY: all build

all: build

build:
	TIME=$$(date --utc --iso-8601=minutes | cut -c1-16); \
	HASH=$$(git rev-parse HEAD | cut -c1-9); \
	go build -ldflags "-X main.gitHash=$$HASH -X main.buildTime=$$TIME" pptext.go
