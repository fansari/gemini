# Makefile for Gemini Rust CLI

# Variables
BINARY_NAME = gemini-rs
CARGO = cargo
HISTORY_FILE = chat_history.json

.PHONY: all build run release clean test help

# Default target: build and run in debug mode
all: run

# Display available commands
help:
	@echo "Gemini Rust CLI Management:"
	@echo "  make build    - Compile the project in debug mode"
	@echo "  make run      - Build and run the project immediately"
	@echo "  make release  - Build an optimized binary in release mode"
	@echo "  make clean    - Remove build artifacts and chat history"
	@echo "  make test     - Run rust tests"

# Build the project
build:
	@echo "Compiling debug binary..."
	$(CARGO) build

# Build and run
run:
	@echo "Starting Gemini Rust CLI..."
	$(CARGO) run

# Build optimized version
release:
	@echo "Compiling optimized release binary..."
	$(CARGO) build --release
	@echo "Release binary is located at: target/release/$(BINARY_NAME)"

# Clean up
clean:
	@echo "Cleaning up build files and history..."
	$(CARGO) clean
	rm -f $(HISTORY_FILE)
	@echo "Done."

# Run tests
test:
	@echo "Running tests..."
	$(CARGO) test
