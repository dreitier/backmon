# syntax=docker/dockerfile:1

# Builder runs on the host platform and cross-compiles for the target platform,
# so multi-arch images don't need QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS builder

ARG TARGETOS
ARG TARGETARCH
# injected by the release workflow so that `backmon` logs its version and commit
ARG GIT_COMMIT=unknown
ARG GIT_TAG=unknown

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X main.gitCommit=${GIT_COMMIT} -X main.gitTag=${GIT_TAG}" \
      -o /out/backmon .

# Runtime image: static binary, CA certificates and tzdata only; no shell, runs as UID 65532
FROM gcr.io/distroless/static-debian13:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88

LABEL org.opencontainers.image.source="https://github.com/dreitier/backmon" \
      org.opencontainers.image.description="Monitoring of backup files in filesystems and object storages" \
      org.opencontainers.image.licenses="MIT"

# config.yaml is looked up in the working directory, ~/.backmon and /etc/backmon (used by the Helm chart)
WORKDIR /app
COPY --from=builder /out/backmon /app/backmon

USER 65532:65532
EXPOSE 8000/tcp

# containers have no interactive terminal, so skip the keyboard handling
ENTRYPOINT ["/app/backmon", "-background"]
