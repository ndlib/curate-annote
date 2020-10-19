BINARIES:=annote-server fix-embargo-dates
GOCMD:=go
VERSION:=$(shell git describe --always)
PACKAGES:=$(shell go list ./... | grep -v /vendor/)

.PHONY: all test test-integration clean rpm $(BINARIES)

all: $(BINARIES) web/static/mirador.js

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

web/static/mirador.js: js-src/index.js
	npm run webpack

fix-embargo-dates:
	$(GOCMD) build ./cmd/fix-embargo-dates

# to be run on the server. updates the running system.
deploy: annote-server
	rsync -a web/ /opt/annote/web
	sudo sv down annote
	cp ./annote-server /opt/annote
	sudo sv up annote

