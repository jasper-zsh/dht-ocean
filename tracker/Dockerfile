FROM golang:alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GOARCH amd64

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
COPY tracker/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/tracker tracker/tracker.go


FROM alpine

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=builder /app/tracker /app/tracker
COPY --from=builder /app/etc /app/etc

CMD ["./tracker", "-f", "etc/tracker.yaml"]
