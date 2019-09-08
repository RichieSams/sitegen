GOIMAGE=golang:1.13-alpine
LDFLAGS?=-ldflags="-s -w"

.PHONY: build vendor

build:
	go build $(LDFLAGS) --mod=vendor -o sitegen.exe

vendor:
	docker run --rm -v $(CURDIR):/ark -w /ark $(GOIMAGE) /bin/sh -c "go mod tidy; go mod vendor"
