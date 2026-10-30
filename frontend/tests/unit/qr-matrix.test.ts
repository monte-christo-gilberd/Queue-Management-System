/// <reference types="node" />
// QR digambar di frontend dari join_url (plan 4.2). Fungsi murni, tanpa DOM.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { buildQrMatrix, buildQrPath, QR_QUIET_ZONE, renderQrPngDataUrl } from '@/lib/qr-matrix';

const URL_A = 'http://localhost:8081/join?t=v1.eyJxaWQiOjF9.abc123';

test('matriks QR persegi dan berisi modul gelap', () => {
  const m = buildQrMatrix(URL_A);
  assert.ok(m.size >= 21);
  let dark = 0;
  for (let r = 0; r < m.size; r++) for (let c = 0; c < m.size; c++) if (m.isDark(r, c)) dark++;
  assert.ok(dark > 0 && dark < m.size * m.size);
});

test('tiga penanda sudut ada di kiri atas, kanan atas, dan kiri bawah', () => {
  const m = buildQrMatrix(URL_A);
  const last = m.size - 1;
  for (const [r, c] of [[0, 0], [0, last], [last, 0]]) assert.equal(m.isDark(r, c), true);
  assert.equal(m.isDark(last, last), false);
});

test('teks yang sama menghasilkan QR yang sama, token berbeda menghasilkan QR berbeda', () => {
  assert.equal(buildQrPath(buildQrMatrix(URL_A)).d, buildQrPath(buildQrMatrix(URL_A)).d);
  assert.notEqual(buildQrPath(buildQrMatrix(URL_A)).d, buildQrPath(buildQrMatrix(`${URL_A}x`)).d);
});

test('path SVG memakai zona tenang dan viewBox = ukuran + 2 x zona tenang', () => {
  const m = buildQrMatrix(URL_A);
  const { d, viewBox } = buildQrPath(m);
  assert.equal(viewBox, m.size + QR_QUIET_ZONE * 2);
  assert.ok(d.startsWith(`M${QR_QUIET_ZONE} ${QR_QUIET_ZONE}`), 'modul pertama (sudut kiri atas) digeser oleh zona tenang');
});

test('unduh PNG hanya di web: tanpa document hasilnya null', () => {
  assert.equal(renderQrPngDataUrl(URL_A), null);
});
