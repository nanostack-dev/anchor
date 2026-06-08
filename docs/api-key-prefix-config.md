# Product API Key Prefix Configuration

Anchor stores product API key generation settings in a dedicated `product_api_key_configs` table. Product create, update, read, and search responses expose the setting as a typed product config:

```json
{
  "config": {
    "api_keys": {
      "prefix": "anchor"
    }
  }
}
```

The configured value is a root prefix. Generated API keys keep the existing key-kind marker:

```text
{prefix}_prd_apikey_<random>_<checksum>
{prefix}_org_apikey_<random>_<checksum>
```

For example, `acme` produces `acme_prd_apikey_...` and `acme_org_apikey_...`.

Existing keys remain valid after changing a product prefix because validation hashes the submitted full key and looks it up directly. The prefix only affects newly generated product and organization API keys.
