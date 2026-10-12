import { useState } from 'react';

import AdminContact from '@/components/AdminContact';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function AdminContactRoute() {
  const [search, setSearch] = useState('');
  // TODO(developer): ambil daftar admin (belum ada endpoint di backend).

  return (
    <AdminContact
      admins={[]}
      search={search}
      onChangeSearch={setSearch}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
