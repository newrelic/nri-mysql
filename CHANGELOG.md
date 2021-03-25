# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## 1.5.1 (2021-03-25)
### Fixed

- Fixed a bug that caused rows to not be fully consumed (and therefore a panic) in some rare circumstances (https://github.com/newrelic/nri-mysql/pull/77).

## 1.5.0 (2020-01-14)
### Added
- Metrics for commits and rollbacks.

## 1.4.0 (2019-11-18)
### Changed
- Renamed the integration executable from nr-mysql to nri-mysql in order to be consistent with the package naming. **Important Note:** if you have any security module rules (eg. SELinux), alerts or automation that depends on the name of this binary, these will have to be updated.
## 1.3.0 (2019-04-29)
### Added
- Upgraded to SDK v3.1.5. This version implements [the aget/integrations
  protocol v3](https://github.com/newrelic/infra-integrations-sdk/blob/cb45adacda1cd5ff01544a9d2dad3b0fedf13bf1/docs/protocol-v3.md),
  which enables [name local address replacement](https://github.com/newrelic/infra-integrations-sdk/blob/cb45adacda1cd5ff01544a9d2dad3b0fedf13bf1/docs/protocol-v3.md#name-local-address-replacement).
  and could change your entity names and alarms. For more information, refer
  to:

  - https://docs.newrelic.com/docs/integrations/integrations-sdk/file-specifications/integration-executable-file-specifications#h2-loopback-address-replacement-on-entity-names
  - https://docs.newrelic.com/docs/remote-monitoring-host-integration://docs.newrelic.com/docs/remote-monitoring-host-integrations

## 1.2.0 (2019-04-08)
### Added
- Upgraded to SDKv3
- Remote monitoring option. It enables monitoring multiple instances, 
  more information can be found at the [official documentation page](https://docs.newrelic.com/docs/remote-monitoring-host-integrations).
- Restored `NRIA_CACHE_PATH` legacy environment variable, for backwards-compatibility.
- Upgrade sql driver to support caching_sha2_password (MySQL 8 default) and sha256_password authentication support

## 1.1.5 (2018-12-05)

### Fixed
- Issue where the plugin returned metrics when only inventory was requested.

## 1.1.4 (2018-10-19)

### Bugs fixed

#### Allow rate and deltas

- Metrics of type rate and delta cannot be added unless there is a namespacing attribute on the metric-set.

## 1.1.3 (2018-10-12)

### Changed:

####  Update to SDKv3

- Updated the integration code from the previous version of the SDK to [SDK v3](https://github.com/newrelic/infra-integrations-sdk/#upgrading-from-sdk-v2-to-v3).

### Added:

#### Old password support

- Previously when trying to run Mysql integration on a mysql-server with old password support https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords integration won't run. If customer has set old_passwords=1 at the MySQL server integration can now connect to it.

#### Ignore bin folder

`bin` folder added to `.gitignore`.

### Bugs fixed

#### Hostname issue 
- Integration CLI-arguments are no longer overridden by environment-variables
As CLI-args are used by the NRI agent `HOSTNAME` that is a common bash env-var is not used to set MySql-host unless no hostname is provided via config.

## 1.1.2 (2018-10-16)
### Added
- Included metric `Master_Host`

### Changed
- Fixed code dependencies

## 1.1.1 (2018-09-07)
### Changed
- Updated Makefile

## 1.1.0 (2018-08-02)
### Added
- Added contributing information
- Added packaging script

### Changed
- Updated Makefile

## 1.0.0 (2018-07-24)
### Added
- Initial release, which contains inventory and metrics data
