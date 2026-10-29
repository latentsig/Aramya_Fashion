CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE categories (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    subtitle text NOT NULL DEFAULT '',
    image_key text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE TABLE products (
    id uuid PRIMARY KEY,
    sku text NOT NULL UNIQUE,
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    description text,
    category_id uuid NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    price_paise bigint NOT NULL CHECK (price_paise >= 0),
    currency char(3) NOT NULL DEFAULT 'INR' CHECK (currency ~ '^[A-Z]{3}$'),
    image_key text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('draft', 'active', 'archived')),
    inventory_policy text NOT NULL DEFAULT 'made_to_order' CHECK (inventory_policy IN ('tracked', 'made_to_order', 'untracked')),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT products_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE INDEX products_category_status_idx ON products(category_id, status, name);
CREATE INDEX products_name_search_idx ON products USING gin (to_tsvector('simple', name || ' ' || coalesce(description, '')));

CREATE TABLE product_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    object_key text NOT NULL,
    alt_text text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT product_images_product_key_unique UNIQUE(product_id, object_key)
);

CREATE UNIQUE INDEX product_images_one_primary_idx ON product_images(product_id) WHERE is_primary;
CREATE INDEX product_images_product_order_idx ON product_images(product_id, sort_order);

CREATE TABLE inventory (
    product_id uuid PRIMARY KEY REFERENCES products(id) ON DELETE RESTRICT,
    on_hand integer NOT NULL CHECK (on_hand >= 0),
    reserved integer NOT NULL DEFAULT 0 CHECK (reserved >= 0 AND reserved <= on_hand),
    updated_at timestamptz NOT NULL DEFAULT now()
);
