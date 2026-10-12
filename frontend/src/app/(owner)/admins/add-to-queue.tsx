import AddAdminToQueue from '@/components/AddAdminToQueue';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function AddAdminToQueueRoute() {
  // TODO(developer): ambil queue aktif + admin yang belum aktif (belum ada endpoint di backend).
  return (
    <AddAdminToQueue admins={[]} onAdd={() => {}} onMenuItemPress={handleOwnerMenuItemPress} />
  );
}
