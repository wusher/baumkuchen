BINARY := baumkuchen
CONFIG ?= baumkuchen.yml
# every setting lives in $(CONFIG); these read it, so there is one place to
# change a folder or a name. Give one on the command line to override it:
#   make export DIST=out
DIST   ?= $(shell awk '$$1=="dist:" {print $$2}' $(CONFIG))
POSTS  ?= $(shell awk '$$1=="posts:" {print $$2}' $(CONFIG))

.PHONY: help setup link build run lint check test cover clean new audit export stats

help: ## Show the targets
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | sed -e 's/:.*## /|/' | awk -F'|' '{printf "  %-7s %s\n", $$1, $$2}'

setup: ## Link the posts folder, then download and verify the Go modules
	@POSTS=$(POSTS) scripts/link-posts.sh
	go mod download
	go mod verify

build: ## Compile the server into ./$(BINARY)
	go build -trimpath -o $(BINARY) .

run: ## Start the server, on the address in $(CONFIG)
	go run .

lint: ## Repair the format, then report what is left
	gofmt -w -s .
	go fix ./... 2>/dev/null || true
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run --fix ./... \
		|| echo "note: golangci-lint is not installed, gofmt and vet only"
	go vet ./...

check: ## Report problems but change no file
	@test -z "$$(gofmt -l .)" || { echo "gofmt: these files need a repair:"; gofmt -l .; exit 1; }
	go vet ./...

test: ## Run the tests
	go test ./...

cover: ## Run the tests and report what they cover
	@go test ./... -coverprofile=.cover.out >/dev/null
	@printf 'the package  '; go test ./internal/... -coverprofile=.pkg.out 2>/dev/null | tail -1
	@printf 'everything   '; go tool cover -func=.cover.out | tail -1
	@echo "(everything counts main.go, which is flags and wiring, and has no test)"
	@echo "for the lines themselves: go test ./... -coverprofile=.cover.out && go tool cover -html=.cover.out"
	@rm -f .cover.out .pkg.out

export: ## Build the static site into ./$(DIST), with no draft in it
	@case "$(DIST)" in ""|/|.|..|"$$HOME"|"$$HOME"/) \
		echo "export: refusing to empty '$(DIST)'"; exit 1;; esac
	rm -rf $(DIST)
	go run . -export $(DIST)

link: ## Point ./posts at the folder where the posts live
	@POSTS=$(POSTS) scripts/link-posts.sh $(TO)

new: ## Start a new draft; it asks for the title
	@POSTS=$(POSTS) scripts/new-draft.sh

audit: ## What is written, and what is wrong with it
	@POSTS=$(POSTS) scripts/audit.sh

stats: ## The same table, with the numbers only
	@POSTS=$(POSTS) scripts/audit.sh --table

clean: ## Remove the compiled binary
	rm -f $(BINARY)
