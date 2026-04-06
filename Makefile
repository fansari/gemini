.PHONY: all clean

# 1. Find all .go files
SRCS := $(wildcard *.go)
# 2. Strip the .go extension to get the binary names
BINS := $(SRCS:%.go=%)

all: $(BINS)

# The pattern rule: 
# To make a binary, look for the corresponding .go file
%: %.go
	@echo "🚀 Building Static Binary: $@"
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $@ $<

clean:
	@echo "🧹 Cleaning up..."
	rm -f $(BINS)
