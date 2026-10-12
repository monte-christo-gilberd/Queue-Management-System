const { getDefaultConfig } = require("expo/metro-config");
const { withNativeWind } = require("nativewind/metro");
const config = getDefaultConfig(__dirname);
// inlineRem: 16 → 1rem = 16px (default NativeWind 14px) supaya skala Tailwind (p-4 = 16px) sama dengan nilai px di mockup.
module.exports = withNativeWind(config, { input: "./src/global.css", inlineRem: 16 });
