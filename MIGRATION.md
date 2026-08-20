# Repository reads return functional.Option

Optional reads return `functional.Option[T]`, not `*T`. The repository stops
translating absence into a nil pointer. The service layer asks `Err`, then
`IsPresent`, then `Value`.

## Rules

- **In scope**: a read where finding nothing is a legitimate outcome —
  `FindBy*`, `GetBy*`, and their unexported forms. Signature becomes
  `functional.Option[T]` with no `error` result. The error rides inside.
- **Out of scope**: `Create`, `Update`, and any write. A create that finds
  nothing is a bug, not an absence. Leave them. Note any you think are wrong
  under "Deferred" rather than changing them.
- **Out of scope**: `List*` and `Search*`. An empty slice already says "none".
- A method that bypasses tenant scope keeps its `*Internal` name.
- No linter disabled, no `//nolint`. Ask before any suppression.

## Status

Mark each file `[x]` only when its methods are migrated, every caller is
updated, and `go build ./...` passes. Add a one-line note for anything odd.

### internal/repository
- [x] integration_event_repository.go — FindByIDInternal, FindByExternalEventIDInternal (list also missed FindByIDInternal, migrated it too; both callers in integration_service.go updated)
- [x] integration_instance_repository.go — FindByID, FindByIDInternal, FindByProductAndProvider, FindByProductAndProviderInternal (UpdateOptional already returned Option pre-migration; left as-is, it's a write). Callers updated in integration_service.go (7 sites) and internal/email/service/service.go
- [x] organization_api_key_repository.go — GetByID, GetByIDInternal, GetByOrganizationIDAndName, GetByOrganizationIDAndHashedValue, GetByProductIDAndHashedValueInternal, findOne (list also missed GetByID; migrated it too). Update() in organization_api_key_service.go kept two separate locals (existingAPIKey original vs updatedAPIKey mutated copy) since a comparison against the pre-mutation state relies on both. Fixed 2 `cmd/it` test files that called the old (*T, error) shape.
- [x] organization_membership_repository.go — the list's FindByProductUserID is WRONG scope: it returns a slice ([]user.OrganizationMembership), so an empty slice already means "none" — left untouched, out of scope by the rule's own substance. Migrated the two real singular finders instead: FindByProductUserIDAndOrgID, FindByOrgIDAndUserID (plus internal callers Create/Update in the same file). Callers fixed in organization_membership_service.go (4 sites), organization_service.go, product_user_service.go.
- [x] organization_repository.go — FindByID. Many callers across internal/service (organization_service.go x4, organization_api_key_service.go x2, workspace_service.go) and internal/license/service (license_history_service.go, license_migration_service.go, usage_series_service.go). Go only reports errors package-by-package once its imports build, so these surfaced in two waves — fixed all.
- [x] platform_invitation_repository.go — FindByTenantIDAndEmail, FindByCodeAndEmail (list also missed FindByCodeAndEmail; migrated it too). Callers fixed in auth_service.go, invitation_service.go.
- [x] platform_tenant_user_repository.go — FindByTenantIDAndUserID, FindByTenantIDAndID, FindByTenantIDAndEmail (list also missed FindByTenantIDAndUserID; migrated it too). Callers fixed in auth_service.go (3 sites), invitation_service.go, platform_user_service.go (2 sites).
- [x] user_repository.go — FindByEmail (NOT on the original list at all). Callers fixed in auth_service.go Register and Login (2 sites).
- [x] product_api_key_repository.go — GetByID, GetByProductIDAndName, GetByProductIDAndHashedValue, findOne (list also missed GetByID and GetByProductIDAndHashedValue; migrated both too). Callers fixed in product_api_key_service.go (GetByID x3, GetByProductIDAndHashedValue wrapped in cache GetOrElse closure, GetByProductIDAndName). validateAPIKey's cache callback still returns (*T, error) to match apiKeys.Key(...).GetOrElse's own signature — converted Option->(*T,error) right at that boundary since GetOrElse's contract is outside this migration's scope.
- [x] product_permission_repository.go — FindByProductIDAndPermissionName (FindByProductIDAndPermissionNames returns a slice, left out of scope). Callers fixed in permission_service.go (4 sites) and cmd/it/repository/product_permission_repository_test.go.
- [x] product_repository.go — FindByID, FindByIDInternal, FindByTenantIDAndName. Callers fixed in organization_api_key_service.go, product_service.go (Get, GetInternal, GetWithCache's cache closure, Create, findProductForUpdate, validateNameUniqueness).
- [x] product_role_repository.go — FindByProductIDAndRoleID, GetByProductIDAndName, findOne. Callers fixed in organization_membership_service.go, organization_service.go, product_role_service.go (6 sites).
- [x] product_resource_permission_repository.go — FindByName (NOT on the original list at all). Callers fixed in product_role_service.go and resource_permission_service.go (Create, GetByID, Update, Delete).
- [x] product_user_repository.go — FindByProductIDAndID, FindByExternalID (list's FindByProductID returns a slice, wrong scope, left alone; list missed FindByExternalID, migrated it too). Callers fixed in organization_membership_service.go, organization_service.go, product_user_service.go (4 sites), cmd/it/shared/dsl/product_links_builder.go.
- [x] tenant_repository.go — FindByID (no current callers; interface + impl updated, build/vet clean).
- [x] workspace_repository.go — FindByID, FindByOrganizationIDAndName. The repo's own Update/DeleteByID (writes, out of scope) call FindByID internally — fixed those call sites too. Callers fixed in workspace_service.go (Find, Create, Update x2, Delete, ensureNameAvailable).

### internal/email/repository
- [x] send_record_repository.go — FindByDedupeKeyInternal, FindByID, FindByDedupeKey (list missed FindByDedupeKey; migrated it too). Interface lives in internal/email/repository/repository.go, updated there. Callers fixed in internal/email/service/service.go (Send, createSendRecord).
- [x] template_repository.go — FindByID, FindBySlug, FindBySlugInternal. Interface in repository.go, updated there.
- [x] template_version_repository.go — FindByID, findByStatus (covers FindCurrentDraft/FindCurrentPublished). Interface in repository.go, updated there. Both files' callers live together in internal/email/service/service.go (resolveTemplate, resolveSendVersion, CreateTemplate, UpdateTemplate, UpdateTemplateDraft, GetTemplateDraft, PublishTemplate, GetTemplate, DeleteTemplate, SaveTemplateExamples, Preview) — fixed all in one pass since they interleave in the same functions.

### internal/license/repository
- [x] organization_license_repository.go — findByOrganization, FindByOrganization, FindByOrganizationForUpdate. FindByOrganizations (returns a map — absence per key is already a missing key, out of scope) left untouched. Interface in repository.go, updated there. Callers fixed in license_migration_service.go and organization_license_service.go (GetLicense's cache closure, AdjustValues, DiffAgainstTemplate).
- [x] schema_repository.go — FindByProduct. Interface in repository.go, updated there. Callers fixed in schema_service.go (CreateSchema, GetSchema, UpdateSchema, DeleteSchema).
- [x] template_repository.go — FindByID, FindByName. Interface in repository.go, updated there. Callers fixed in template_service.go (CreateTemplate, GetTemplate, UpdateTemplate x2, ArchiveTemplate, DeleteTemplate).

## Deferred
_Anything deliberately not migrated, with the reason._

None. Every `Create`/`Update`/write method reviewed during this pass looked correct as a write (return value is the row just written, no legitimate-absence case). `IntegrationInstanceRepository.UpdateOptional` already returned `functional.Option[T]` before this migration started (a deliberate exception documented on the method: an UPDATE...RETURNING racing a concurrent delete treats a zero-row result as absence, not an error) — it is a write, so it stayed out of scope, but noted here since its shape could be mistaken for an in-scope read.

## Interfaces changed
_Every interface whose method signature moved, so reviewers can find the seams._

All declared in `internal/repository/*.go` (interface lives with its impl) unless noted:

- `IntegrationEventRepository` — FindByIDInternal, FindByExternalEventIDInternal
- `IntegrationInstanceRepository` — FindByID, FindByIDInternal, FindByProductAndProvider, FindByProductAndProviderInternal
- `OrganizationAPIKeyRepository` — GetByID, GetByIDInternal, GetByOrganizationIDAndName, GetByOrganizationIDAndHashedValue, GetByProductIDAndHashedValueInternal
- `OrganizationMembershipRepository` — FindByProductUserIDAndOrgID, FindByOrgIDAndUserID
- `OrganizationRepository` — FindByID
- `InvitationRepository` — FindByTenantIDAndEmail, FindByCodeAndEmail
- `PlatformTenantUserRepository` — FindByTenantIDAndUserID, FindByTenantIDAndID, FindByTenantIDAndEmail
- `UserRepository` — FindByEmail
- `ProductAPIKeyRepository` — GetByID, GetByProductIDAndName, GetByProductIDAndHashedValue
- `ProductPermissionRepository` — FindByProductIDAndPermissionName
- `ProductRepository` — FindByID, FindByIDInternal, FindByTenantIDAndName
- `ProductRoleRepository` — FindByProductIDAndRoleID, GetByProductIDAndName
- `ProductResourcePermissionRepository` — FindByName
- `ProductUserRepository` — FindByProductIDAndID, FindByExternalID
- `TenantRepository` — FindByID
- `WorkspaceRepository` — FindByID, FindByOrganizationIDAndName
- `SendRecordRepository` (internal/email/repository/repository.go) — FindByDedupeKeyInternal, FindByDedupeKey, FindByID
- `TemplateRepository` (internal/email/repository/repository.go) — FindByID, FindBySlug, FindBySlugInternal
- `TemplateVersionRepository` (internal/email/repository/repository.go) — FindByID, FindCurrentDraft, FindCurrentPublished
- `OrganizationLicenseRepository` (internal/license/repository/repository.go) — FindByOrganization, FindByOrganizationForUpdate
- `SchemaRepository` (internal/license/repository/repository.go) — FindByProduct
- `TemplateRepository` (internal/license/repository/repository.go) — FindByID, FindByName

Unexported helpers whose shape also moved (not part of any interface, but shared across a repo's own methods): `findOne` in organization_api_key_repository.go, product_api_key_repository.go, and product_role_repository.go; `findByOrganization` in organization_license_repository.go; `findByStatus` in template_version_repository.go (email).
