FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

RUN apk add clang lld bash
COPY --from=xx / /

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

WORKDIR /cfwidget

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

ARG TARGETPLATFORM

RUN xx-apk add musl-dev gcc
RUN xx-go build -buildvcs=false -o /go/bin/cfwidget github.com/cfwidget/cfwidget
RUN xx-verify /go/bin/cfwidget

FROM alpine

WORKDIR /cfwidget

COPY --from=builder /go/bin/cfwidget /go/bin/cfwidget

EXPOSE 8080

ENV DB_HOST="" \
    DB_USER="" \
    DB_PASS="" \
    DB_DATABASE="" \
    DB_DEBUG="false" \
    CACHE_TTL="1h" \
    CORE_KEY_FILE="/run/secrets/core_key" \
    CORE_KEY="" \
    API_HOSTNAME="api.localhost:8080" \
    DEBUG="false" \
    GIN_MODE="release"

ENTRYPOINT ["/go/bin/cfwidget"]
CMD [""]