# Default workspaces

An organization can request a default workspace when it is created:

```json
{"name":"Acme","create_default_workspace":true}
```

`POST /v1/products/{product_id}/organizations` creates the organization, its
optional founding membership/license, the default workspace and their events in
one transaction. A failure rolls back these writes together. The existing
`organization:create` permission authorizes this composite creation. Omitting
the option or setting it to false preserves the existing creation behavior.
Organization updates ignore this creation-only option.

The Go SDK exposes the option as
`client.Organizations().Create("Acme").DefaultWorkspace().Do(ctx)`.

A founding-member retry returns the existing organization. When the retry opts
in, it also ensures that organization's default workspace exists. Repeating the
request reuses the same workspace and does not emit another creation event.

Workspace responses carry the read-only `is_default` field. The designation is
stored independently of the name. A default initially uses the name `Default`;
an existing workspace with that exact name is designated instead of creating
another. Renaming the designated workspace preserves its identity and designation.
Default designation uses a separate write, so concurrent ordinary edits preserve
the designation and provisioning preserves name/description changes.
The service serializes provisioning on the organization's row, and a partial
unique index permits at most one designated default per organization.

## Linked organization reads

Use the existing organization read with `?include=workspace,license` to obtain
its workspace list and license together. `include=workspace` expands the
`workspaces` array, including each workspace's `is_default` designation. An
included organization without workspaces returns `[]`; omitting the include
omits the field. Organization searches support the same include and read each
related resource for the whole page in one statement.

The Go SDK accepts both include constants on `Organizations().Get`. To request a
designated default through ordinary workspace creation, use
`client.Organization(orgID).Workspaces().Create("Default").IsDefault(true).Do(ctx)`.

The existing workspace Create API accepts `is_default: true`. It atomically
returns the designated default if one exists, otherwise designates a workspace
with the requested name or creates it. Renamed defaults retain their identity.
This gives product startup syncs idempotent provisioning without a special
lookup or ensure endpoint. Ordinary workspace creation remains unchanged.

## Echopoint-owned startup sync

Anchor does not run a global organization sweep. Echopoint owns startup
reconciliation for its configured Anchor product: page through organizations,
get each with `include=workspace,license`, and provision a default through the
existing workspace Create API when one is missing. No other product is swept.

The organization creation option remains creation-only. An organization created
without it has no default until its owning product provisions one. Workspace
deletion retains its existing contract. This prerequisite does not assign
Echopoint resource ownership or enforce workspace filtering.

## Verification

The default-workspace CTs cover opted-in creation/events, false/omitted
compatibility, founding-member retries, rollback and stale-edit interleaving.
`organization_workspace_include_test.go` covers absent versus included-empty
workspace lists, combined workspace/license includes, product isolation and
idempotent provisioning through ordinary workspace Create. Run from
`apps/anchor`:

```sh
go test ./cmd/it/ct/... -run 'Test(Organization.*DefaultWorkspace|DefaultWorkspace|OrganizationReadIncludes|OrganizationWorkspaceInclude)' -count=1
```
