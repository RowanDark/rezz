.PHONY: build install-browsers test integration lint

build:
	go build -o v0x ./...

install-browsers:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium

test:
	go test ./...

integration:
	go test -tags integration ./...

lint:
	golangci-lint run
