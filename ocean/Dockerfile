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
COPY ocean/etc /app/etc
RUN go build -ldflags="-s -w" -o /app/ocean ocean/ocean.go


FROM --platform=linux/amd64 alpine

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=builder /app/ocean /app/ocean
COPY --from=builder /app/etc /app/etc

CMD ["./ocean", "-f", "etc/ocean.yaml"]
