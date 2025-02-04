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

1. Comment the `mysql_master-5-7-35`, `mysql_slave-5-7-35` services in the [docker compose](./docker-compose.yml) as mysql doesn't support arm images for this version.
2. Also comment the lines 57-60 in [integration test file](./integration_test.go).
3. Make you have docker installed and running in the mac.
4. In [Makefile](../../Makefile) line 31 change `@go test` -> `@go test -v` to see the output of tests running.
4. Run the command `make integration-test`.

## Steps to run Integration in local using docker

1. Run the following commands
    - `cd test/integration`
    - `docker compose -f docker-compose-performance.yml build --no-cache`
    - `docker compose -f docker-compose-performance.yml up`
    - once all the containers are up and running procced to next steps. (verify by checking the last log of the mysql containers it should be `finished executing blocking session queries`)
    - `chmod +x mysql-performance-config/block.sh`
    - `./block.sh` executing this file will create blocking sessions in `mysql_8-0-40` server
2. In the integration_nri-mysql_perf_1 docker container shell execute the integration using the following command
    - `./nri-mysql -username=root -password=DBpwd1234 -hostname=mysql_8-0-40 -port=3306 -verbose=true -enable_query_performance=true -slow_query_fetch_interval=300`
3. Change the hostname, enable_query_performance, slow_query_fetch_interval flags to see the integrations stdout for different scenarios


## Performance Integration test setup

1. A custom image is built for mysql server to enable performance extensions/flags [Dockerfile](./mysql-performance-config/versions/8.0.40/Dockerfile)
2. The entrypoint of the custom image is modified to populate sample data in the mysql server, execute slow queries and blocking sessions queries
3. These custom Dockerfiles are used in [Docker Compose Performance](./docker-compose-performance.yml)
4. Once the mysql containers are up and running.
5. [Performance Integration tests](./performance_integration_test.go) executes the binary of the nri-mysql integration with the above mysql container details and validate if the six output jsons mach the [defined schemas](./json-schema-performance-files/).

#### Note: We have separate [docker compose](./docker-compose-performance.yml) for performance metrics integration testing because:
1. Tests should run against multiple mysql server versions list below
    1. 5.7.35(master, slave)
    2. 8.0.40(master, slave, perf)
    3. 8.4(master, perf)
    4. 9.1(master, slave, perf)
2. Including all these containers under the same [docker compose](./docker-compose.yml) was crashing few of the mysql version containers
3. So, separate containers were created for the performance testing under [docker compose](./docker-compose-performance.yml) file
4. The [Makefile](../../Makefile) `integration-test` target brings up the docker compose containers, executes the integrations test, brings down the containers. Next brings up the docker compose performance containers, extecutes performance integration tests, brings down the containers. This resolved the crashing of few mysql version containers