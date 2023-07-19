# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

Unreleased section should follow [Release Toolkit](https://github.com/newrelic/release-toolkit#render-markdown-and-update-markdown)

## Unreleased
### enhancement
 - bumped golang version pinning 1.20.6

## 1.9.0 (2023-06-06)
### Changed
- Update Go version to 1.20

## 1.8.1  (2022-06-27)
### Changed
- Bump dependencies
### Added
Added support for more distributions:
- RHEL(EL) 9
- Ubuntu 22.04

## 1.8.0  (2022-03-08)
### Added
- `mysql-log.yml.example` is now in Linux packages to help setting up log parsing.

## 1.7.1  (2021-10-20)
### Added
Added support for more distributions:
- Debian 11
- Ubuntu 20.10
- Ubuntu 21.04
- SUSE 12.15
- SUSE 15.1
- SUSE 15.2
- SUSE 15.3
- Oracle Linux 7
- Oracle Linux 8

## 1.7.0  (2021-06-27)
### Added
- New parameter for MySQL local socket connection. Local socket connection is secure so MySQL will not complain about non-secure connection when `require_secure_transport = ON`.

- Moved default config.sample to [V4](https://docs.newrelic.com/docs/create-integrations/infrastructure-integrations-sdk/specifications/host-integrations-newer-configuration-format/), added a dependency for infra-agent version 1.20.0

Please notice that old [V3](https://docs.newrelic.com/docs/create-integrations/infrastructure-integrations-sdk/specifications/host-integrations-standard-configuration-format/) configuration format is deprecated, but still supported.


### Fix
- Detection of slave running for all mysql versions
- IPv6 address URI formation

## 1.6.1 (2021-06-08)
### Changed
- Support for ARM

## 1.6.0 (2021-05-05)
## Changed
- Update Go to v1.16.
- Migrate to Go Modules
- Update Infrastracture SDK to v3.6.7.
- Update other dependecies.
- Improve logs

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
