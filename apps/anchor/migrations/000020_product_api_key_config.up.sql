CREATE TABLE product_api_key_configs (
    product_id VARCHAR(255) PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    prefix VARCHAR(32) NOT NULL DEFAULT 'anchor',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO product_api_key_configs (product_id, prefix)
SELECT id, 'anchor'
FROM products;

CREATE TRIGGER update_product_api_key_configs_updated_at
BEFORE UPDATE ON product_api_key_configs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
