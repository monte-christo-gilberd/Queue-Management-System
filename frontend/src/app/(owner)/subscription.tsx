import Subscription from '@/components/Subscription';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function SubscriptionRoute() {
  // TODO(developer): ambil status subscription (belum ada endpoint di backend).
  return (
    <Subscription
      paid={false}
      title=""
      subtitle=""
      username=""
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
