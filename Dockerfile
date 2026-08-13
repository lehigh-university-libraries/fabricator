FROM golang:1.26-alpine@sha256:70b46548e42db77e0966aaf3619fd068734dc6c77584d526b91126504fd95816

WORKDIR /app

COPY . ./

RUN mkdir -p /home/runner

RUN apk update && apk add curl --no-cache \
  && go mod download \
  && go build -o /app/fabricator \
  && go clean -cache -modcache

ENTRYPOINT [ "/app/fabricator"]
