.PHONY: fmt fmt-check vet test check precommit-install

fmt:
	gofmt -w $(shell find . -name '*.go' -type f)

fmt-check:
	@unformatted=$$(gofmt -l $$(find . -name '*.go' -type f)); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	go vet ./...

test:
	go test ./...

check: fmt-check vet test

precommit-install:
	@mkdir -p .githooks
	@cp scripts/pre-commit.sh .githooks/pre-commit
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "Installed git pre-commit hook to .githooks/pre-commit"
