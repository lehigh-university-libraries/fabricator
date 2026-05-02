FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1

WORKDIR /app

COPY . ./

RUN mkdir -p /home/runner

RUN apk update && apk add curl --no-cache \
  && go mod download \
  && go build -o /app/fabricator \
  && go clean -cache -modcache

ENTRYPOINT [ "/app/fabricator"]
