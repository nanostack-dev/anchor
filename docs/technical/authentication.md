# Platform user authentication and tenant resolution

Platform users authenticate to the Anchor admin API via `POST /v1/auth/register` and `POST /v1/auth/login` (`apps/anchor/internal/service/auth_service.go`).

## Tenant resolution

- **Registration (`POST /v1/auth/register`)**: validates the invitation (or creates the initial tenant), creates the `users` and `platform_users` rows in one transaction, and immediately starts a session for that created `platform.User` via `AuthService.StartSession` without re-running `Login`.
- **Password login (`POST /v1/auth/login`)**: verifies the password against `users` and looks up the user's memberships via `PlatformTenantUserRepository.FindAllByUserID(ctx, user.ID)`. When the user belongs to exactly one platform tenant, login issues a session scoped to that tenant.

## Known limitation: multi-tenant users

- Password login carries no tenant selector. If a `users` row has memberships in more than one platform tenant, `Login` deliberately fails closed with `401 EMAIL_OR_PASSWORD_INCORRECT` (matching the invalid-credentials response so account state is not leaked) rather than silently picking an arbitrary tenant.
- Today, `Register` rejects any email that already exists in `users`, so multi-tenant memberships cannot be created through the API. Supporting users in multiple platform tenants requires a follow-up tenant-selection flow on login.
