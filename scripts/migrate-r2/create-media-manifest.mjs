#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { mkdir, readFile, readdir, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const outputPath = path.join(repositoryRoot, 'docs/asset-migration-manifest.json');
const acceptedExtensions = new Set(['.avif', '.gif', '.jpeg', '.jpg', '.png', '.svg', '.webm', '.webp', '.mp4']);
const mimeTypes = new Map([
  ['.avif', 'image/avif'], ['.gif', 'image/gif'], ['.jpeg', 'image/jpeg'], ['.jpg', 'image/jpeg'],
  ['.png', 'image/png'], ['.svg', 'image/svg+xml'], ['.webm', 'video/webm'], ['.webp', 'image/webp'], ['.mp4', 'video/mp4'],
]);
const sourceRoots = [
  { path: 'image assests', role: 'original' },
  { path: 'legacy-storefront/dist/image assests', role: 'legacy-dist-copy' },
  { path: 'frontend/public/pwa', role: 'pwa-brand' },
  { path: 'mobile/assets/images', role: 'native-brand' },
];

async function filesBelow(absoluteRoot, relativeRoot) {
  let entries;
  try {
    entries = await readdir(absoluteRoot, { withFileTypes: true });
  } catch (error) {
    if (error.code === 'ENOENT') return [];
    throw error;
  }
  const files = [];
  for (const entry of entries) {
    const absolute = path.join(absoluteRoot, entry.name);
    const relative = path.posix.join(relativeRoot, entry.name);
    if (entry.isDirectory()) {
      files.push(...await filesBelow(absolute, relative));
    } else if (entry.isFile()) {
      const extension = path.extname(entry.name).toLowerCase();
      const isNativeBrandIcon = relative.startsWith('mobile/assets/images/') && /^aramya-(?:192|512)\.png$/i.test(entry.name);
      if (acceptedExtensions.has(extension) && (!relative.startsWith('mobile/assets/images/') || isNativeBrandIcon)) {
        files.push({ absolute, relative, extension });
      }
    }
  }
  return files;
}

async function sha256(filename) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(filename)) hash.update(chunk);
  return hash.digest('hex');
}

function categoryFor(filename) {
  const name = filename.toLowerCase();
  if (name.includes('logo') || name.includes('aramya-mark') || name.includes('aramya-')) return 'brand';
  if (/^(saree|halfsaree)/.test(name)) return 'products/sarees';
  if (/^(lehanga|bridal)/.test(name)) return 'products/bridal';
  if (/^(menkurtha|menblazer)/.test(name)) return 'products/menswear';
  if (/^kidzgown/.test(name)) return 'products/kids';
  if (/^(gown|sharara)/.test(name)) return 'products/occasion';
  return 'collections';
}

function candidateKey(filename, digest, usedKeys) {
  const extension = path.extname(filename).toLowerCase();
  const stem = path.basename(filename, extension)
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '') || 'asset';
  const base = `${categoryFor(filename)}/${stem}`;
  let key = `${base}${extension}`;
  if (usedKeys.has(key)) key = `${base}-${digest.slice(0, 12)}${extension}`;
  usedKeys.add(key);
  return key;
}

const allFiles = [];
for (const root of sourceRoots) {
  allFiles.push(...(await filesBelow(path.join(repositoryRoot, root.path), root.path)).map((file) => ({ ...file, sourceRole: root.role })));
}
allFiles.sort((a, b) => a.relative.localeCompare(b.relative, 'en'));

const grouped = new Map();
for (const file of allFiles) {
  const info = await stat(file.absolute);
  const digest = await sha256(file.absolute);
  const group = grouped.get(digest) ?? { sha256: digest, byteSize: info.size, copies: [] };
  if (group.byteSize !== info.size) throw new Error(`SHA-256 collision with different sizes detected: ${digest}`);
  group.copies.push({ path: file.relative, sourceRole: file.sourceRole, mimeType: mimeTypes.get(file.extension), byteSize: info.size });
  grouped.set(digest, group);
}

const usedKeys = new Set();
const assets = [...grouped.values()].sort((a, b) => a.copies[0].path.localeCompare(b.copies[0].path, 'en')).map((group) => {
  const canonical = [...group.copies].sort((a, b) => {
    const priority = { original: 0, 'pwa-brand': 1, 'native-brand': 2, 'legacy-dist-copy': 3 };
    return priority[a.sourceRole] - priority[b.sourceRole] || a.path.localeCompare(b.path, 'en');
  })[0];
  const filename = path.posix.basename(canonical.path);
  return {
    sha256: group.sha256,
    byteSize: group.byteSize,
    mimeType: canonical.mimeType,
    canonicalSource: canonical.path,
    objectKeyCandidate: candidateKey(filename, group.sha256, usedKeys),
    duplicatePaths: group.copies.filter((copy) => copy.path !== canonical.path).map((copy) => copy.path),
  };
});

const duplicateGroups = assets.filter((asset) => asset.duplicatePaths.length > 0).map(({ sha256: digest, byteSize, canonicalSource, duplicatePaths }) => ({
  sha256: digest,
  byteSize,
  canonicalSource,
  duplicatePaths,
}));
const keysBySourcePath = new Map();
for (const asset of assets) {
  keysBySourcePath.set(asset.canonicalSource, asset.objectKeyCandidate);
  for (const duplicatePath of asset.duplicatePaths) keysBySourcePath.set(duplicatePath, asset.objectKeyCandidate);
}
const catalog = JSON.parse(await readFile(path.join(repositoryRoot, 'database/seeds/catalog.json'), 'utf8'));
function catalogMediaReference(record, type) {
  const sourcePath = String(record.image_key ?? '').replaceAll('\\', '/');
  const objectKeyCandidate = keysBySourcePath.get(sourcePath);
  if (!objectKeyCandidate) throw new Error(`${type} ${record.slug} refers to missing local media: ${sourcePath}`);
  return { id: record.id, slug: record.slug, sourcePath, objectKeyCandidate };
}
const sourceCounts = {};
for (const file of allFiles) {
  const item = sourceCounts[file.sourceRole] ?? { fileCopies: 0, bytes: 0 };
  const info = await stat(file.absolute);
  item.fileCopies += 1;
  item.bytes += info.size;
  sourceCounts[file.sourceRole] = item;
}
const manifest = {
  version: 1,
  generatedBy: 'scripts/migrate-r2/create-media-manifest.mjs',
  sourceRoots: sourceRoots.map(({ path: sourcePath, role }) => ({ path: sourcePath, role })),
  sourceCounts,
  totals: {
    fileCopies: allFiles.length,
    uniqueBinaries: assets.length,
    duplicateCopies: allFiles.length - assets.length,
    duplicateGroups: duplicateGroups.length,
    totalUniqueBytes: assets.reduce((sum, asset) => sum + asset.byteSize, 0),
  },
  catalogReferences: {
    categoryImages: catalog.categories.map((record) => catalogMediaReference(record, 'category')),
    products: catalog.products.map((record) => catalogMediaReference(record, 'product')),
  },
  uploadCandidates: assets,
  duplicateGroups,
};

await mkdir(path.dirname(outputPath), { recursive: true });
const serialized = `${JSON.stringify(manifest, null, 2)}\n`;
const checkOnly = process.argv.includes('--check');
if (checkOnly) {
  let existing = '';
  try { existing = await readFile(outputPath, 'utf8'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  if (existing !== serialized) {
    console.error('Media manifest is missing or stale. Run: node scripts/migrate-r2/create-media-manifest.mjs');
    process.exitCode = 1;
  } else {
    console.log(`Media manifest is current: ${manifest.totals.fileCopies} file copies, ${manifest.totals.uniqueBinaries} unique binaries, ${manifest.totals.duplicateCopies} duplicate copies.`);
  }
} else {
  await writeFile(outputPath, serialized, 'utf8');
  console.log(`Wrote docs/asset-migration-manifest.json: ${manifest.totals.fileCopies} file copies, ${manifest.totals.uniqueBinaries} unique binaries, ${manifest.totals.duplicateCopies} duplicate copies.`);
}
