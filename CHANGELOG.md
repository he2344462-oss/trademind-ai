# Changelog

All notable changes to TradeMind are documented here.

## Unreleased

### Runtime baseline and Collector security (2026-08-12)

- Aligned local documentation, CI, and Docker builds on Node.js 24 LTS, pnpm 9.15.4, and Go 1.25.x.
- Unified Compose PostgreSQL bootstrap credentials with backend `DB_*` settings and limited infrastructure/Collector host port publishing to loopback.
- Added fail-closed Backend-to-Collector token authentication, loopback-by-default native binding, and bounded Collector request bodies.
- Added Collector and backend regression coverage for the internal authentication contract.

### Admin theme (2026-08-09)

- Added an icon-only, tooltip-labelled top-navigation light/dark theme switch with light mode as the default and local preference persistence.
- Applied Ant Design theme tokens across shared Admin chrome, login, dashboard, status surfaces, and responsive regression coverage.
- Consolidated mobile brand, theme, and account actions into one fixed header, kept the full desktop brand in the sidebar only, and made the navigation drawer opaque above scrolled content.
- Made theme switching atomic and fully reversible for header, elevated, and portal surfaces, and safely centered mobile login and registration layouts.

### Production maintenance cleanup (2026-08-09)

- Removed historical phase gates, load-test harnesses, generated evidence, one-off acceptance scripts, and local Playwright/test outputs from the working tree.
- Removed residual P6/P7 verification commands, unreferenced backend placeholders, a one-off Admin codemod, and unused Admin/Collector symbols.
- Deduplicated the Admin brand image, aligned Collector Playwright dependencies, and added tracked documentation-path checks to CI.
- Kept GitHub Actions and their frontend, collector, backend, contract, architecture, PostgreSQL, Redis, and Admin E2E regression dependencies.
- Replaced the historical PostgreSQL phase wrapper with direct, isolated CI inventory integration commands.
- Adopted GitHub Actions for automated regression and human sign-off for product acceptance.
- Documented that a local test database is optional and is not recreated automatically; CI provisions isolated service containers.

## v0.1.0

- Initial TradeMind monorepo foundation with Go backend, React Admin, Node collector, PostgreSQL, Redis, Docker Compose, Provider abstractions, and open-source governance.
