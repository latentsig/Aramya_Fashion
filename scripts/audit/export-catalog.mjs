import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const legacyScript = await readFile(path.join(root, 'legacy-storefront/script.js'), 'utf8');
const catalogSource = legacyScript.slice(0, legacyScript.indexOf('const money ='));
const sandbox = {};
vm.runInNewContext(`${catalogSource}\nglobalThis.catalog = { products, categories };`, sandbox);
const namespace = Buffer.from('bfc0f22a1f5342c6a5832122e1289b0f', 'hex');

function stableId(value) {
  const bytes = createHash('sha1').update(namespace).update(value).digest().subarray(0, 16);
  bytes[6] = (bytes[6] & 0x0f) | 0x50;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = bytes.toString('hex');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function slugify(value) {
  return value.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

const categoryByKey = new Map(sandbox.catalog.categories.map((category) => [category.id, category]));
const categoryCounts = new Map();
const categories = sandbox.catalog.categories.map((category) => ({
  id: stableId(`category:${category.id}`),
  slug: category.id,
  name: category.title,
  subtitle: category.subtitle,
  image_key: `image assests/${category.image}`,
  sort_order: sandbox.catalog.categories.indexOf(category) + 1,
}));
const products = sandbox.catalog.products.map((product) => {
  const category = categoryByKey.get(product.category);
  if (!category) throw new Error(`Unknown category for product: ${product.name}`);
  const sequence = (categoryCounts.get(product.category) ?? 0) + 1;
  categoryCounts.set(product.category, sequence);
  return {
    id: stableId(`product:${slugify(product.name)}`),
    sku: `${product.category.toUpperCase()}-${String(sequence).padStart(3, '0')}`,
    slug: slugify(product.name),
    name: product.name,
    description: null,
    category_slug: product.category,
    price_paise: Math.round(product.price * 100),
    currency: 'INR',
    image_key: `image assests/${product.img}`,
    status: 'active',
    inventory_policy: 'made_to_order',
  };
});

const expectedCounts = new Map([['sarees', 5], ['bridal', 5], ['occasion', 14], ['menswear', 10], ['kids', 5]]);
const actualCounts = new Map(categories.map(({ slug }) => [slug, products.filter(({ category_slug }) => category_slug === slug).length]));
if (products.length !== 39 || categories.length !== 5 || [...expectedCounts].some(([slug, count]) => actualCounts.get(slug) !== count)) {
  throw new Error(`Unexpected catalog shape: ${products.length} products in ${JSON.stringify(Object.fromEntries(actualCounts))}`);
}
const output = { source: 'legacy-storefront/script.js', currency: 'INR', categories, products };
await writeFile(path.join(root, 'database/seeds/catalog.json'), `${JSON.stringify(output, null, 2)}\n`);
console.log(`Exported ${products.length} products and ${categories.length} categories from legacy-storefront/script.js`);
