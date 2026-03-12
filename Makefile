# Added variables for build paths and versioning
BINARY_NAME=fileater
BUILD_DIR=build
# Logic to grab git tag or short hash
VERSION=$$(git describe --tags --always)

# Target to create build dir and run the go build command
build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags="-X 'main.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Clean target to remove the build directory
clean:
	rm -rf $(BUILD_DIR)

.PHONY: build clean
