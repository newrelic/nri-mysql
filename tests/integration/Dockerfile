FROM golang:1.11 as builder
WORKDIR /go/src/github.com/newrelic/nri-mysql
COPY . .
RUN make compile

FROM alpine:latest
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/bin /
CMD ["sleep", "1h"]



