FROM golang:1.11-alpine AS builder

RUN apk add --update --no-cache ca-certificates make git curl mercurial

ARG PACKAGE=github.com/banzaicloud/spot-config-webhook

RUN mkdir -p /go/src/${PACKAGE}
WORKDIR /go/src/${PACKAGE}

COPY Gopkg.* Makefile /go/src/${PACKAGE}/
RUN make vendor

COPY . /go/src/github.com/banzaicloud/spot-config-webhook
RUN BUILD_DIR=/tmp make build-release


FROM alpine:3.7

RUN apk add --update libcap && rm -rf /var/cache/apk/*

COPY --from=builder /tmp/spot-config-webhook /usr/local/bin/spot-config-webhook
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nobody

ENTRYPOINT ["/usr/local/bin/spot-config-webhook"]
