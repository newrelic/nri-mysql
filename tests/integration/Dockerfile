FROM ubuntu:18.04

COPY target/packages/deb/nri-mysql_0.0.0-dev_amd64.deb /nri-mysql.deb
RUN dpkg --ignore-depends=newrelic-infra -i /nri-mysql.deb
COPY tests/integration/mysql-config.yml /etc/newrelic-infra/integrations.d/

CMD ["/var/db/newrelic-infra/newrelic-integrations/bin/nr-mysql"]



