# Changelog

All notable changes to the Chargebee CLI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-03-16

### Added
- `--type` filter for items resource (plan, addon, charge)
- Filter validation with clear error messages listing accepted values for status and type filters

### Changed
- `--limit` flag now enforces the Chargebee API maximum of 100

### Removed
- Legacy `plans` and `addons` resources — use `items` and `item-prices` instead

## [0.1.4] - 2026-03-16

### Added
- `cb entitlements` command for inspecting feature entitlements across subscriptions
  - Customer lookup by ID, company name, first name, or email
  - Feature lookup with exact or prefix name matching
- `--raw` flag for JSON output in entitlements command

## [0.1.3] - 2026-03-12

### Changed
- Help text for `--after`/`--before` flags now includes format examples

### Fixed
- Empty list results now show "No results." instead of blank output
- Update notification throttled to once per 24 hours

## [0.1.1] - 2026-03-11

### Changed
- Improved CLI help text

## [0.1.0] - 2026-03-11

### Added
- `cb login` / `cb logout` — authentication with profile support
- `cb config` — get, set, and list configuration
- `cb listen` — poll Chargebee events and forward webhooks locally
- `cb trigger` — trigger test events with PC1/PC2 fixture support
- `cb completion` — shell completions for bash, zsh, fish, powershell
- Homebrew formula distribution

[Unreleased]: https://github.com/jhuiting/chargebee-cli/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/jhuiting/chargebee-cli/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/jhuiting/chargebee-cli/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/jhuiting/chargebee-cli/compare/v0.1.1...v0.1.3
[0.1.1]: https://github.com/jhuiting/chargebee-cli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jhuiting/chargebee-cli/releases/tag/v0.1.0
