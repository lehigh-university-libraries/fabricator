FROM golang:1.24-alpine@sha256:daae04ebad0c21149979cd8e9db38f565ecefd8547cf4a591240dc1972cf1399

WORKDIR /app

COPY . ./

RUN mkdir -p /home/runner

RUN apk update && apk add curl --no-cache \
  && go mod download \
  && go build -o /app/fabricator \
  && go clean -cache -modcache

ENTRYPOINT [ "/app/fabricator"]
