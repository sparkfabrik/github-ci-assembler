INSTALL_DIR ?= /usr/local/bin

.PHONY: install
install: build
	@echo "Installing the gh-ci-assembler binary in $(INSTALL_DIR) ..."
	@sudo cp gh-ci-assembler $(INSTALL_DIR)/gh-ci-assembler
	@echo "Installation complete. You can now run 'gh-ci-assembler' from the command line."

.PHONY: build
build:
	@echo "Building the gh-ci-assembler binary..."
	@go build ./cmd/gh-ci-assembler/
	@echo "Build complete. The binary is ready to be installed."

.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	@rm -f gh-ci-assembler
	@echo "Clean complete."

test:
	@echo "Running tests..."
	@go test -cover ./...
	@echo "Tests complete."

test-coverage:
	@echo "Running tests with detailed coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out

test-coverage-html:
	@echo "Running tests with coverage..."
	@go test -coverprofile coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Test coverage report generated at coverage.html"
	@xdg-open coverage.html
