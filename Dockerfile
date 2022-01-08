FROM golang:alpine AS builder

WORKDIR /cfwidget
COPY . .

RUN go build -o /go/bin/cfwidget github.com/lordralex/cfwidget

FROM alpine
COPY --from=builder /go/bin/cfwidget /go/bin/cfwidget

WORKDIR /updatejson

EXPOSE 8080

ENV REDIS_HOST="redis:6379" \
    DB_HOST="" \
    DB_USER="" \
    DB_PASS="" \
    DB_DATABASE=""

ENTRYPOINT ["/go/bin/cfwidget"]
CMD [""]