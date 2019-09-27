#!/bin/bash -e

# credit to https://github.com/wixyvir/docker-mysql-tls

/usr/bin/mysql_ssl_rsa_setup --datadir=/shared --suffix=mysql-tls

echo "======= MYSQL CLIENT KEY ========"
cat /shared/client-key.pem

echo "======= MYSQL CLIENT CERT ========"
cat /shared/client-cert.pem

echo "======= MYSQL CA ========"
cat /shared/ca.pem

cat << EOF > /etc/mysql/mysql.conf.d/ssl.cnf
[mysqld]
ssl-ca=/shared/ca.pem
ssl-cert=/shared/server-cert.pem
ssl-key=/shared/server-key.pem
EOF
