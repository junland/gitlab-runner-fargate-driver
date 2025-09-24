ARG GO_VERSION
ARG ALPINE_VERSION

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION}

RUN apk update && apk upgrade && apk add --no-cache make git py-pip bash curl aws-cli && \
    go install github.com/goreleaser/goreleaser/v2@latest
