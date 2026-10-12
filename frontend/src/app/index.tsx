import { router } from 'expo-router';
import { useState } from 'react';

import LoginScreen from '@/components/LoginScreen';

export default function LoginRoute() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  return (
    <LoginScreen
      username={username}
      password={password}
      onChangeUsername={setUsername}
      onChangePassword={setPassword}
      // TODO(developer): ganti dengan logika login (fetch + store token). Sementara langsung ke dashboard.
      onSubmit={() => router.replace('/dashboard')}
      loading={false}
    />
  );
}
