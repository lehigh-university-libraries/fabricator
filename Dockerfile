FROM golang:1.25-alpine@sha256:f6751d823c26342f9506c03797d2527668d095b0a15f1862cddb4d927a7a4ced

WORKDIR /app

COPY . ./

RUN mkdir -p /home/runner

RUN apk update && apk add curl --no-cache \
  && go mod download \
  && go build -o /app/fabricator \
  && go clean -cache -modcache

ENTRYPOINT [ "/app/fabricator"]
