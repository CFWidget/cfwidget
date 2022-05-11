FROM golang:1.17-alpine AS builder

WORKDIR /cfwidget
COPY . .

RUN go build -o /go/bin/cfwidget github.com/lordralex/cfwidget

FROM alpine

WORKDIR /cfwidget

COPY --from=builder /go/bin/cfwidget /go/bin/cfwidget
COPY --from=builder /cfwidget/favicon.ico /cfwidget/favicon.ico
COPY --from=builder /cfwidget/css /cfwidget/css
COPY --from=builder /cfwidget/js /cfwidget/js
COPY --from=builder /cfwidget/templates /cfwidget/templates

EXPOSE 8080

ENV DB_HOST="" \
    DB_USER="" \
    DB_PASS="" \
    DB_DATABASE="" \
    DB_DEBUG="false" \
    CACHE_TTL="5m" \
    CORE_KEY_FILE="/run/secrets/core_key" \
    CORE_KEY="" \
    WEB_HOSTNAME="www.localhost" \
    DEBUG="false" \
    GIN_MODE="release"

ENTRYPOINT ["/go/bin/cfwidget"]
CMD [""]