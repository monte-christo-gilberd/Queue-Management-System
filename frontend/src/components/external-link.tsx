/**
 * ExternalLink: tautan ke URL eksternal dari sisa template Expo.
 *
 * Dipakai oleh : tidak ada
 * Bergantung   : expo-router (Link), expo-web-browser (openBrowserAsync)
 * Peran        : semua
 * Catatan      : TIDAK DIPAKAI: sisa template Expo. Di native, tautan dibuka di browser dalam aplikasi.
 */
import { Href, Link } from 'expo-router';
import { openBrowserAsync, WebBrowserPresentationStyle } from 'expo-web-browser';
import { type ComponentProps } from 'react';

type Props = Omit<ComponentProps<typeof Link>, 'href'> & { href: Href & string };

/** Tautan yang di web membuka tab baru dan di native membuka browser dalam aplikasi. */
export function ExternalLink({ href, ...rest }: Props) {
  return (
    <Link
      target="_blank"
      {...rest}
      href={href}
      onPress={async (event) => {
        if (process.env.EXPO_OS !== 'web') {
          // Mencegah pembukaan browser bawaan perangkat agar pengguna tetap berada di aplikasi.
          event.preventDefault();
          // Membuka URL di browser dalam aplikasi dengan gaya presentasi otomatis.
          await openBrowserAsync(href, {
            presentationStyle: WebBrowserPresentationStyle.AUTOMATIC,
          });
        }
      }}
    />
  );
}
