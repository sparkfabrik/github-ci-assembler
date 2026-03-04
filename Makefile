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
