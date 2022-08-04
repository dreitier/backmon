# syntax=docker/dockerfile:1

# builder image
FROM golang:1.18 AS builder

# if available, inject build args from GitHub Actions so that we have the current commit and tag
ARG GIT_COMMIT
ARG GIT_TAG

WORKDIR /build
COPY . ./
RUN make

# interpolator dependency
FROM dreitier/interpolator:1.0.0 AS interpolator

# target image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/cloudmon .
COPY --from=interpolator /app/interpolator .
COPY entrypoint.sh /app

EXPOSE 8000/tcp
ENTRYPOINT ["/app/entrypoint.sh"]