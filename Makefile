.PHONY: all build

DATE := $(shell command -v gdate 2>/dev/null || echo date)

all: build

build:
	TIME=$$($(DATE) --utc --iso-8601=minutes | cut -c1-16); \
	HASH=$$(git rev-parse HEAD | cut -c1-9); \
	go build -ldflags "-X main.gitHash=$$HASH -X main.buildTime=$$TIME" pptext.go
