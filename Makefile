.PHONY: build install-browsers test integration lint

build:
	go build -o rezz ./...

# Install Chromium for headless crawling (required for default mode)
install-browsers:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium

test:
	go test ./...

integration:
	go test -tags integration ./...

lint:
	golangci-lint run
