# SECURITY.md — Anchor Core

## Scope
Focused review on **multi‑tenant authorization** and access to unauthorized resources.

## Last Review
- **Reviewed commit:** `69a1ddf1cb7d75603bbf3d2c19099bb3a0fd72ba`
- **Date (UTC):** 2026-02-02

## Findings (High Priority)
1) **Login selects first tenant**
   - **Risk:** If multi‑tenant mode is enabled, login chooses `tenants[0]` and searches user there, which can mint tokens for the wrong tenant.
   - **Ref:** `apps/anchor/internal/service/auth/auth_service.go` (login flow).
   - **Status:** **Open**

## Findings (OK)
- Product‑scoped endpoints enforce tenant ownership via token tenant ID.
- API key auth is scoped to `product_id` and validated against hashed key + scopes.

## Required Actions
- Require explicit tenant selection during login (e.g., tenant_id in request or subdomain) and validate user membership.
- Re‑review after fix and update this file.

## Notes
- This review focused on tenant isolation. Expand to org/workspace boundaries as needed.
