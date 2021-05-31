ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

clean:
	rm elm-package-proxy db.sqlite3

configure:
	cd /tmp && \
	go install golang.org/x/tools/cmd/goimports@latest && \
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest && \
	cd ${ROOT_DIR} && \
	curl https://pre-commit.com/install-local.py | python - && \
	pre-commit install
