ifeq ($(OS),Windows_NT)
EXT?=.exe
RM_CMD=del /q/s
else
EXT?=
RM_CMD=rm -f
endif

GOIMAGE=golang:1.15-alpine
LDFLAGS?=-ldflags="-s -w"

export GOOS

.PHONY: build vendor

build:
	go build $(LDFLAGS) --mod=vendor -o sitegen$(EXT)

build_windows:
	$(MAKE) GOOS=windows EXT=.exe build

build_linux:
	$(MAKE) GOOS=linux EXT='' build

clean:
	$(RM_CMD) sitegen sitegen.exe

vendor:
	docker run --rm -v $(CURDIR):/ark -w /ark $(GOIMAGE) /bin/sh -c "go mod tidy; go mod vendor"
