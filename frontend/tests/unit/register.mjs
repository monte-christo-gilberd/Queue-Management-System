// Dipanggil lewat `node --import ./tests/unit/register.mjs --test ...` (lihat script `test:unit`).
import { register } from 'node:module';

register(new URL('./hooks.mjs', import.meta.url));
