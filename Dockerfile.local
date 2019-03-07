FROM alpine:3.7 AS builder

RUN apk add --update --no-cache ca-certificates


FROM alpine:3.7

RUN apk add --update libcap && rm -rf /var/cache/apk/*

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ARG BUILD_DIR
ARG BINARY_NAME

COPY $BUILD_DIR/$BINARY_NAME /usr/local/bin/spot-config-webhook

USER nobody

ENTRYPOINT ["/usr/local/bin/spot-config-webhook"]
