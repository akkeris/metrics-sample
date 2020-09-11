FROM golang:1.12-alpine
RUN apk update
RUN apk add git
RUN apk add tzdata
RUN apk add build-base
RUN cp /usr/share/zoneinfo/America/Denver /etc/localtime
RUN mkdir -p /go/src/metrics-sample
WORKDIR /go/src/github.com/akkeris/metrics-sample
ENV GO111MODULE=on
ADD metrics-sample.go metrics-sample.go
ADD go.mod go.mod
ADD go.sum go.sum
RUN go build metrics-sample.go
CMD ["./metrics-sample"]