.PHONY: hooks check test

# Turn on the pre-commit hook for this clone.
hooks:
	git config core.hooksPath .githooks

# What the pre-commit hook runs, plus a secret scan of the whole history.
check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || (gofmt -l $$(git ls-files '*.go'); exit 1)
	go vet ./...
	go test -count=1 -race ./...
	gitleaks git --no-banner --redact

test:
	go test ./...
