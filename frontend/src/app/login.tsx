import { Redirect } from 'expo-router';

// Login sekarang ada di "/" — route lama /login diarahkan ke sana.
export default function LoginRedirect() {
  return <Redirect href="/" />;
}
