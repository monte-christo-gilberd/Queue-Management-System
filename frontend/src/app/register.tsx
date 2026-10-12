import { useState } from 'react';

import RegisterScreen from '@/components/RegisterScreen';

export default function RegisterRoute() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  // Pengecekan kesamaan password ada di route (bukan di komponen tampilan).
  const confirmPasswordError =
    confirmPassword && confirmPassword !== password ? 'Password tidak sama' : undefined;
  // TODO(developer): sambungkan logika register (POST /users/register + simpan token).

  return (
    <RegisterScreen
      username={username}
      email={email}
      password={password}
      confirmPassword={confirmPassword}
      onChangeUsername={setUsername}
      onChangeEmail={setEmail}
      onChangePassword={setPassword}
      onChangeConfirmPassword={setConfirmPassword}
      confirmPasswordError={confirmPasswordError}
      onSubmit={() => {}}
      loading={false}
    />
  );
}
