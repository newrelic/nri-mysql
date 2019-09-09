FROM golang:1.9 as builder
RUN go get -d github.com/newrelic/nri-mysql/... && \
    cd /go/src/github.com/newrelic/nri-mysql && \
    make && \
    strip ./bin/nr-mysql

FROM newrelic/infrastructure:latest
ENV NRIA_IS_FORWARD_ONLY true
ENV NRIA_K8S_INTEGRATION true
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/bin/nr-mysql /nri-sidecar/newrelic-infra/newrelic-integrations/bin/nr-mysql
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/mysql-definition.yml /nri-sidecar/newrelic-infra/newrelic-integrations/definition.yaml
USER 1000
