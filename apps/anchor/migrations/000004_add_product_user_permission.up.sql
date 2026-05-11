DO $$
DECLARE
rec RECORD;
BEGIN
FOR rec IN SELECT id FROM products LOOP
    INSERT INTO product_permissions (product_id, name, description)
           VALUES (rec.id, 'product_user:read', 'Allow reading of the product user resource')
           ON CONFLICT (product_id, name) DO NOTHING;
INSERT INTO product_permissions (product_id, name, description)
VALUES (rec.id, 'product_user:update', 'Allow updating the product user resource')
    ON CONFLICT (product_id, name) DO NOTHING;
INSERT INTO product_permissions (product_id, name, description)
VALUES (rec.id, 'product_user:create', 'Allow creating a new product user resource')
    ON CONFLICT (product_id, name) DO NOTHING;
INSERT INTO product_permissions (product_id, name, description)
VALUES (rec.id, 'product_user:delete', 'Allow deleting the product user resource')
    ON CONFLICT (product_id, name) DO NOTHING;
END LOOP;
END $$;
