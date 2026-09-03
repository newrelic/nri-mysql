FROM mariadb:12.3.3
# Create and set permissions for log directory
RUN mkdir -p /var/log/mysql && \
    chown -R mysql:mysql /var/log/mysql
COPY tests/integration/mysql-performance-config/versions/mariadb-11.4.10/mariadb.conf.cnf /etc/mysql/my.cnf
COPY tests/integration/mysql-performance-config/init.sql /etc/mysql/init.sql


# Update package lists and install necessary tools
RUN apt-get update && \
    apt-get install -y curl unzip tmux && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create MySQL compatibility symlinks for MariaDB 11.4.10
RUN ln -s /usr/bin/mariadb /usr/bin/mysql && \
    ln -s /usr/bin/mariadb-admin /usr/bin/mysqladmin && \
    ln -s /usr/sbin/mariadbd /usr/sbin/mysqld

# Set the working directory to /tmp
WORKDIR /tmp

# Download and unzip the test_db ZIP archive
RUN curl -L https://github.com/datacharmer/test_db/archive/refs/heads/master.zip -o test_db-master.zip \
    && unzip -o test_db-master.zip \
    && mv test_db-master test_db

# Set the working directory
WORKDIR /tmp/test_db

# Copy the custom entrypoint script to the image
COPY tests/integration/mysql-performance-config/custom-entrypoint.sh /usr/local/bin/custom-entrypoint.sh
RUN chmod +x /usr/local/bin/custom-entrypoint.sh

ENV MYSQL_ROOT_PASSWORD=""
ENV CONTAINER_NAME=mariadb_perf_11-4
# Replace the default entrypoint with the custom script
ENTRYPOINT ["/usr/local/bin/custom-entrypoint.sh"]
