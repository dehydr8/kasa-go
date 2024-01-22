FROM golang:1.21 as build

WORKDIR /go/src/github.com/dehydr8/kasa-go
COPY . .
RUN CGO_ENABLED=0 go build -o /kasa-exporter

FROM alpine
COPY --from=build /kasa-exporter /kasa-exporter
ENTRYPOINT ["/kasa-exporter"]