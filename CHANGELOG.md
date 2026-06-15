# Changelog

## [0.2.0](https://github.com/pixelvide/aegis/compare/v0.1.1...v0.2.0) (2026-06-15)


### ⚠ BREAKING CHANGES

* **api:** The X-Org-ID header is no longer accepted. Org context is now resolved exclusively from the request Host header (subdomain or custom domain). AEGIS_BASE_DOMAIN defaults to lvh.me for local dev.

### Features

* **agent:** add parallel scan pipeline with chunking, dedup, and tiered models ([a1930b4](https://github.com/pixelvide/aegis/commit/a1930b439d11c4a76bde8a37f16b28976a249d43))
* **api,ui:** add scan lifecycle endpoints and scans page ([764a94f](https://github.com/pixelvide/aegis/commit/764a94f32a0c644274e7351201a092825866ab3f))
* **api:** add MFA brute-force protection with Valkey-backed session allowlist ([45248e8](https://github.com/pixelvide/aegis/commit/45248e86e86d7c858d713735a6f7fc4b188ae815))
* **api:** enforce require_mfa org feature flag in TenantResolver ([6cca496](https://github.com/pixelvide/aegis/commit/6cca496b4902883fde0456b58258fa5cfa776427))


### Bug Fixes

* **api:** add error logging to agent ingest and fix upsert table qualifier ([b7f1342](https://github.com/pixelvide/aegis/commit/b7f134295ec47a7686ff4aca24cfad4e775f2bab))
* **ci:** remove LOCALHARNESS_PAT — localharness is a public repo ([cb3db7d](https://github.com/pixelvide/aegis/commit/cb3db7d773acade37cc95fdf5448b8fcb217f6ec))
* **ui:** remove non-functional UI elements and fix broken links ([d0c77d5](https://github.com/pixelvide/aegis/commit/d0c77d588c2ac4e661d7e684d666781bdd36480f))
* **ui:** wait for project context before fetching findings data ([2d85110](https://github.com/pixelvide/aegis/commit/2d85110d26631c9e77c195d558b54116578d7443))


### Documentation

* update all references from X-Org-Slug to X-Org-ID ([a5fde0b](https://github.com/pixelvide/aegis/commit/a5fde0bb17b999798f1754a0a31af055e4ae1cf1))


### Miscellaneous

* **api:** remove X-Org-ID header, resolve orgs from subdomain only ([b2e2b34](https://github.com/pixelvide/aegis/commit/b2e2b34c804762639fbcd907bfbc3e8dfd1ed22a))

## [0.1.1](https://github.com/pixelvide/aegis/compare/v0.1.0...v0.1.1) (2026-06-12)


### Features

* **agent:** add scanning agent with server reporting ([f2579ca](https://github.com/pixelvide/aegis/commit/f2579ca098a3660001e2663c9907e6f7b0a2b505))
* **api:** standardized response envelope, structured error codes, and request trace IDs ([27d4dbb](https://github.com/pixelvide/aegis/commit/27d4dbbc5710316e3459fe5f5d48eb39e8cd8164))
* base-domain auth restriction, email templates, structured logging, and UI improvements ([6f121b7](https://github.com/pixelvide/aegis/commit/6f121b70eb01f9bd73a42e4b552dcfe3d0aa862d))
* initial release — multi-tenant security platform with agent ingest API ([9773107](https://github.com/pixelvide/aegis/commit/97731073d87861cb3fc18dab80845d1526cc6b01))
* org feature flags, token management, settings redesign, and roadmap overhaul ([64b11b7](https://github.com/pixelvide/aegis/commit/64b11b7dd0ed04c0be612d66694d56a5a3225dc1))
* **ui:** add .env snippet to API tokens page and fix clipboard on HTTP ([702c3f2](https://github.com/pixelvide/aegis/commit/702c3f2bc179db76062b06a7493a52bff49a47d2))
* **ui:** add org creation dialog and post-login subdomain redirect ([329fcfb](https://github.com/pixelvide/aegis/commit/329fcfbc4a18459cbe21acd34cbbe84d648d06a0))
* **ui:** improve UX with sonner toasts, shadcn alerts, password modal, and auto-logout ([827ab9f](https://github.com/pixelvide/aegis/commit/827ab9f27859dfaeb0cee0ccb113ccfa351ad90e))


### Bug Fixes

* **ci:** use pnpm and Node 24 in UI lint & build job ([58f9c8e](https://github.com/pixelvide/aegis/commit/58f9c8e055b594ca0047b45121949b47f8a8751b))
* correct SMTP_HOST default for Docker Compose ([46fbd7d](https://github.com/pixelvide/aegis/commit/46fbd7d443a36d4cbd534b2ad6512b11b8f61274))


### Documentation

* add infrastructure route tier, API standardization roadmap, and updated docs ([00afd8a](https://github.com/pixelvide/aegis/commit/00afd8a2103e35ed3733c0f179b5379e9a1917b8))
* update agent and deployment docs for aegis-agent image ([a7fa321](https://github.com/pixelvide/aegis/commit/a7fa32153c989379c0bb43d6a89e9559656697e8))
