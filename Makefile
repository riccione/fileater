# Added variables for build paths and versioning
BINARY_NAME=fileater
BUILD_DIR=bin
# Logic to grab git tag or short hash
VERSION=$$(git describe --tags --always)

# Production-ready flags: 
# -s: Omit symbol table
# -w: Omit DWARF debugging info (smaller binary)
# -X: Inject version string
LDFLAGS=-ldflags="-s -w -X 'main.Version=$(VERSION)'"

# Standard build for your current OS
build:
	mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Cross-platform build target (useful for local verification)
build-all:
	mkdir -p $(BUILD_DIR)
	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	# macOS (Universal)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Runs all tests in the current directory with verbose output
test:
	go test -v ./...

# Clean target to remove the build directory
clean:
	rm -rf $(BUILD_DIR)

.PHONY: build test clean
