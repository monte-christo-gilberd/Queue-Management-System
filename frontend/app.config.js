/**
 * app.config.js: memperluas app.json. Satu-satunya tambahan: lokasi google-services.json (Firebase/FCM Android).
 *
 * - EAS Build: unggah berkas sebagai EAS file environment variable bernama GOOGLE_SERVICES_JSON
 *   (`eas env:create --name GOOGLE_SERVICES_JSON --type file --visibility secret ...`); EAS mengisi env ini
 *   dengan path berkasnya.
 * - Lokal: taruh google-services.json di folder frontend/ (di-gitignore).
 * Tanpa keduanya build tetap jalan, tetapi notifikasi Android tidak aktif (getDevicePushTokenAsync gagal).
 */
const fs = require('node:fs');
const path = require('node:path');

module.exports = ({ config }) => {
  // __dirname: folder frontend/ (tempat berkas ini), tidak bergantung pada folder kerja saat build.
  // eslint-disable-next-line no-undef
  const local = path.join(__dirname, 'google-services.json');
  // Prioritas: berkas dari EAS (env), lalu berkas lokal. Tidak ada keduanya: config dikembalikan apa adanya.
  const googleServicesFile =
    process.env.GOOGLE_SERVICES_JSON || (fs.existsSync(local) ? './google-services.json' : undefined);
  if (!googleServicesFile) return config;
  // android.googleServicesFile membuat prebuild menyalin berkas dan memasang plugin Gradle google-services.
  return { ...config, android: { ...config.android, googleServicesFile } };
};
