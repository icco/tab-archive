FROM golang:1.15-alpine as builder

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"
RUN apk add --no-cache git make g++ ca-certificates

WORKDIR /go/src/github.com/icco/tab-archive
COPY . .

RUN go build -v -o /go/bin/server .

CMD ["/go/bin/server"]
