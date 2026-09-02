.PHONY: build test test-race vet cover install clean

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

install:
	go install ./cmd/vq

clean:
	rm -f coverage.out
