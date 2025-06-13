.DEFAULT_GOAL := help

export GO_VERSION=$(shell grep "^go " go.mod | sed 's/^go //')
export PRODUCT_NAME := llmc

.PHONY: init
init:
	cd .devcontainer && cat devcontainer.json.dist | envsubst '$${GO_VERSION} $${PRODUCT_NAME}' > devcontainer.json

.PHONY: build
build:
	go mod tidy
	go mod vendor


.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
