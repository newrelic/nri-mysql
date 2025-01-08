# Integration tests

Steps to update the integration tests for the latest supported version:

1. Update the mysql image in docker file that is mentioned under `mysql_master-latest-supported`, `mysql_slave-latest-supported` of the [docker compose](./docker-compose.yml).
2. Execute the integration tests
    * If the JSON-schema validation fails:
        - Check the inventory, some server settings might have been removed.
        - Check the number of entities: the query schema or variable names might change (metrics failures).
        - Check the release notes ([MySQL 8.0.23 example](https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-23.html))
3. Once the failures are understood (if any), update the corresponding JSON-schema files, you may need to generate it
   using the integration output, specially if there is any metric failure.

## Steps to run Integration tests locally on Mac

1. Comment the `mysql_master-5-7-35`, `mysql_slave-5-7-35` services in the [docker compose](./docker-compose.yml).
2. Also comment the lines 53-57 in [integration test file](./integration_test.go).
3. Make you have docker installed and running in the mac.
4. In [Makefile](../../Makefile) line 31 change `@go test` -> `@go test -v` to see the output of tests running.
4. Run the command `make integration-test`.
