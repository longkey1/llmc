.DEFAULT_GOAL := help

export GO_VERSION=$(shell grep "^go " go.mod | sed 's/^go //')
export PRODUCT_NAME := llmc

.PHONY: init
init: ## Initialize the project
	cd .devcontainer && cat devcontainer.json.dist | envsubst '$${GO_VERSION} $${PRODUCT_NAME}' > devcontainer.json

.PHONY: build
build: ## Build the project
	go mod tidy
	go mod vendor

.PHONY: release

# Get current version from git tag
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
MAJOR := $(shell echo $(VERSION) | cut -d. -f1 | tr -d 'v')
MINOR := $(shell echo $(VERSION) | cut -d. -f2)
PATCH := $(shell echo $(VERSION) | cut -d. -f3)

# Calculate next version based on release type
define next_version
$(if $(filter patch,$1),v$(MAJOR).$(MINOR).$(shell expr $(PATCH) + 1),\
$(if $(filter minor,$1),v$(MAJOR).$(shell expr $(MINOR) + 1).0,\
$(if $(filter major,$1),v$(shell expr $(MAJOR) + 1).0.0,\
v$(MAJOR).$(MINOR).$(shell expr $(PATCH) + 1))))
endef

release: ## Release target with type argument. Usage: make release type=patch|minor|major (default: patch)
	@if [ "$(type)" = "" ]; then \
		echo "Usage: make release type=<type>"; \
		echo ""; \
		echo "Types:"; \
		echo "  patch  - Increment patch version (e.g., v1.2.3 -> v1.2.4)"; \
		echo "  minor  - Increment minor version (e.g., v1.2.3 -> v1.3.0)"; \
		echo "  major  - Increment major version (e.g., v1.2.3 -> v2.0.0)"; \
		echo ""; \
		echo "Current version: $(VERSION)"; \
		exit 1; \
	elif [ "$(type)" = "patch" ] || [ "$(type)" = "minor" ] || [ "$(type)" = "major" ]; then \
		echo "Current version: $(VERSION)"; \
		echo "Next version: $(call next_version,$(type))"; \
		echo "Creating new tag $(call next_version,$(type))..."; \
		git push origin master --no-verify --force-with-lease; \
		git tag -a $(call next_version,$(type)) -m "Release $(call next_version,$(type))"; \
		git push origin $(call next_version,$(type)) --no-verify --force-with-lease; \
		echo "Tag $(call next_version,$(type)) has been created and pushed"; \
	else \
		echo "Error: Invalid release type. Use 'patch', 'minor', or 'major'"; \
		exit 1; \
	fi

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
