import { createContext } from 'react';

/**
 * True bila user yang login adalah owner. Disediakan oleh layout (owner) dari GET /users/me.
 * Dipakai layout untuk menampilkan menu khusus owner di panel profil.
 */
export const OwnerContext = createContext<boolean>(false);
