BINARY_NAME=tidyup
VERSION=0.2.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build install clean

build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) main.go

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

clean:
	@rm -f $(BINARY_NAME)
