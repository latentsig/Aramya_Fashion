CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text,
    display_name text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    email_verified_at timestamptz,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_nonempty CHECK (length(trim(email)) > 3)
);
CREATE UNIQUE INDEX users_email_lower_unique_idx ON users(lower(email));

CREATE TABLE roles (
    id smallint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE CHECK (name IN ('customer', 'admin'))
);
INSERT INTO roles(name) VALUES ('customer'), ('admin');

CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id smallint NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id, role_id)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    rotated_from uuid REFERENCES sessions(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sessions_expiration_after_creation CHECK (expires_at > created_at)
);
CREATE INDEX sessions_user_expiration_idx ON sessions(user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE user_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('google')),
    provider_subject text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_subject),
    UNIQUE(user_id, provider)
);

CREATE TABLE password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE addresses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_name text NOT NULL,
    phone text NOT NULL,
    line1 text NOT NULL,
    line2 text NOT NULL DEFAULT '',
    city text NOT NULL,
    region text NOT NULL,
    postal_code text NOT NULL,
    country_code char(2) NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(trim(recipient_name)) > 0),
    CHECK (length(trim(line1)) > 0),
    CHECK (country_code ~ '^[A-Z]{2}$')
);
CREATE INDEX addresses_user_idx ON addresses(user_id, is_default DESC, created_at DESC);

CREATE TABLE product_variants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    sku text NOT NULL UNIQUE,
    option_name text NOT NULL,
    option_value text NOT NULL,
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    price_adjustment_paise bigint NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT true,
    is_default boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(product_id, option_name, option_value),
    CHECK (length(trim(option_name)) > 0),
    CHECK (length(trim(option_value)) > 0)
);
CREATE UNIQUE INDEX product_variants_default_unique_idx ON product_variants(product_id) WHERE is_default;
CREATE INDEX product_variants_product_active_idx ON product_variants(product_id, active);

-- Legacy catalog items have no recorded size inventory. Create a made-to-measure
-- purchasable variant for identity only; this does not create or imply stock.
INSERT INTO product_variants(product_id, sku, option_name, option_value, is_default)
SELECT id, sku || '-MTO', 'Fit', 'Made to measure', true FROM products
ON CONFLICT(sku) DO NOTHING;

-- Re-key any operator-entered legacy inventory against the default product variant,
-- preserving every existing quantity. Products without inventory remain unstocked.
ALTER TABLE inventory ADD COLUMN product_variant_id uuid REFERENCES product_variants(id) ON DELETE RESTRICT;
UPDATE inventory i SET product_variant_id = v.id
FROM product_variants v WHERE v.product_id = i.product_id AND v.is_default;
ALTER TABLE inventory ALTER COLUMN product_variant_id SET NOT NULL;
ALTER TABLE inventory DROP CONSTRAINT inventory_pkey;
ALTER TABLE inventory DROP CONSTRAINT inventory_product_id_fkey;
ALTER TABLE inventory DROP COLUMN product_id;
ALTER TABLE inventory ADD CONSTRAINT inventory_pkey PRIMARY KEY(product_variant_id);
CREATE TABLE inventory_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_variant_id uuid NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    movement_type text NOT NULL CHECK (movement_type IN ('opening', 'receipt', 'sale', 'reservation', 'release', 'adjustment', 'return')),
    quantity_delta integer NOT NULL CHECK (quantity_delta <> 0),
    quantity_after integer NOT NULL CHECK (quantity_after >= 0),
    reference_type text NOT NULL DEFAULT '',
    reference_id uuid,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inventory_movements_variant_created_idx ON inventory_movements(product_variant_id, created_at DESC);

CREATE TABLE carts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    guest_token_hash bytea UNIQUE CHECK (guest_token_hash IS NULL OR octet_length(guest_token_hash) = 32),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((user_id IS NOT NULL AND guest_token_hash IS NULL) OR (user_id IS NULL AND guest_token_hash IS NOT NULL))
);
CREATE UNIQUE INDEX carts_one_per_user_idx ON carts(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX carts_expiration_idx ON carts(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE cart_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id uuid NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    product_variant_id uuid NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 20),
    customization jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(customization) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(cart_id, product_variant_id, customization)
);
CREATE INDEX cart_items_cart_idx ON cart_items(cart_id);

CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number text NOT NULL UNIQUE,
    customer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    guest_email text NOT NULL DEFAULT '',
    guest_phone text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending_payment' CHECK (status IN ('pending_payment', 'paid', 'confirmed', 'processing', 'ready_to_ship', 'shipped', 'delivered', 'cancelled', 'refund_pending', 'refunded')),
    currency char(3) NOT NULL DEFAULT 'INR' CHECK (currency ~ '^[A-Z]{3}$'),
    subtotal_paise bigint NOT NULL CHECK (subtotal_paise >= 0),
    tax_paise bigint NOT NULL DEFAULT 0 CHECK (tax_paise >= 0),
    shipping_paise bigint NOT NULL DEFAULT 0 CHECK (shipping_paise >= 0),
    discount_paise bigint NOT NULL DEFAULT 0 CHECK (discount_paise >= 0),
    total_paise bigint NOT NULL CHECK (total_paise >= 0 AND total_paise = subtotal_paise + tax_paise + shipping_paise - discount_paise),
    shipping_address jsonb NOT NULL CHECK (jsonb_typeof(shipping_address) = 'object'),
    billing_address jsonb CHECK (billing_address IS NULL OR jsonb_typeof(billing_address) = 'object'),
    customer_note text NOT NULL DEFAULT '',
    idempotency_key text NOT NULL UNIQUE,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (customer_id IS NOT NULL OR length(trim(guest_email)) > 3)
);
CREATE INDEX orders_customer_created_idx ON orders(customer_id, created_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX orders_status_created_idx ON orders(status, created_at DESC);

CREATE TABLE order_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    product_variant_id uuid NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    product_name text NOT NULL,
    product_sku text NOT NULL,
    variant_description text NOT NULL,
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 20),
    unit_price_paise bigint NOT NULL CHECK (unit_price_paise >= 0),
    tax_rate_basis_points integer NOT NULL DEFAULT 0 CHECK (tax_rate_basis_points BETWEEN 0 AND 10000),
    tax_paise bigint NOT NULL DEFAULT 0 CHECK (tax_paise >= 0),
    line_total_paise bigint NOT NULL CHECK (line_total_paise = unit_price_paise * quantity + tax_paise),
    customization jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(customization) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_items_order_idx ON order_items(order_id);

CREATE TABLE order_status_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    from_status text,
    to_status text NOT NULL CHECK (to_status IN ('pending_payment', 'paid', 'confirmed', 'processing', 'ready_to_ship', 'shipped', 'delivered', 'cancelled', 'refund_pending', 'refunded')),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_status_events_order_idx ON order_status_events(order_id, created_at);

CREATE TABLE payment_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    provider text NOT NULL CHECK (provider = 'razorpay'),
    status text NOT NULL CHECK (status IN ('initiated', 'pending', 'authorized', 'captured', 'failed', 'cancelled', 'partially_refunded', 'refunded')),
    amount_paise bigint NOT NULL CHECK (amount_paise > 0),
    currency char(3) NOT NULL DEFAULT 'INR' CHECK (currency ~ '^[A-Z]{3}$'),
    provider_order_id text UNIQUE,
    provider_payment_id text UNIQUE,
    idempotency_key text NOT NULL UNIQUE,
    signature_verified_at timestamptz,
    failure_code text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_attempts_order_idx ON payment_attempts(order_id, created_at DESC);

CREATE TABLE payment_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL CHECK (provider = 'razorpay'),
    provider_event_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payment_attempt_id uuid REFERENCES payment_attempts(id) ON DELETE SET NULL,
    payload_sha256 bytea NOT NULL CHECK (octet_length(payload_sha256) = 32),
    processed_at timestamptz,
    processing_error_code text NOT NULL DEFAULT '',
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_events_unprocessed_idx ON payment_events(received_at) WHERE processed_at IS NULL;

CREATE TABLE inventory_reservations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id uuid NOT NULL UNIQUE REFERENCES order_items(id) ON DELETE RESTRICT,
    product_variant_id uuid NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity integer NOT NULL CHECK (quantity > 0),
    status text NOT NULL CHECK (status IN ('active', 'consumed', 'released', 'expired')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);
CREATE INDEX inventory_reservations_expiration_idx ON inventory_reservations(expires_at) WHERE status = 'active';

CREATE TABLE shipments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    provider text NOT NULL DEFAULT '',
    tracking_number text NOT NULL DEFAULT '',
    tracking_url text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'unfulfilled' CHECK (status IN ('unfulfilled', 'ready', 'shipped', 'delivered', 'returned')),
    shipped_at timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refunds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_attempt_id uuid NOT NULL REFERENCES payment_attempts(id) ON DELETE RESTRICT,
    provider_refund_id text UNIQUE,
    amount_paise bigint NOT NULL CHECK (amount_paise > 0),
    status text NOT NULL CHECK (status IN ('pending', 'processed', 'failed')),
    idempotency_key text NOT NULL UNIQUE,
    reason text NOT NULL DEFAULT '',
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE appointments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    name text NOT NULL,
    email text NOT NULL DEFAULT '',
    phone text NOT NULL,
    preferred_at timestamptz,
    interest text NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'confirmed', 'completed', 'cancelled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX appointments_status_created_idx ON appointments(status, created_at DESC);

CREATE TABLE custom_order_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    name text NOT NULL,
    email text NOT NULL DEFAULT '',
    phone text NOT NULL,
    garment_type text NOT NULL,
    occasion text NOT NULL DEFAULT '',
    requirements text NOT NULL DEFAULT '',
    measurements jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(measurements) = 'object'),
    quotation_paise bigint CHECK (quotation_paise IS NULL OR quotation_paise >= 0),
    currency char(3) NOT NULL DEFAULT 'INR' CHECK (currency ~ '^[A-Z]{3}$'),
    status text NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'reviewing', 'fitting', 'quoted', 'awaiting_approval', 'approved', 'in_progress', 'completed', 'declined', 'cancelled')),
    quote_approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX custom_order_requests_status_created_idx ON custom_order_requests(status, created_at DESC);

CREATE TABLE media_objects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    object_key text NOT NULL UNIQUE,
    original_filename text NOT NULL,
    mime_type text NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size > 0),
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    alt_text text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'private', 'deleting')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(sha256, object_key)
);
CREATE INDEX media_objects_checksum_idx ON media_objects(sha256);

CREATE TABLE idempotency_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope text NOT NULL,
    idempotency_key text NOT NULL,
    request_sha256 bytea NOT NULL CHECK (octet_length(request_sha256) = 32),
    response_status integer,
    response_body jsonb CHECK (response_body IS NULL OR jsonb_typeof(response_body) = 'object'),
    completed_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(scope, idempotency_key)
);
CREATE INDEX idempotency_records_expiration_idx ON idempotency_records(expires_at);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid,
    request_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_entity_idx ON audit_logs(entity_type, entity_id, created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs(actor_user_id, created_at DESC) WHERE actor_user_id IS NOT NULL;
