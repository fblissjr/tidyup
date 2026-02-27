BINARY_NAME=tidyup
VERSION=0.4.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build install clean test

build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) .

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

test:
	@go test -v -count=1 ./...

clean:
	@rm -f $(BINARY_NAME)
