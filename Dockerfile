FROM golang:1.12-alpine
RUN apk update
RUN apk add git
RUN apk add tzdata
RUN apk add build-base
RUN cp /usr/share/zoneinfo/America/Denver /etc/localtime
ADD root /var/spool/cron/crontabs/root
RUN mkdir -p /go/src/metrics-sample
ADD metrics-sample.go  /go/src/metrics-sample/metrics-sample.go
ADD build.sh /build.sh
RUN chmod +x /build.sh
RUN /build.sh
CMD ["crond", "-f"]

