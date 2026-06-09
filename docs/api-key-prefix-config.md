# Organization API Key Prefix Configuration

Anchor stores product-scoped organization API key generation settings in a dedicated `product_organization_api_key_configs` table. Product create, update, read, and search responses expose the setting as a typed product config:

```json
{
  "config": {
    "organization_api_keys": {
      "prefix": "anchor"
    }
  }
}
```

The configured value is a root prefix. Generated organization API keys keep the existing organization key marker:

```text
{prefix}_org_apikey_<random>_<checksum>
```

For example, `acme` produces `acme_org_apikey_...`.

Product API keys are Anchor management credentials for a product and always use the fixed `anchor_prd_apikey_` prefix.

Existing organization keys remain valid after changing a product's organization API key prefix because validation hashes the submitted full key and looks it up directly. The prefix only affects newly generated organization API keys.
