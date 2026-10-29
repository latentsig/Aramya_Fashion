DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM inventory i
        JOIN product_variants v ON v.id = i.product_variant_id
        GROUP BY v.product_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot roll back variant inventory: multiple stocked variants exist for one product';
    END IF;
END $$;

DROP TABLE audit_logs;
DROP TABLE idempotency_records;
DROP TABLE media_objects;
DROP TABLE custom_order_requests;
DROP TABLE appointments;
DROP TABLE refunds;
DROP TABLE shipments;
DROP TABLE inventory_reservations;
DROP TABLE payment_events;
DROP TABLE payment_attempts;
DROP TABLE order_status_events;
DROP TABLE order_items;
DROP TABLE orders;
DROP TABLE cart_items;
DROP TABLE carts;
DROP TABLE inventory_movements;

ALTER TABLE inventory ADD COLUMN product_id uuid;
UPDATE inventory i SET product_id = v.product_id FROM product_variants v WHERE v.id = i.product_variant_id;
ALTER TABLE inventory ALTER COLUMN product_id SET NOT NULL;
ALTER TABLE inventory DROP CONSTRAINT inventory_pkey;
ALTER TABLE inventory DROP CONSTRAINT inventory_product_variant_id_fkey;
ALTER TABLE inventory DROP COLUMN product_variant_id;
ALTER TABLE inventory ADD CONSTRAINT inventory_pkey PRIMARY KEY(product_id);
ALTER TABLE inventory ADD CONSTRAINT inventory_product_id_fkey FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE RESTRICT;

DROP TABLE product_variants;
DROP TABLE addresses;
DROP TABLE password_reset_tokens;
DROP TABLE user_identities;
DROP TABLE sessions;
DROP TABLE user_roles;
DROP TABLE roles;
DROP TABLE users;
