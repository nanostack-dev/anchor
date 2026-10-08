
# 🚀 Organization Infrastructure Service (Anchor OaaS) 🚀

[![License: FSL-1.1-ALv2](https://img.shields.io/badge/License-FSL--1.1--ALv2-blue.svg)](./LICENSE)

**Tired of rebuilding multi-tenancy, organizational structures, user roles, and permissions for every new product?** This project provides a foundational, source-available service to manage it all, letting you focus on your core application logic.

Built for developers and organizations creating multi-product platforms or complex internal tools, Anchor offers a robust solution designed for a single `PlatformTenant` deployment, with an architecture ready for multi-tenant SaaS scaling.

## ✨ Key Features

* **Hierarchical Structure:** Manage `Product` -> `Organization` -> `Workspace` relationships out-of-the-box.
* **User Directory:** Separate management for `PlatformUser` (admins) and `ProductUser` (end-users, directory-only).
* **Flexible Roles & Permissions:** Define custom `AppRole`s and `AppPermission` strings per `Product`.
* **Granular Access Control:** Assign roles to users at the `Organization` and `Workspace` levels. Check effective permissions easily via API.
* **API Key Management:** Secure backend access for your products with Product-specific API Keys carrying fine-grained permissions.
* **API-First Design:** Defined with OpenAPI 3.0, using KSUID identifiers.
* **Built for Scale:** Go-based, stateless design, PostgreSQL backend.
* **Developer Friendly:** Uses Dependency Injection (Uber FX) for clean architecture and tenancy management. Code generation via `oapi-codegen`. Type-safe SQL with `go-jet`.

## 🏛️ Core Concepts

This service revolves around a clear hierarchy and distinct user types:

1.  **Platform Tenant:** Your top-level instance.
2.  **Product:** Your distinct applications/services managed by the Platform Tenant.
3.  **Organization:** Your Product's customers or tenants.
4.  **Workspace:** Sub-units within an Organization (teams, projects, etc.).

And the users:

* `PlatformUser`: Admins managing the platform (Bearer Token Auth).
* `ProductUser`: Your Product's end-users (API Key Auth for management, **Directory Only** - bring your own authentication via external IdP).


## 🏗️ Architecture

* **Tech:** Go | Uber FX | PostgreSQL | go-jet | KSUIDs | OpenAPI 3.0 | oapi-codegen
* **API:** Versioned (`/v1`), RESTful endpoints.
* **Auth:** Bearer Tokens (Platform Admins) & `X-Product-API-Key` (Product Backends).
* **Tenancy:** Architecture supports multi-tenant platforms, while the current self-hosted shape uses Dependency Injection to run in single-tenant mode.


## 🚀 Getting Started

Follow [local setup](docs/development/setup.md) for a standalone clone and [testing](docs/development/testing.md) for the managed disposable app and validation commands. The runtime creates its own local accounts and needs no shared workspace or external identity-provider credentials.

## 📖 Documentation

- [Agent guide](AGENTS.md), [domain vocabulary](CONTEXT.md) and [documentation index](docs/README.md).
- [Architecture](docs/technical/architecture.md), [dependency ownership](docs/technical/dependencies.md) and [release runbooks](docs/runbooks/deployment.md).
- [OpenAPI contract](apps/anchor/cmd/http/openapi.yaml), [Go SDK](clients/go/) and [frontend guide](anchor-ui/AGENTS.md).

## 🌍 Licensing & Community

* **License:** Functional Source License 1.1 with Apache 2.0 future license (`FSL-1.1-ALv2`)
* **Usage:** Free for permitted uses under the FSL, including internal use, non-commercial education and research, and professional services delivered to a valid licensee.
* **Commercial restriction:** You may not offer Anchor itself, or a substantially similar competing service, as a commercial product under the FSL terms.
* **Future license:** Each version converts to Apache 2.0 two years after it is made available.
* **Current scope:** Single Platform Tenant mode. Broader multi-tenant platform support may be offered separately.
* **Contribute:** We welcome contributions\! Use the [agent delivery workflow](docs/development/agent-workflow.md) and GitHub Issues/Pull Requests.

Current behavior and unresolved proposals are documented in the [documentation index](docs/README.md); verify proposal status against current source before planning a change.
