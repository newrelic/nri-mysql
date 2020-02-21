# New Relic Infrastructure Integration for MySQL
New Relic Infrastructure Integration for MySQL captures critical performance metrics and inventory reported by MySQL database.

Data is obtained by querying directly the database for its status and configuration variables to build the reported metrics and inventory.

See our [documentation web site](https://docs.newrelic.com/docs/integrations/host-integrations/host-integrations-list/mysql-monitoring-integration) for more details.

<!---
See [metrics]() or [inventory]() for more details about collected data and review [dashboard]() in order to know how the data is presented.
--->

## Configuration
It is required to create a user with [replication privilege](https://dev.mysql.com/doc/refman/5.7/en/privileges-provided.html#priv_replication-client). Execute the following command, replacing \<SET_PASSWORD> with selected password.
```bash
$ sudo mysql -e "CREATE USER 'newrelic'@'localhost' IDENTIFIED BY '<SET_PASSWORD>';"
$ sudo mysql -e "GRANT REPLICATION CLIENT ON *.* TO 'newrelic'@'localhost' WITH MAX_USER_CONNECTIONS 5;"
```

## Installation
* download an archive file for the MySQL Integration
* extract `mysql-definition.yml` and `/bin` directory into `/var/db/newrelic-infra/newrelic-integrations`
* add execute permissions for the binary file `nri-mysql` (if required)
* extract `mysql-config.yml.sample` into `/etc/newrelic-infra/integrations.d`

## Usage
This is the description about how to run the MySQL Integration with New Relic Infrastructure agent, so it is required to have the agent installed (see [agent installation](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/installation/install-infrastructure-linux)).

In order to use the MySQL Integration it is required to configure `mysql-config.yml.sample` file. Firstly, rename the file to `mysql-config.yml`. Then, depending on your needs, specify all instances that you want to monitor with correct credentials. Once this is done, restart the Infrastructure agent.

You can view your data in Insights by creating your own custom NRQL queries. To do so use the **MysqlSample** event type.

## Custom Queries

To add custom queries, use the **-custom_metrics_query** option to provide a single query, or the **-custom_metrics_config** option to specify a YAML file with one or more queries, such as the sample `mysql-custom-query.yml.sample`

### How attributes are named

Each query that returns a table of values will be parsed row by row, adding the **MysqlCustomQuerySample** event as follows:

- The column name is the attribute name
- Each row value in that column is the attribute value
- The metric type is auto-detected whether it is a number (type GAUGE), or a string (type ATTRIBUTE)

One customizable attribute in each row can be configured by database values using the following names:

- The column `metric_name` specifies its attribute name
- The column `metric_value` specifies its attribute value
- The column `metric_type` specifies its metric type, i.e. `gauge` or `attribute`

For example, the following query makes attributes named `diagnostics.allow_i_s_tables`, `diagnostics.include_raw`, `ps_thread_trx_info.max_length` and so on.
```sql
SELECT variable AS metric_name, value AS metric_value FROM sys.sys_config
```

### Specifying queries in YAML

When using a YAML file containing queries, you can specify the following parameters for each query:

- `query` (required) contains the SQL query
- `database` (optional) adds the database name as an attribute
- `prefix` (optional) prefix to prepend to the attribute name
- `metric_name` (optional) specify the name for the customizable attribute
- `metric_type` (optional) specify the metric type for the customizable attribute

## Integration development usage
Assuming that you have the source code and Go tool installed you can build and run the MySQL Integration locally.
* Go to the directory of the MySQL Integration and build it
```bash
$ make
```
* The command above will execute the tests for the MySQL Integration and build an executable file called `nri-mysql` under `bin` directory. Run `nri-mysql` with parameters specifying username and password
```bash
$ ./bin/nri-mysql -username <username> -password <password>
```
* If you want to know more about usage of `./bin/nri-mysql` check
```bash
$ ./bin/nri-mysql -help
```

For managing external dependencies [govendor tool](https://github.com/kardianos/govendor) is used. It is required to lock all external dependencies to specific version (if possible) into vendor directory.

### Integration testing

Requirements:

* `docker` and `docker-compose` commands

To run integration tests:

```
$ make integration-test
```

## Contributing Code

We welcome code contributions (in the form of pull requests) from our user
community. Before submitting a pull request please review [these guidelines](https://github.com/newrelic/nri-mysql/blob/master/CONTRIBUTING.md).

Following these helps us efficiently review and incorporate your contribution
and avoid breaking your code with future changes to the agent.

## Custom Integrations

To extend your monitoring solution with custom metrics, we offer the Integrations
Golang SDK which can be found on [github](https://github.com/newrelic/infra-integrations-sdk).

Refer to [our docs site](https://docs.newrelic.com/docs/infrastructure/integrations-sdk/get-started/intro-infrastructure-integrations-sdk)
to get help on how to build your custom integrations.

## Support

You can find more detailed documentation [on our website](http://newrelic.com/docs),
and specifically in the [Infrastructure category](https://docs.newrelic.com/docs/infrastructure).

If you can't find what you're looking for there, reach out to us on our [support
site](http://support.newrelic.com/) or our [community forum](http://forum.newrelic.com)
and we'll be happy to help you.

Find a bug? Contact us via [support.newrelic.com](http://support.newrelic.com/),
or email support@newrelic.com.

New Relic, Inc.
