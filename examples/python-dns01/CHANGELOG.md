# Changelog

All notable changes to this example plugin are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project uses [semantic versioning](https://semver.org/).

## [0.1.0] - 2026-09-22

### Added

- Initial reference implementation: `plugin.initialize`, `plugin.configure`,
  `plugin.ping`, `plugin.shutdown`, `plugin.exit`, `dns01.present` and
  `dns01.cleanup` for the fictional MyDNS API, in pure Python 3 standard
  library.
- `test_main.py`: subprocess-driven NDJSON test covering initialize, ping,
  an unknown method, and exit.
