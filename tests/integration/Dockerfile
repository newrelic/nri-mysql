FROM golang:1.10 as builder
ARG CGO_ENABLED=0
WORKDIR /go/src/github.com/newrelic/nri-mysql
COPY . .
RUN make clean compile

FROM alpine:latest
RUN apk add ca-certificates
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/bin /
CMD ["sleep", "1h"]
