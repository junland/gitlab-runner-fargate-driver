ARG GO
ARG ALPINE

FROM golang:${GO}-alpine${ALPINE}

RUN apk add --no-cache make git py-pip bash curl && \
    pip install awscli

