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

## Tes

- Tes unit: `npm run test:unit` (folder `tests/unit/`, 193 tes pada pengecekan terakhir).
- Tes E2E Playwright: `npm run e2e` (folder `playwright/unit-test/`, 246 tes di 33 file spec menurut `npx playwright test --list`). Cara menjalankan
  dan prasyaratnya ada di [`playwright/unit-test/README.md`](playwright/unit-test/README.md).

## Catatan perilaku layar

- Halaman Daftar dan Reset Password: pesan "Password tidak sama" tampil inline di bawah field konfirmasi, hanya setelah field itu
  ditinggalkan atau tombol kirim ditekan, dan hilang saat isinya sama.
- Kartu profil: foto bisa diketuk untuk membuka popup foto yang diperbesar (tombol X menutupnya). Tanpa foto, tidak ada yang bisa diketuk.
- Layar langganan owner (`src/app/(owner)/subscription.tsx`) adalah MOCK: satu tombol menambah masa langganan 30 hari lewat
  `POST /api/v1/owners/subscription/extend`, tanpa pembayaran.

## Learn more

To learn more about developing your project with Expo, look at the following resources:

- [Expo documentation](https://docs.expo.dev/): Learn fundamentals, or go into advanced topics with our [guides](https://docs.expo.dev/guides).
- [Learn Expo tutorial](https://docs.expo.dev/tutorial/introduction/): Follow a step-by-step tutorial where you'll create a project that runs on Android, iOS, and the web.

## Join the community

Join our community of developers creating universal apps.

- [Expo on GitHub](https://github.com/expo/expo): View our open source platform and contribute.
- [Discord community](https://chat.expo.dev): Chat with Expo users and ask questions.
