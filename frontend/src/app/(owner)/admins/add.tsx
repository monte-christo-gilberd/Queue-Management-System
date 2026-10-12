import { useState } from 'react';

import AddToSavedAdmin from '@/components/AddToSavedAdmin';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function AddToSavedAdminRoute() {
  const [search, setSearch] = useState('');
  // TODO(developer): cari user terdaftar (belum ada endpoint di backend).

  return (
    <AddToSavedAdmin
      search={search}
      onChangeSearch={setSearch}
      results={[]}
      onAdd={() => {}}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
