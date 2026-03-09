# Changelog

All notable changes to this project are documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for public releases.

## [0.1.1] - 2026-03-09

### Added
- Added `SUPPORT.md` to define the community support boundary.
- Added `.github/ISSUE_TEMPLATE/config.yml` for cleaner issue routing.
- Added English and Chinese open-source launch update documents.
- Added English and Chinese launch messaging kit documents.
- Added `docs/README.md`, `docs/README_CN.md`, and documentation index pages for working docs navigation.

### Changed
- Simplified `README.md` and `README_CN.md` documentation entry points.
- Updated repository community guidance to direct readers toward support and documentation index pages.

## [0.1.0] - 2026-03-09

### Added
- Published the open-source repository core under `Apache-2.0`.
- Added community health files: `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue templates, and pull request template.
- Added platform closure across orchestration, auth, audit, scheduling, action bridge, and skill resolution.
- Added LLM-backed agent loop and tool-calling support on the main execution path.
- Added API-level audit, replay, and SSE telemetry surfaces.

### Changed
- Updated `README.md` and `README_CN.md` to present AgentOS as a community-core open-source execution platform.

### Verified
- `go test ./... -count=1`
- `./scripts/acceptance.sh`
