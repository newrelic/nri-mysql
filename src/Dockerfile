FROM golang:1.9 as builder
RUN go get -d github.com/newrelic/nri-mysql/... && \
    cd /go/src/github.com/newrelic/nri-mysql && \
    make && \
    strip ./bin/nr-mysql

FROM newrelic/infrastructure:latest
COPY . .
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/bin/nr-mysql /var/db/newrelic-infra/newrelic-integrations/bin/nr-mysql
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/mysql-definition.yml /var/db/newrelic-infra/newrelic-integrations/mysql-definition.yml
