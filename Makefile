BINARY_NAME=tidyup

.PHONY: build install clean

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) main.go

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

clean:
	@rm -f $(BINARY_NAME)
