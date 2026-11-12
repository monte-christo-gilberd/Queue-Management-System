// summary.js: ringkasan akhir uji: teks singkat di terminal dan berkas JSON di perf/reports/ (diabaikan git).
//
// k6 hanya mencetak ringkasan bawaan bila handleSummary tidak didefinisikan; karena skenario mendefinisikannya, teks
// di sini menggantikan ringkasan bawaan. Isinya: latensi per endpoint penting (p50/p95/p99), jumlah request, galat,
// 429, dan hasil tiap threshold (gerbang uji).

// ms memformat angka milidetik.
function ms(v) {
  return v === undefined ? '   -  ' : v.toFixed(1).padStart(7) + 'ms';
}

// metric mengambil nilai statistik dari data ringkasan; kembali undefined bila metrik belum ada.
function metric(data, name, stat) {
  const m = data.metrics[name];
  return m && m.values ? m.values[stat] : undefined;
}

// text menyusun ringkasan teks. endpoints = nama tag `name` yang ingin ditampilkan latensinya.
function text(data, label, endpoints) {
  const lines = [];
  lines.push('');
  lines.push(`== Ringkasan uji: ${label}`);
  const reqs = metric(data, 'http_reqs', 'count') || 0;
  const rate = metric(data, 'http_reqs', 'rate') || 0;
  const failed = metric(data, 'http_req_failed', 'rate') || 0;
  lines.push(`   request: ${reqs} (${rate.toFixed(1)}/s)  gagal: ${(failed * 100).toFixed(2)}%  429: ${metric(data, 'perf_http_429', 'count') || 0}`);
  lines.push(`   check lulus: ${((metric(data, 'checks', 'rate') || 0) * 100).toFixed(2)}%`);
  lines.push('   latensi per endpoint            p50        p95        p99        maks   jumlah');
  for (const name of endpoints) {
    const key = `http_req_duration{name:${name}}`;
    const m = data.metrics[key];
    if (!m || !m.values) continue;
    const c = data.metrics[`http_reqs{name:${name}}`];
    lines.push(`   ${name.padEnd(30)} ${ms(m.values.med)} ${ms(m.values['p(95)'])} ${ms(m.values['p(99)'])} ${ms(m.values.max)} ${c && c.values ? c.values.count : ''}`);
  }
  // hasil gerbang: tiap threshold dengan tanda lulus atau gagal
  const gates = [];
  for (const [name, m] of Object.entries(data.metrics)) {
    if (!m.thresholds) continue;
    for (const [expr, res] of Object.entries(m.thresholds)) {
      gates.push(`   ${res.ok ? 'LULUS' : 'GAGAL'}  ${name}  ${expr}`);
    }
  }
  if (gates.length) {
    lines.push('   gerbang:');
    for (const g of gates.sort()) lines.push(g);
  }
  lines.push('');
  return lines.join('\n');
}

// build mengembalikan objek untuk handleSummary: teks ke stdout dan JSON lengkap ke berkas laporan.
// label = nama skenario; endpoints = daftar nama tag untuk tabel latensi.
export function build(data, label, endpoints) {
  const dir = __ENV.PERF_REPORT_DIR || 'perf/reports';
  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  const out = { stdout: text(data, label, endpoints) };
  out[`${dir}/${label}-${stamp}.json`] = JSON.stringify(data, null, 2);
  return out;
}
