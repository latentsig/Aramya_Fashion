import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Brand } from '@/constants/brand';

export function BrandHeader() {
  return (
    <View style={styles.header}>
      <View style={styles.brand}>
        <Text style={styles.brandName}>ARAMYA</Text>
        <Text style={styles.brandCaption}>FASHION HOUSE · BY SHUBHA BHARAT</Text>
      </View>
      <Link href="/(tabs)/cart" asChild>
        <Pressable accessibilityRole="button" accessibilityLabel="Open your bag" style={({ pressed }) => [styles.bag, pressed && styles.pressed]}>
          <Text style={styles.bagText}>BAG</Text><Text style={styles.bagCount}>↗</Text>
        </Pressable>
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  header: { minHeight: 66, paddingHorizontal: 21, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', backgroundColor: Brand.paper, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: Brand.line },
  brand: { alignItems: 'center' },
  brandName: { color: Brand.red, fontFamily: 'Georgia', fontSize: 21, letterSpacing: 2.5 },
  brandCaption: { color: Brand.muted, fontSize: 7, letterSpacing: 1.15, marginTop: 2 },
  bag: { minWidth: 52, minHeight: 44, alignItems: 'center', justifyContent: 'center', flexDirection: 'row', gap: 5 },
  bagText: { color: Brand.ink, fontSize: 9, letterSpacing: 1.2 },
  bagCount: { width: 18, height: 18, textAlign: 'center', textAlignVertical: 'center', borderWidth: 1, borderColor: Brand.ink, borderRadius: 10, color: Brand.red, fontSize: 10 },
  pressed: { opacity: 0.65 },
});
