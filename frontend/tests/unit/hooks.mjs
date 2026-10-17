// Hook loader Node untuk `node --test`. Tanpa dependensi tambahan:
// - alias `@/` dipetakan ke `src/` (seperti tsconfig paths);
// - import relatif tanpa ekstensi dicarikan `.ts` / `index.ts` (gaya bundler Expo/Metro);
// - file `.ts` dijalankan lewat type stripping bawaan Node.
import { existsSync, statSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const srcRoot = fileURLToPath(new URL('../../src/', import.meta.url));

function withTsExtension(filePath) {
  if (existsSync(filePath) && statSync(filePath).isFile()) return filePath;
  for (const candidate of [`${filePath}.ts`, `${filePath}.tsx`, path.join(filePath, 'index.ts')]) {
    if (existsSync(candidate)) return candidate;
  }
  return null;
}

export async function resolve(specifier, context, nextResolve) {
  let target = null;
  if (specifier.startsWith('@/')) {
    target = withTsExtension(path.join(srcRoot, specifier.slice(2)));
  } else if ((specifier.startsWith('./') || specifier.startsWith('../')) && context.parentURL?.startsWith('file:')) {
    const base = path.resolve(path.dirname(fileURLToPath(context.parentURL)), specifier);
    target = path.extname(base) ? null : withTsExtension(base);
  }
  if (target) return { url: pathToFileURL(target).href, shortCircuit: true };
  return nextResolve(specifier, context);
}

export async function load(url, context, nextLoad) {
  if (url.endsWith('.ts') || url.endsWith('.tsx')) {
    return nextLoad(url, { ...context, format: 'module-typescript' });
  }
  return nextLoad(url, context);
}
