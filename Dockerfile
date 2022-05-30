# syntax=docker/dockerfile:1

# builder image
FROM golang:1.16 AS builder
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