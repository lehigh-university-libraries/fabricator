FROM golang:1.24-alpine@sha256:ddf52008bce1be455fe2b22d780b6693259aaf97b16383b6372f4b22dd33ad66

WORKDIR /app

COPY . ./

RUN mkdir -p /home/runner

RUN apk update && apk add curl --no-cache \
  && go mod download \
  && go build -o /app/fabricator \
  && go clean -cache -modcache

ENTRYPOINT [ "/app/fabricator"]
