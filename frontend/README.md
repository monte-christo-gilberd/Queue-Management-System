# Welcome to your Expo app 👋

This is an [Expo](https://expo.dev) project created with [`create-expo-app`](https://www.npmjs.com/package/create-expo-app).

## Get started

1. Install dependencies

   ```bash
   npm install
   ```

2. Start the app

   ```bash
   npx expo start
   ```

In the output, you'll find options to open the app in a

- [development build](https://docs.expo.dev/develop/development-builds/introduction/)
- [Android emulator](https://docs.expo.dev/workflow/android-studio-emulator/)
- [iOS simulator](https://docs.expo.dev/workflow/ios-simulator/)
- [Expo Go](https://expo.dev/go), a limited sandbox for trying out app development with Expo

You can start developing by editing the files inside the **src/app** directory. This project uses [file-based routing](https://docs.expo.dev/router/introduction).

### Other setup steps

- To set up ESLint for linting, run `npx expo lint`, or follow our guide on ["Using ESLint and Prettier"](https://docs.expo.dev/guides/using-eslint/)
- If you'd like to set up unit testing, follow our guide on ["Unit Testing with Jest"](https://docs.expo.dev/develop/unit-testing/)
- Learn more about the TypeScript setup in this template in our guide on ["Using TypeScript"](https://docs.expo.dev/guides/typescript/)

> Catatan: perintah `npm run reset-project` (template Expo) sudah dihapus dari proyek ini karena tidak dipakai. Jangan dijalankan ulang: ia memindahkan `src/` dan `scripts/` ke `example/`.

## Perintah

| Perintah | Fungsi |
| --- | --- |
| `npm start` | Dev server Expo. |
| `npm run web` / `npm run web:lan` | Dev server web (lan: bisa dibuka dari perangkat lain). |
| `npm run android` / `npm run ios` | Build native lokal (`expo run:android` / `expo run:ios`). |
| `npm run lint` | `expo lint`. |
| `npm run test:unit` | Tes unit (Node test runner). |
| `npm run e2e` / `npm run e2e:report` | Tes E2E Playwright dan laporannya. |
| `npm run icons` | Membuat ulang semua ikon PNG di `assets/images/` dari `logo/logo.svg` (`scripts/gen-icons.mjs`, memakai Chromium Playwright). Ikon ada di dalam APK, jadi build ulang APK setelahnya. |

## Struktur folder

- `src/app/`: rute (Expo Router). Publik/auth: `index.tsx`, `login`, `register`, `forgot-password`, `reset-password`, `verify-email`, `link-google`, `join`, `ticket/[queueId]`, `display/[token]`. Grup `(user)/` (setelah login; sebelumnya `(owner)`): `dashboard`, `antrian`, `riwayat`, `history/` (`index`, `[queueId]`), `queues/` (`index`, `[queueId]/index`, `[queueId]/live`), `profile/`, `subscription`.
- `src/components/` (layar dan kartu; `ui/` berisi komponen dasar seperti `AppButton`, `Toast`, `ConfirmModal`, `UserAvatar`), `src/hooks/`, `src/lib/` (klien API, SSE, push), `src/store/`, `src/utils/` (mis. `menu.ts`), `src/types/`, `src/constants/`.
- `tests/unit/` (tes unit), `playwright/` (E2E dan test case), `scripts/` (`gen-icons.mjs`), `logo/`, `assets/`.

## Tes

- Tes unit: `npm run test:unit` (folder `tests/unit/`, 208 tes di 27 berkas `*.test.ts`, semua lulus pada pengecekan terakhir).
- Tes E2E Playwright: `npm run e2e` (folder `playwright/unit-test/`, 247 tes di 33 file spec menurut `npx playwright test --list`). Cara menjalankan
  dan prasyaratnya ada di [`playwright/unit-test/README.md`](playwright/unit-test/README.md).

## Build dan notifikasi

- Web: `Dockerfile` (target `dev`, `build`, `prod`): `expo export` lalu disajikan nginx (`nginx.conf`). `EXPO_PUBLIC_API_URL` dibakar ke bundle saat build.
- Android (APK): profil di `eas.json` (`development`, `preview`, `production`; keduanya APK). Build: `npx eas-cli@latest build --platform android --profile production`.
  Env `EXPO_PUBLIC_API_URL` dan client ID Google untuk profil production diisi di `eas.json`.
- Push: Web memakai Web Push (`src/lib/push.ts`); Android memakai FCM (`src/lib/push-native*.ts`, kanal `queue-turn`, token didaftarkan ke
  `POST /api/v1/push/fcm/register`). FCM butuh `google-services.json`: di EAS unggah sebagai file env `GOOGLE_SERVICES_JSON`
  (`eas env:create --name GOOGLE_SERVICES_JSON --type file --visibility secret ...`); lokal taruh di `frontend/` (di-gitignore).
  `app.config.js` yang menyambungkannya. Tanpa berkas itu build tetap jalan, tetapi notifikasi Android tidak aktif.

## Catatan perilaku layar

- Halaman Daftar dan Reset Password: pesan "Password tidak sama" tampil inline di bawah field konfirmasi, hanya setelah field itu
  ditinggalkan atau tombol kirim ditekan, dan hilang saat isinya sama.
- Kartu profil: foto bisa diketuk untuk membuka popup foto yang diperbesar (tombol X menutupnya). Tanpa foto, tidak ada yang bisa diketuk.
- Layar langganan owner (`src/app/(user)/subscription.tsx`) membuat pesanan pembayaran QRIS BorderPay lewat
  `POST /api/v1/owners/subscription/checkout` (satu bulan langganan, 30 hari).

## Learn more

To learn more about developing your project with Expo, look at the following resources:

- [Expo documentation](https://docs.expo.dev/): Learn fundamentals, or go into advanced topics with our [guides](https://docs.expo.dev/guides).
- [Learn Expo tutorial](https://docs.expo.dev/tutorial/introduction/): Follow a step-by-step tutorial where you'll create a project that runs on Android, iOS, and the web.

## Join the community

Join our community of developers creating universal apps.

- [Expo on GitHub](https://github.com/expo/expo): View our open source platform and contribute.
- [Discord community](https://chat.expo.dev): Chat with Expo users and ask questions.
