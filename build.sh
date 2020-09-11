#!/bin/sh

cd /go/src
go get "gopkg.in/shopify/sarama.v1"
cd /go/src/metrics-sample
go build metrics-sample.go