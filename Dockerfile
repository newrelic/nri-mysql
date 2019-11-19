FROM golang:1.9 as builder
COPY . /go/src/github.com/newrelic/nri-mysql/
RUN cd /go/src/github.com/newrelic/nri-mysql && \
    make && \
    strip ./bin/nri-mysql

FROM newrelic/infrastructure:latest
ENV NRIA_IS_FORWARD_ONLY true
ENV NRIA_K8S_INTEGRATION true
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/bin/nri-mysql /nri-sidecar/newrelic-infra/newrelic-integrations/bin/nri-mysql
COPY --from=builder /go/src/github.com/newrelic/nri-mysql/mysql-definition.yml /nri-sidecar/newrelic-infra/newrelic-integrations/definition.yaml
USER 1000
