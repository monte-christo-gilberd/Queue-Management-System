// checks.js: pemeriksaan respons yang dipakai skenario. Satu fungsi per jenis hasil yang diharapkan.
import { check } from 'k6';

// okStatus memeriksa status respons termasuk dalam daftar yang diharapkan. label dipakai sebagai nama check
// (muncul di ringkasan k6 sebagai "<label>: status ok").
export function okStatus(res, label, expected) {
  const want = expected || [200];
  return check(res, { [`${label}: status ok`]: (r) => want.indexOf(r.status) >= 0 });
}

// hasData memeriksa body JSON punya field data (bentuk {"data": ...} semua respons sukses backend).
export function hasData(res, label) {
  return check(res, {
    [`${label}: body ada data`]: (r) => {
      try {
        return r.json('data') !== undefined && r.json('data') !== null;
      } catch (e) {
        return false;
      }
    },
  });
}
