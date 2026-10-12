import Dashboard from '@/components/Dashboard';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function DashboardRoute() {
  // TODO(developer): ambil queue aktif + tiket (GET /queues, SSE) dan sambungkan onNext dsb.
  return <Dashboard onMenuItemPress={handleOwnerMenuItemPress} />;
}
