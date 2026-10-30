import { useRef, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { BrandHeader } from '@/components/BrandHeader';
import { Brand } from '@/constants/brand';
import { createAppointment, createCustomOrderRequest, newStudioRequestKey, type StudioRequestResult } from '@/lib/api';

type RequestKind = 'appointment' | 'custom';
type FormFields = { name: string; email: string; phone: string; interest: string; note: string; garmentType: string; occasion: string; requirements: string };
const emptyFields: FormFields = { name: '', email: '', phone: '', interest: '', note: '', garmentType: '', occasion: '', requirements: '' };

export default function StudioScreen() {
  const [kind, setKind] = useState<RequestKind>('appointment');
  const [fields, setFields] = useState<FormFields>(emptyFields);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [created, setCreated] = useState<StudioRequestResult | null>(null);
  const idempotencyKey = useRef('');

  function update<K extends keyof FormFields>(key: K, value: FormFields[K]) {
    idempotencyKey.current = '';
    setFields((current) => ({ ...current, [key]: value }));
  }

  function selectKind(next: RequestKind) {
    setKind(next);
    idempotencyKey.current = '';
    setError('');
    setCreated(null);
  }

  async function submit() {
    setError('');
    setCreated(null);
    if (!fields.name.trim() || !fields.phone.trim() && !fields.email.trim() || kind === 'custom' && !fields.garmentType.trim()) {
      setError(kind === 'custom' ? 'Add your name, garment type and an email or phone number.' : 'Add your name and an email or phone number.');
      return;
    }
    if (fields.email.trim() && !/^\S+@\S+\.\S+$/.test(fields.email.trim())) {
      setError('Enter a valid email address or clear the email field.');
      return;
    }
    setSubmitting(true);
    if (!idempotencyKey.current) idempotencyKey.current = newStudioRequestKey();
    const requestKey = idempotencyKey.current;
    try {
      const request = kind === 'appointment'
        ? await createAppointment({ name: fields.name.trim(), email: fields.email.trim(), phone: fields.phone.trim(), interest: fields.interest.trim(), note: fields.note.trim() }, requestKey)
        : await createCustomOrderRequest({ name: fields.name.trim(), email: fields.email.trim(), phone: fields.phone.trim(), garmentType: fields.garmentType.trim(), occasion: fields.occasion.trim(), requirements: fields.requirements.trim() }, requestKey);
      setCreated(request);
      idempotencyKey.current = '';
      setFields(emptyFields);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Your request could not be sent. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <BrandHeader />
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <Text style={styles.eyebrow}>THE ARAMYA STUDIO · ANKOLA</Text>
        <Text style={styles.title}>Made with <Text style={styles.italic}>you in mind.</Text></Text>
        <Text style={styles.intro}>Meet the studio for a personal styling appointment, or tell us about a garment you would like us to create.</Text>

        <View style={styles.switcher} accessibilityRole="tablist">
          <Pressable disabled={submitting} accessibilityRole="tab" accessibilityState={{ selected: kind === 'appointment', disabled: submitting }} onPress={() => selectKind('appointment')} style={[styles.tab, kind === 'appointment' && styles.activeTab]}>
            <Text style={[styles.tabText, kind === 'appointment' && styles.activeTabText]}>STUDIO VISIT</Text>
          </Pressable>
          <Pressable disabled={submitting} accessibilityRole="tab" accessibilityState={{ selected: kind === 'custom', disabled: submitting }} onPress={() => selectKind('custom')} style={[styles.tab, kind === 'custom' && styles.activeTab]}>
            <Text style={[styles.tabText, kind === 'custom' && styles.activeTabText]}>CUSTOM GARMENT</Text>
          </Pressable>
        </View>

        <View style={styles.form}>
          <Text style={styles.sectionEyebrow}>{kind === 'appointment' ? 'REQUEST A VISIT' : 'TELL US YOUR VISION'}</Text>
          <Text style={styles.formTitle}>{kind === 'appointment' ? 'Let’s find a time.' : 'Start a conversation.'}</Text>
          <Field label="Your name" value={fields.name} onChangeText={(value) => update('name', value)} autoComplete="name" maxLength={120} editable={!submitting} />
          <Field label="Email address (or phone below)" value={fields.email} onChangeText={(value) => update('email', value)} keyboardType="email-address" autoCapitalize="none" autoComplete="email" maxLength={254} editable={!submitting} />
          <Field label="Phone number (or email above)" value={fields.phone} onChangeText={(value) => update('phone', value)} keyboardType="phone-pad" autoComplete="tel" maxLength={32} editable={!submitting} />
          {kind === 'appointment' ? <>
            <Field label="What would you like to explore?" value={fields.interest} onChangeText={(value) => update('interest', value)} maxLength={120} editable={!submitting} />
            <Field label="Anything we should know?" value={fields.note} onChangeText={(value) => update('note', value)} multiline maxLength={1000} editable={!submitting} />
          </> : <>
            <Field label="Garment type" value={fields.garmentType} onChangeText={(value) => update('garmentType', value)} placeholder="For example, a lehenga or saree" maxLength={120} editable={!submitting} />
            <Field label="Occasion" value={fields.occasion} onChangeText={(value) => update('occasion', value)} maxLength={160} editable={!submitting} />
            <Field label="Describe your design" value={fields.requirements} onChangeText={(value) => update('requirements', value)} multiline maxLength={3000} editable={!submitting} />
            <Text style={styles.note}>The studio will review your request and discuss measurements and a quotation with you. This request does not create an order or charge.</Text>
          </>}
          {error ? <Text accessibilityRole="alert" style={styles.error}>{error}</Text> : null}
          {created ? <View accessibilityLiveRegion="polite" style={styles.success}><Text style={styles.successTitle}>Your request is with the studio.</Text><Text style={styles.successText}>Reference {created.id.slice(0, 8).toUpperCase()} · The team will follow up using your contact details.</Text></View> : null}
          <Pressable accessibilityRole="button" accessibilityState={{ disabled: submitting }} disabled={submitting} onPress={() => void submit()} style={({ pressed }) => [styles.submit, (pressed || submitting) && styles.submitPressed]}>
            {submitting ? <ActivityIndicator color={Brand.white} /> : <Text style={styles.submitText}>{kind === 'appointment' ? 'REQUEST AN APPOINTMENT' : 'SEND CUSTOM REQUEST'}  ↗</Text>}
          </Pressable>
        </View>
        <Text style={styles.address}>ARAMYA FASHION HOUSE  ·  ANKOLA, KARNATAKA</Text>
      </ScrollView>
    </SafeAreaView>
  );
}

type FieldProps = { label: string; value: string; onChangeText: (value: string) => void; multiline?: boolean; placeholder?: string; keyboardType?: 'default' | 'email-address' | 'phone-pad'; autoComplete?: 'name' | 'email' | 'tel'; autoCapitalize?: 'none' | 'sentences'; maxLength: number; editable: boolean };
function Field({ label, multiline, ...inputProps }: FieldProps) {
  return <View style={styles.field}>
    <Text style={styles.label}>{label}</Text>
    <TextInput {...inputProps} accessibilityLabel={label} multiline={multiline} textAlignVertical={multiline ? 'top' : 'center'} style={[styles.input, multiline && styles.multiline]} placeholderTextColor={Brand.muted} />
  </View>;
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper },
  content: { paddingHorizontal: 21, paddingTop: 30, paddingBottom: 42 },
  eyebrow: { color: Brand.red, fontSize: 8, letterSpacing: 1.6, textAlign: 'center' },
  title: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 36, lineHeight: 42, textAlign: 'center', marginTop: 11 },
  italic: { fontStyle: 'italic' },
  intro: { color: Brand.muted, fontSize: 11, lineHeight: 18, textAlign: 'center', marginTop: 12, marginBottom: 23 },
  switcher: { flexDirection: 'row', borderBottomWidth: 1, borderBottomColor: Brand.line },
  tab: { flex: 1, minHeight: 45, alignItems: 'center', justifyContent: 'center', borderBottomWidth: 2, borderBottomColor: 'transparent' },
  activeTab: { borderBottomColor: Brand.red },
  tabText: { color: Brand.muted, fontSize: 8, letterSpacing: 1 },
  activeTabText: { color: Brand.red },
  form: { paddingTop: 24 },
  sectionEyebrow: { color: Brand.red, fontSize: 8, letterSpacing: 1.4 },
  formTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 25, marginTop: 7, marginBottom: 13 },
  field: { marginTop: 13 },
  label: { color: Brand.ink, fontSize: 10, marginBottom: 7 },
  input: { minHeight: 48, borderWidth: 1, borderColor: Brand.line, backgroundColor: Brand.white, paddingHorizontal: 12, color: Brand.ink, fontSize: 12 },
  multiline: { minHeight: 100, paddingTop: 12 },
  note: { color: Brand.muted, fontSize: 9, lineHeight: 16, marginTop: 13 },
  error: { color: Brand.red, backgroundColor: '#F2E5E3', fontSize: 10, lineHeight: 16, padding: 12, marginTop: 16 },
  success: { backgroundColor: '#E8EEE5', padding: 13, marginTop: 16 },
  successTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 17 },
  successText: { color: Brand.muted, fontSize: 9, lineHeight: 15, marginTop: 5 },
  submit: { minHeight: 50, backgroundColor: Brand.red, alignItems: 'center', justifyContent: 'center', marginTop: 18, paddingHorizontal: 14 },
  submitPressed: { opacity: 0.72 },
  submitText: { color: Brand.white, fontSize: 8, letterSpacing: 1 },
  address: { color: Brand.muted, fontSize: 7, letterSpacing: 1, textAlign: 'center', marginTop: 27 },
});
