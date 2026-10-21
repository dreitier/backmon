IMAGE=dreitier/backmon
EXECUTABLE=backmon

# if environment variables are defined (e.g. through GitHub Actions), use that. Otherwise, try to resolve them during build
# @see https://stackoverflow.com/a/24264930/2545275
GIT_COMMIT ?= $(shell git rev-list -1 HEAD | cut -c1-7)
GIT_TAG ?= $(shell git tag --points-at HEAD)
TAG ?= latest
# S3-compatible server for the S3 integration tests (same image as in .github/workflows/ci.yml)
S3MOCK_IMAGE ?= adobe/s3mock:5.2.3

.PHONY: all test test-s3 fmt lint hooks build snapshot clean docker-build pin-images

all: build

test:
	go test -v ./...

# runs the S3 integration tests against S3Mock in Docker
test-s3:
	docker run -d --rm --name backmon-s3mock -p 9090:9090 $(S3MOCK_IMAGE) > /dev/null
	curl -fsS --retry 30 --retry-delay 1 --retry-all-errors -o /dev/null http://localhost:9090/ ; \
	BACKMON_TEST_S3_ENDPOINT=http://localhost:9090 go test -race -count=1 -v -run S3 ./storage/provider/ ; \
	status=$$? ; docker stop backmon-s3mock > /dev/null ; exit $$status

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
	goreleaser release --snapshot --clean --skip=sbom,sign

clean:
	go clean
	rm -rf vendor dist $(EXECUTABLE)

# pins the Dockerfile's base images to their current digest (requires docker buildx); Dependabot keeps them updated
pin-images:
	.github/scripts/pin-images.py Dockerfile

# local image only; releases are published by .github/workflows/release.yml
docker-build:
	docker build --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg GIT_TAG=$(GIT_TAG) -t ${IMAGE}:${TAG} .
