# Gate flows (exported)

These files are the production gate flows of the Anchor workspace, exported
read-only from the prod Echopoint CI organisation on 2026-11-08 (workspace
`ws_3LXA9s55TGISuDA9yp3ZUfKCodG`, 36 flows). The post-deploy gate runs them
against dev after every Anchor deploy and holds prod until they pass.

Each file keeps only `id`, `name`, `tags`, `description`, `updated_at` and
`flow_definition`. Template references (`{{...}}`) stay as exported; no
secret values are in the export (`"secret": true` marks an output, not a
value).

- `*.json`: the 32 flows tagged `anchor`, replayed by `TestGateFlows`.
- `modules/`: the two flows the gate flows call as modules
  (`anchor: login (reusable)`, `anchor: setup-org (module)`).

Excluded (not gate flows):

| Flow | Id | Tags | Why |
|---|---|---|---|
| anchor: workspace | `a1c500f4-1802-4224-b7d2-2ac02ae8c985` | `anchor-retired` | Failing; the workspace routes were dropped in migration 000073. |
| anchor: known bugs | `c1e16869-5221-4c70-bf9b-c12a20c2f310` | `known-bugs` | Failing by design (open bugs), not part of the gate. |

## Refresh

Re-export after the gate flows change, then regenerate the goldens and review
the diff:

```sh
ws=ws_3LXA9s55TGISuDA9yp3ZUfKCodG
echopoint --profile prod --workspace "$ws" flows list -o json > list.json
# for each id tagged anchor (and the two modules):
echopoint --profile prod --workspace "$ws" flows get <id> -o json > <id>.json
```

Trim each file to the keys above and name it after the flow name
(`anchor: product user external id` -> `product-user-external-id.json`).

Then, from `apps/anchor`, run `rm -rf cmd/it/contract/testdata/golden/gate` and
`go test -tags contract ./cmd/it/contract/ -update` so goldens of renamed or dropped nodes do not stay behind,
and review the diff: every changed golden is a contract change on the gate side.
