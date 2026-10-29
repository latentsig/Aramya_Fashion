import { cp, mkdir, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const frontendDirectory = path.dirname(fileURLToPath(import.meta.url));
const sourceDirectory = path.resolve(frontendDirectory, '../../image assests');
const publicDirectory = path.resolve(frontendDirectory, '../public');
const destinationDirectory = path.join(publicDirectory, 'media');

await mkdir(publicDirectory, { recursive: true });
await rm(destinationDirectory, { recursive: true, force: true });
await cp(sourceDirectory, destinationDirectory, { recursive: true });
console.log(`Synced storefront media from ${sourceDirectory}`);
