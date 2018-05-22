#!/bin/sh

cd /go/src
go get "github.com/stackimpact/stackimpact-go"
go get "gopkg.in/Shopify/sarama.v1"
go get "github.com/akkeris/vault-client"
cd /go/src/metrics-sample
go build metrics-sample.go

