BINARIES:=annote-server
GOCMD:=go
VERSION:=$(shell git describe --always)
PACKAGES:=$(shell go list ./... | grep -v /vendor/)

.PHONY: all test test-integration clean rpm $(BINARIES)

all: $(BINARIES)

test:
	$(GOCMD) test -v github.com/ndlib/curate-annote/internal/annote

# use the command line flag -mysql to set the correct dial command
clean:
	rm -rf $(BINARIES)

# go will track changes in dependencies, so the makefile does not need to do
# that. That means we always compile everything here.
# Need to include initial "./" in path so go knows it is a relative package path.
annote-server:
	$(GOCMD) build ./cmd/annote-server
