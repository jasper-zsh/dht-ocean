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
COPY crawler/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/crawler crawler/crawler.go

COPY ocean/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/ocean ocean/ocean.go

COPY proxy/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/proxy proxy/proxy.go

COPY tracker/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/tracker tracker/tracker.go

FROM --platform=linux/amd64 alpine

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=builder /app

CMD ["./crawler", "-f", "etc/crawler.yaml"]
