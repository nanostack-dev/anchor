# Nanostack design-system adoption

Tracked by [Anchor #174](https://github.com/nanostack-dev/anchor/issues/174). This supersedes the earlier standalone shadcn migration.

## Investigation baseline

Anchor `f193b8a`, Echopoint current `origin/main`, and design-system `0.2.2` were inspected. The published package version and tarball integrity were verified on npm. Echopoint pins the same version.

The inventory found 38 route modules, 39 ordinary page files, 29 story files, 51 local primitive source files, 31 primitives with external callers, two unit-test files, and one local-backend bulk-delete e2e test. The old app duplicated shared tokens and fonts and allowed custom classes on generic primitives.

## Decisions and coverage

| Area | Shared parts | Anchor retains |
| --- | --- | --- |
| Theme | Canonical stylesheet, Tailwind source registration, Fontsource families | Light-only product policy |
| Shell | AppShell, sidebar/navigation, menus, provider | Auth, product selection, route map, user actions |
| Page/feedback | PageHeader, breadcrumbs, alerts, badges, empty/skeleton/spinner/toast | Domain status inference and form/API-error adaptation |
| Tables | Table/checkbox/menu/select/input and layout | v8 controlled server sorting/pagination/filtering, visibility, all-matching and page selection, bulk operations/retry, row navigation |
| Forms and dialogs | Field/input/textarea/select/switch, Dialog and AlertDialog | Generated Zod validation, secret lifecycle, CRUD, permissions |
| Role wizard | Tabs, Progress and layout | Validation, step navigation and permission-tree state |
| Licensing | Shared forms, tables, navigation, chart wrappers | Schema DSL, template/migration/value/history/usage behavior |
| Email | Shared controls and overlays | Monaco and template domain |
| Integrations/settings | Shared controls and surfaces | Clerk/SMTP configuration, auth and mutation state |

The generic DataTable block uses TanStack Table v9 and lacks Anchor's full controlled behavior. Replacing the adapter would lose functionality; retaining its state layer over shared Table primitives is the deliberate boundary, consistent with Echopoint. The old vertical stepper has one live role-wizard consumer; existing Tabs/Progress/layout express that workflow, so a new shared component is not required. Status badges, alerts and routed license navigation also compose existing public APIs. New common gaps should be raised with two concrete consumer uses, not copied locally.

## Shared extension

`CopyButton` and `CopyIconButton` in design-system 0.2.3 replace repeated clipboard state in Clerk identifiers, webhook URLs and email HTML. They await the clipboard result, announce errors as well as success, preserve focus and suppress duplicate pending writes. The same control applies to Echopoint endpoint URLs. The package also fixes light-theme text contrast on muted/selected surfaces, exposed by Anchor's accessibility run.

Anchor pins published `0.2.4`. [Design-system #46](https://github.com/nanostack-dev/nanostack-design-system/pull/46) preserves native disabled navigation with tooltips and routes Select names to the opened listbox. Every Anchor option list has an explicit accessible name, including tested open-popup states. Both fixes live in the shared library and use its existing closed props.

## Regression ledger

- Authentication/register/init, sidebar collapse/mobile menu, product switch, user navigation and query/hash/external links.
- Table filter/search/sort/visibility/page size, checkbox and selection menu, all-matching callbacks, disabled controls, clickable rows, loading/empty/load-error/stale/retry.
- Page-only bulk selection, sequential mutations, partial failure/retry, eligibility, selection-scope resets and empty-last-page recovery.
- Product create/edit/delete, API-key secrets and generated validation, role wizard and permission expansion/selection.
- Schema text/visual drafts, field rules/types/errors, templates, migration outcomes, adjusted values, usage ranges and history.
- Email builder, SMTP and Clerk settings, read-only/detail routes, breadcrumbs and mobile overflow.

Run and record actual verification in the issue/PR; a checklist here is not test evidence. The generated OpenAPI client and backend contract are unchanged.
