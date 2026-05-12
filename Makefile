.PHONY: build install-browsers test

build:
	go build -o v0x .

install-browsers:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium

test:
	go test ./...
