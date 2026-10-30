import { NativeTabs } from 'expo-router/native-tabs';
import { Brand } from '@/constants/brand';

export default function TabLayout() {
  return (
    <NativeTabs backgroundColor={Brand.paper} indicatorColor="#E6DCD3" labelStyle={{ selected: { color: Brand.red } }}>
      <NativeTabs.Trigger name="index"><NativeTabs.Trigger.Label>Home</NativeTabs.Trigger.Label></NativeTabs.Trigger>
      <NativeTabs.Trigger name="explore"><NativeTabs.Trigger.Label>Collections</NativeTabs.Trigger.Label></NativeTabs.Trigger>
      <NativeTabs.Trigger name="studio"><NativeTabs.Trigger.Label>Studio</NativeTabs.Trigger.Label></NativeTabs.Trigger>
      <NativeTabs.Trigger name="cart"><NativeTabs.Trigger.Label>Bag</NativeTabs.Trigger.Label></NativeTabs.Trigger>
    </NativeTabs>
  );
}
