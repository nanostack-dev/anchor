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
The service serializes provisioning on the organization's row, and a partial
unique index permits at most one designated default per organization.

## Startup sweep

After migrations, Anchor's startup hook visits every organization that lacks a
designated default, across all products, using keyset pages of 200. It calls the
same provisioning operation used by organization creation. Each organization
commits independently, including its workspace event. Multiple starting replicas
serialize per organization and converge on the same workspace.

The hook completes before HTTP startup. It honors the startup context. A failure
prevents startup; the next start resumes with organizations that still lack a
default, preserving the work already committed. An organization deleted while
the sweep runs no longer needs provisioning and is skipped. No detached task,
external API call or SQL business trigger creates these workspaces.

The option controls creation time: an organization created after the sweep with
the option false/absent has no default until a later startup or an opted-in
founding-member retry. Deleting a designated workspace follows the existing
workspace deletion contract; a later ensure/startup can create its replacement.
This prerequisite does not assign Echopoint resources to workspaces or enforce
workspace filtering. Those changes belong to later consumer PRs.

## Verification

`apps/anchor/cmd/it/ct/default_workspace_test.go` covers opted-in creation and
signed organization/workspace events, false/omitted compatibility, founding-member
retries, multi-product startup pagination, concurrent sweeps and rename/restart
identity. Run the owning CTs from `apps/anchor` with:

```sh
go test ./cmd/it/ct/... -run 'Test(Organization.*DefaultWorkspace|DefaultWorkspaceStartup)' -count=1
```

The startup test stays serial because its reconciliation pass visits all products.
The ordinary request tests use isolated product fixtures and run in parallel.
