DO $$
DECLARE
 rec RECORD;
 BEGIN
 FOR rec IN SELECT id FROM products LOOP
     DELETE FROM product_permissions
     WHERE product_id = rec.id
       AND name IN (
         'product_user:read',
         'product_user:update',
         'product_user:create',
         'product_user:delete'
       );
 END LOOP;
 END $$;

