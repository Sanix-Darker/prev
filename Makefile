BIN_NAME=prev
IMAGE_NAME=sanix-darker/${BIN_NAME}
BIN_PATH=${GOPATH}/bin
GO_VERSION=1.24
REVIEWER_BIN_PATH?=../prev-test-reviewer/bin/local
LINUX_GOOS?=linux
LINUX_GOARCH?=amd64

default: help

## Get this project dependencies.
local-deps:
	go mod tidy
	go install github.com/spf13/cobra-cli@v1.3.0
	go install github.com/goreleaser/goreleaser@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1

## Locally run the golang test.
test:
	go test ./...

## Run unit tests with verbose output.
test-unit:
	go test ./internal/... -v -count=1

## Run e2e tests.
test-e2e:
	go test ./tests/... -tags=e2e -v -count=1

## Build locally the go project.
build:
	@echo "building ${BIN_NAME}"
	@echo "GOPATH=${GOPATH}"
	go generate ./...
	go build -o ${BIN_PATH}/${BIN_NAME}
	# TODO: go optimizations with flags are failing in the CI,
	# will check later
	# GO111MODULE=on \
	# CGO_ENABLED=0 \
	# go build -a -installsuffix cgo -o ${BIN_PATH}/${BIN_NAME}

## Build Linux x86_64 binary for prev-test-reviewer at ../prev-test-reviewer/bin/local.
build-linux-reviewer:
	@echo "building ${BIN_NAME} for ${LINUX_GOOS}/${LINUX_GOARCH}"
	@echo "output=${REVIEWER_BIN_PATH}"
	@mkdir -p $(dir ${REVIEWER_BIN_PATH})
	GOOS=${LINUX_GOOS} GOARCH=${LINUX_GOARCH} CGO_ENABLED=0 go build -o ${REVIEWER_BIN_PATH}

## Build portable Linux x86_64 binary for prev-test-reviewer at ../prev-test-reviewer/bin/local.
build-linux-portable:
	@echo "building portable ${BIN_NAME} for linux/amd64"
	@echo "output=../prev-test-reviewer/bin/local"
	@mkdir -p ../prev-test-reviewer/bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../prev-test-reviewer/bin/local

## Compile optimized for alpine linux.
docker-build:
	@echo "building image ${IMAGE_NAME}"
	docker build --build-arg GO_VERSION=${GO_VERSION} -t $(IMAGE_NAME):latest .

## Make sure everything is ok before a commit
pre-commit: test
	go fmt ./...
# BEGIN __INCLUDE_MKDOCS__
## Locally serve the documentation
docs-serve:
	mkdocs serve
# END __INCLUDE_MKDOCS__

## Test the goreleaser configuration locally.
goreleaser/test:
	goreleaser --snapshot --skip-publish --rm-dist

# BEGIN __DO_NOT_INCLUDE__
## Test go-archetype
go-archetype-test:
	@rm -rf /tmp/prev
	@go-archetype transform \
		--transformations .go-archetype.yaml \
		--source . --destination /tmp/prev \
		-- \
		--repo_base_url gitlab.com \
    --repo_user user \
    --repo_name my-awesome-cli \
    --short_description "short description" \
    --long_description "long description" \
    --maintainer "test user <test@user.com>" \
    --license MIT \
    --includeMkdocs no
# END __DO_NOT_INCLUDE__

## Print his help screen
help:
	@printf "Available targets:\n\n"
	@awk '/^[a-zA-Z\-\_0-9%:\\]+/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
		helpCommand = $$1; \
		helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
	gsub("\\\\", "", helpCommand); \
	gsub(":+$$", "", helpCommand); \
		printf "  \x1b[32;01m%-15s\x1b[0m %s\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST) | sort -u
	@printf "\n"
