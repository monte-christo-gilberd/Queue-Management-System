import { useEffect, useState } from 'react';

import { getMe, type Me } from '@/lib/users';

/**
 * Profil user yang login. `loaded` false selama request berjalan; selama itu jangan tampilkan fitur owner.
 */
export function useMe() {
  const [me, setMe] = useState<Me | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let active = true;
    void getMe().then(
      (data) => {
        if (active) {
          setMe(data);
          setLoaded(true);
        }
      },
      () => {
        if (active) setLoaded(true);
      },
    );
    return () => {
      active = false;
    };
  }, []);

  return { me, isOwner: me?.is_owner === true, loaded };
}
