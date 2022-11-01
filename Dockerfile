FROM golang:1.16 as build

WORKDIR /go/src/github.com/wymangr/blueiris_exporter

COPY ./go.mod /go/src/github.com/wymangr/blueiris_exporter
COPY ./go.sum /go/src/github.com/wymangr/blueiris_exporter
COPY ./blueiris_exporter.go /go/src/github.com/wymangr/blueiris_exporter
COPY ./metrics.go /go/src/github.com/wymangr/blueiris_exporter
COPY ./common /go/src/github.com/wymangr/blueiris_exporter/common
COPY ./blueiris /go/src/github.com/wymangr/blueiris_exporter/blueiris

RUN go build

FROM debian:buster-slim

COPY --from=build /go/src/github.com/wymangr/blueiris_exporter/blueiris_exporter /

EXPOSE 2112
ENTRYPOINT ["/blueiris_exporter"]