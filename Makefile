configure:
	go install golang.org/x/tools/cmd/goimports && \
	go install github.com/fzipp/gocyclo/cmd/gocyclo
	curl https://pre-commit.com/install-local.py | python - && \
	pre-commit install && \
