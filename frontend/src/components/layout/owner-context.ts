/**
 * OwnerContext: status owner untuk seluruh layout owner.
 *
 * Dipakai oleh : app/(user)/_layout.tsx (UserSessionProvider), components/layout/ResponsiveScreenLayout.tsx
 * Alur         : app/(user)/_layout.tsx mengambil GET /users/me -> OwnerContext -> menu owner di panel profil
 */
import { createContext } from 'react';

/**
 * True bila user yang login adalah owner. Disediakan oleh layout (user) dari GET /users/me.
 * Dipakai layout untuk menampilkan menu khusus owner di panel profil.
 * Default false: di luar provider owner, menu khusus owner tidak tampil.
 */
export const OwnerContext = createContext<boolean>(false);
