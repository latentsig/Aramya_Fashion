import { useState } from 'react';
import { Link } from 'expo-router';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { BrandHeader } from '@/components/BrandHeader';
import { Brand } from '@/constants/brand';
import { createDemoOrder, formatPrice, type CheckoutOrder, type CheckoutQuote } from '@/lib/api';

type Form = { name: string; email: string; phone: string; address: string; city: string; region: string; postalCode: string };
const emptyForm: Form = { name: '', email: '', phone: '', address: '', city: '', region: '', postalCode: '' };
const fields: { key: keyof Form; label: string; keyboard?: 'email-address' | 'phone-pad' }[] = [
  { key: 'name', label: 'FULL NAME' }, { key: 'email', label: 'EMAIL ADDRESS', keyboard: 'email-address' },
  { key: 'phone', label: 'PHONE NUMBER', keyboard: 'phone-pad' }, { key: 'address', label: 'DELIVERY ADDRESS' },
  { key: 'city', label: 'CITY' }, { key: 'region', label: 'STATE' }, { key: 'postalCode', label: 'POSTAL CODE', keyboard: 'phone-pad' },
];

export default function CheckoutScreen() {
  const [form, setForm] = useState(emptyForm);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<{ order: CheckoutOrder; quote: CheckoutQuote } | null>(null);

  async function submit() {
    if (Object.values(form).some((value) => !value.trim()) || !/^\S+@\S+\.\S+$/.test(form.email)) {
      setError('Enter your contact and delivery details, including a valid email.'); return;
    }
    setBusy(true); setError('');
    try { setResult(await createDemoOrder(form, `mobile-demo-${Date.now()}-${Math.random().toString(36).slice(2)}`)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not save the demo order. Please try again.'); }
    finally { setBusy(false); }
  }

  return <SafeAreaView edges={['top']} style={styles.safe}><BrandHeader /><ScrollView contentContainerStyle={styles.content}>
    <Text style={styles.eyebrow}>SAMPLE DEMO</Text><Text style={styles.title}>Delivery details</Text>
    {result ? <View style={styles.success}>
      <Text style={styles.successTitle}>Your order is saved.</Text><Text style={styles.body}>Order {result.order.orderNumber}</Text>
      <Text style={styles.body}>Status: {result.order.status.replaceAll('_', ' ')}</Text>
      <Text style={styles.price}>{formatPrice(result.order.totalPaise)}</Text>
      <Text style={styles.note}>This is a pending demo order. No payment was collected.</Text>
      <Link href="/(tabs)/cart" asChild><Pressable style={styles.button}><Text style={styles.buttonText}>BACK TO YOUR BAG</Text></Pressable></Link>
    </View> : <>
      <Text style={styles.note}>A simple demo order will be saved to the API. Payment is not enabled here.</Text>
      {fields.map(({ key, label, keyboard }) => <View key={key} style={styles.field}>
        <Text style={styles.label}>{label}</Text><TextInput value={form[key]} onChangeText={(value) => setForm((current) => ({ ...current, [key]: value }))} keyboardType={keyboard ?? 'default'} autoCapitalize={key === 'email' ? 'none' : 'words'} style={styles.input} />
      </View>)}
      {error ? <Text accessibilityRole="alert" style={styles.error}>{error}</Text> : null}
      <Pressable disabled={busy} onPress={() => void submit()} style={[styles.button, busy && styles.disabled]}>{busy ? <ActivityIndicator color={Brand.white} /> : <Text style={styles.buttonText}>SAVE DEMO ORDER</Text>}</Pressable>
    </>}
  </ScrollView></SafeAreaView>;
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper }, content: { padding: 22, paddingBottom: 48 },
  eyebrow: { color: Brand.red, fontSize: 8, letterSpacing: 1.5 }, title: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 34, marginTop: 8 },
  note: { color: Brand.muted, fontSize: 10, lineHeight: 17, marginTop: 12 }, field: { marginTop: 17 }, label: { color: Brand.ink, fontSize: 8, letterSpacing: 1 },
  input: { minHeight: 43, borderBottomWidth: 1, borderBottomColor: Brand.line, color: Brand.ink, fontSize: 13, paddingHorizontal: 0 },
  button: { minHeight: 48, alignItems: 'center', justifyContent: 'center', backgroundColor: Brand.red, marginTop: 24 }, buttonText: { color: Brand.white, fontSize: 9, letterSpacing: 1 }, disabled: { opacity: 0.6 },
  error: { color: Brand.red, fontSize: 10, marginTop: 15 }, success: { marginTop: 36, padding: 22, backgroundColor: '#F0E8E1' }, successTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 25 }, body: { color: Brand.ink, fontSize: 12, marginTop: 12 }, price: { color: Brand.red, fontFamily: 'Georgia', fontSize: 22, marginTop: 20 },
});