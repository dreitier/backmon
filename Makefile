IMAGE=dreitier/backmon
EXECUTABLE=backmon

# if environment variables are defined (e.g. through GitHub Actions), use that. Otherwise, try to resolve them during build
# @see https://stackoverflow.com/a/24264930/2545275
GIT_COMMIT ?= $(shell git rev-list -1 HEAD | cut -c1-7)
GIT_TAG ?= $(shell git tag --points-at HEAD)
TAG ?= latest

.PHONY: all test fmt lint hooks build snapshot clean docker-build

all: build

test:
	go test -v ./...

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

hooks:
	pre-commit install

# builds for the current OS/arch; cross-compile with e.g. `make build GOOS=linux GOARCH=arm64`
build:
	CGO_ENABLED=0 go build -trimpath -o $(EXECUTABLE) -ldflags "-s -w -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)"

# builds all release binaries into dist/ without publishing anything
snapshot:
	goreleaser release --snapshot --clean --skip=sbom

clean:
	go clean
	rm -rf vendor dist $(EXECUTABLE)

# local image only; releases are published by .github/workflows/release.yml
docker-build:
	docker build --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg GIT_TAG=$(GIT_TAG) -t ${IMAGE}:${TAG} .
