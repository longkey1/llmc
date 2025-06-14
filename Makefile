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
VERSION := $(shell git tag --sort=-v:refname | head -n1 2>/dev/null || echo "v0.0.0")
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

# Variables for release target
dryrun ?= true
type ?=

release: ## Release target with type argument. Usage: make release type=patch|minor|major dryrun=false
	@if [ "$(type)" = "" ]; then \
		echo "Usage: make release type=<type> [dryrun=false]"; \
		echo ""; \
		echo "Types:"; \
		echo "  patch  - Increment patch version (e.g., v1.2.3 -> v1.2.4)"; \
		echo "  minor  - Increment minor version (e.g., v1.2.3 -> v1.3.0)"; \
		echo "  major  - Increment major version (e.g., v1.2.3 -> v2.0.0)"; \
		echo ""; \
		echo "Options:"; \
		echo "  dryrun - Set to false to actually create and push the tag (default: true)"; \
		echo ""; \
		echo "Current version: $(VERSION)"; \
		exit 1; \
	elif [ "$(type)" = "patch" ] || [ "$(type)" = "minor" ] || [ "$(type)" = "major" ]; then \
		NEXT_VERSION=$(call next_version,$(type)); \
		echo "Current version: $(VERSION)"; \
		echo "Next version: $$NEXT_VERSION"; \
		if [ "$(dryrun)" = "false" ]; then \
			echo "Creating new tag $$NEXT_VERSION..."; \
			git push origin master --no-verify --force-with-lease; \
			git tag -a $$NEXT_VERSION -m "Release $$NEXT_VERSION"; \
			git push origin $$NEXT_VERSION --no-verify --force-with-lease; \
			echo "Tag $$NEXT_VERSION has been created and pushed"; \
		else \
			echo "[DRY RUN] Showing what would be done..."; \
			echo "Would push to origin/master"; \
			echo "Would create tag: $$NEXT_VERSION"; \
			echo "Would push tag to origin: $$NEXT_VERSION"; \
			echo "Dry run complete."; \
		fi \
	else \
		echo "Error: Invalid release type. Use 'patch', 'minor', or 'major'"; \
		exit 1; \
	fi

.PHONY: rerelease

# Variables for rerelease target
dryrun ?= true
tag ?=

rerelease: ## Rerelease target with tag argument. Usage: make rerelease tag=<tag> dryrun=false
	@TAG="$(tag)"; \
	if [ -z "$$TAG" ]; then \
		TAG=$$(git describe --tags --abbrev=0); \
	fi; \
	if [ -z "$$TAG" ]; then \
		echo "Error: No tag found near HEAD and no tag specified."; \
		exit 1; \
	fi; \
	echo "Target tag: $$TAG"; \
	if [ "$(dryrun)" = "false" ]; then \
		echo "Deleting GitHub release..."; \
		gh release delete "$$TAG" -y; \
		echo "Deleting local tag..."; \
		git tag -d "$$TAG"; \
		echo "Deleting remote tag..."; \
		git push origin ":refs/tags/$$TAG"; \
		echo "Recreating tag on HEAD..."; \
		git tag "$$TAG"; \
		git push origin "$$TAG"; \
		echo "Recreating GitHub release..."; \
		gh release create "$$TAG" --title "$$TAG" --notes "Re-release of $$TAG"; \
		echo "Done!"; \
	else \
		echo "[DRY RUN] Showing what would be done..."; \
		echo "Would delete release: $$TAG"; \
		echo "Would delete local tag: $$TAG"; \
		echo "Would delete remote tag: $$TAG"; \
		echo "Would create new tag at HEAD: $$TAG"; \
		echo "Would push tag to origin: $$TAG"; \
		echo "Would create new release for: $$TAG"; \
		echo "Dry run complete."; \
	fi

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


