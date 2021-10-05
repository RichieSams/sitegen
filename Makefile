ifeq ($(OS),Windows_NT)
EXT=.exe
else
EXT=
endif

GOIMAGE=golang:1.15-alpine
LDFLAGS?=-ldflags="-s -w"

.PHONY: build vendor

build:
	go build $(LDFLAGS) --mod=vendor -o sitegen$(EXT)

vendor:
	docker run --rm -v $(CURDIR):/ark -w /ark $(GOIMAGE) /bin/sh -c "go mod tidy; go mod vendor"
