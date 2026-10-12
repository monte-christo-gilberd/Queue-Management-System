import { useState } from 'react';

import Profile from '@/components/Profile';
import { handleOwnerMenuItemPress } from '@/utils/owner-menu';

export default function ProfileRoute() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  // Pengecekan kesamaan password ada di route (bukan di komponen tampilan).
  const confirmPasswordError =
    confirmPassword && confirmPassword !== password ? 'Password tidak sama' : undefined;
  // TODO(developer): isi nilai awal dari data user + sambungkan simpan perubahan + status subscription
  // (subscriptionPaid; sementara dianggap belum bayar karena pengecekan belum ada).

  return (
    <Profile
      name={name}
      email={email}
      password={password}
      confirmPassword={confirmPassword}
      onChangeName={setName}
      onChangeEmail={setEmail}
      onChangePassword={setPassword}
      onChangeConfirmPassword={setConfirmPassword}
      confirmPasswordError={confirmPasswordError}
      onSubmit={() => {}}
      loading={false}
      subscriptionPaid={false}
      onMenuItemPress={handleOwnerMenuItemPress}
    />
  );
}
