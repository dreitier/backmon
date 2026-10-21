# syntax=docker/dockerfile:1

# Builder runs on the host platform and cross-compiles for the target platform,
# so multi-arch images don't need QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:3745f1aa11822297b79113f5e84ce2bffeb039734571c39855809cfe9e5d7be6 AS builder

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
FROM gcr.io/distroless/static-debian13:nonroot@sha256:516757106de35205efb70b91ba03f6f1298424a0678c603255cf2852a7000087

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
