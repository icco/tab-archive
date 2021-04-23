FROM golang:1.16-alpine as builder

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"
RUN apk add --no-cache git

WORKDIR /go/src/github.com/icco/tab-archive
COPY . .

RUN go build -v -o /go/bin/server .

CMD ["/go/bin/server"]
