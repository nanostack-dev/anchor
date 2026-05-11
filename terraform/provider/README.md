# Terraform Provider: Anchor

Terraform provider for managing:
- `anchor_product`
- `anchor_product_role`
- `anchor_product_permission` (product resource permissions, e.g. `flows:create`)

## Authentication

Provider supports either:
- `token` (`Authorization: Bearer ...`)
- `api_key` (`X-Product-API-Key: ...`)

Environment variable alternatives:
- `ANCHOR_BASE_URL`
- `ANCHOR_TOKEN`
- `ANCHOR_API_KEY`

## Build

```bash
go build ./...
```

## GitHub release distribution

This provider is published from the private `anchor` GitHub repository as
release assets attached to tags in the form:

```text
terraform/provider/vX.Y.Z
```

Each release publishes zipped provider binaries and a matching SHA256 checksum file.

This is intended for controlled internal/private installation flows where the
consumer has GitHub access to the repository but the provider is not published
to the public Terraform Registry.

## Local Terraform usage

Use a Terraform CLI config development override to point `nanostack-dev/anchor`
to this local provider binary.

Example `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "nanostack-dev/anchor" = "/ABSOLUTE/PATH/TO/anchor/terraform/provider"
  }

  direct {}
}
```

Then in Terraform configuration:

```hcl
terraform {
  required_providers {
    anchor = {
      source  = "nanostack-dev/anchor"
      version = "0.1.0"
    }
  }
}
```
