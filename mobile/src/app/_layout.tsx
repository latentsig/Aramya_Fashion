import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Brand } from '@/constants/brand';
import { CatalogProvider } from '@/lib/catalog';

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <CatalogProvider>
        <StatusBar style="light" />
        <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: Brand.paper } }}>
          <Stack.Screen name="(tabs)" />
          <Stack.Screen name="product/[slug]" options={{ animation: 'slide_from_right' }} />
        </Stack>
      </CatalogProvider>
    </SafeAreaProvider>
  );
}
