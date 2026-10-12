import { ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LogoMark } from '@/components/auth/LogoMark';

type AuthLayoutProps = {
  title: string;
  subtitle: string;
  children: React.ReactNode;
};

/** Kerangka halaman auth: latar gelap penuh + blob organik halus + logo di tengah-atas + form ditengahkan. */
export function AuthLayout({ title, subtitle, children }: AuthLayoutProps) {
  return (
    <View className="flex-1 overflow-hidden bg-dark">
      {/* Blob dekoratif: bentuk organik beropasitas rendah di sudut-sudut (rotasi lewat style: transform). */}
      <View
        className="absolute -left-[160px] -top-[180px] h-[460px] w-[560px] rounded-bl-[300px] rounded-br-[200px] rounded-tl-[260px] rounded-tr-[320px] bg-primary opacity-[0.14]"
        style={{ transform: [{ rotate: '-14deg' }] }}
        pointerEvents="none"
      />
      <View
        className="absolute -bottom-[220px] -right-[180px] h-[520px] w-[640px] rounded-bl-[260px] rounded-br-[300px] rounded-tl-[340px] rounded-tr-[220px] bg-primary opacity-[0.12]"
        style={{ transform: [{ rotate: '18deg' }] }}
        pointerEvents="none"
      />
      <View
        className="absolute -bottom-[120px] -left-[140px] h-[380px] w-[380px] rounded-full bg-primarySoft opacity-[0.08]"
        pointerEvents="none"
      />

      {/* SafeAreaView dari safe-area-context tidak dibungkus NativeWind → style. */}
      <SafeAreaView style={{ flex: 1 }}>
        <ScrollView
          contentContainerClassName="grow items-center justify-center p-6"
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}>
          <View className="w-full max-w-[360px] items-stretch">
            <View className="mb-5 items-center">
              <LogoMark />
            </View>
            <Text className="text-center text-[24px] font-black text-white">{title}</Text>
            <Text className="mb-7 mt-1 text-center text-[13px] text-placeholder">{subtitle}</Text>
            <View className="gap-3.5">{children}</View>
          </View>
        </ScrollView>
      </SafeAreaView>
    </View>
  );
}
