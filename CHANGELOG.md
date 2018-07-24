# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## 1.1.0 (2017-10-16)
### Added
- Include replica metrics

## 0.2.0 (2017-06-06)
### Added
- New license file

### Changed
- Update vendored dependencies
- Update inventory prefix to `config/`

### Fixed
- Fix functions in metrics definition, fixing "divide by zero" errors.
- Fix typo in password help message
- Use correct case for `software.edition`, `software.version` and `cluster.nodeType`
- Change metrics that should be GAUGEs

## 0.1.0 (2017-05-16)
### Added
- Initial release, which contains inventory and metrics data
