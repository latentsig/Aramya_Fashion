import { Image } from 'expo-image';
import { Link } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { BrandHeader } from '@/components/BrandHeader';
import { Brand } from '@/constants/brand';
import { getCart, formatPrice, mediaUrl, removeCartItem, setCartItemQuantity, type CartSnapshot } from '@/lib/api';

export default function CartScreen() {
  const [cart, setCart] = useState<CartSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try { setCart(await getCart()); setError(null); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Your bag could not be loaded.'); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => {
    let current = true;
    getCart()
      .then((snapshot) => { if (current) { setCart(snapshot); setError(null); } })
      .catch((cause: unknown) => { if (current) setError(cause instanceof Error ? cause.message : 'Your bag could not be loaded.'); })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, []);

  const changeQuantity = async (itemId: string, quantity: number) => {
    setBusy(true); setError(null);
    try { setCart(quantity < 1 ? await removeCartItem(itemId) : await setCartItemQuantity(itemId, quantity)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Your bag could not be updated.'); }
    finally { setBusy(false); }
  };

  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <BrandHeader />
      <ScrollView refreshControl={<RefreshControl refreshing={loading} onRefresh={() => void refresh()} tintColor={Brand.red} />} contentContainerStyle={styles.content}>
        <Text style={styles.eyebrow}>YOUR ARAMYA EDIT</Text>
        <Text style={styles.title}>Your bag</Text>
        {error && <View style={styles.errorBox}><Text style={styles.error}>{error}</Text><Pressable onPress={() => void refresh()}><Text style={styles.retry}>RETRY</Text></Pressable></View>}
        {loading && !cart ? <ActivityIndicator color={Brand.red} style={styles.loader} /> : cart?.lines.length ? <>
          <Text style={styles.count}>{cart.count} {cart.count === 1 ? 'STYLE' : 'STYLES'}</Text>
          {cart.lines.map((line) => {
            const image = mediaUrl(line.imageKey);
            return <View key={line.id} style={styles.line}>
              <View style={styles.image}>{image && <Image source={{ uri: image }} style={StyleSheet.absoluteFill} contentFit="cover" />}</View>
              <View style={styles.lineCopy}>
                <Text numberOfLines={2} style={styles.productName}>{line.name}</Text>
                <Text style={styles.category}>{line.categoryName}</Text>
                <Text style={styles.price}>{formatPrice(line.unitPricePaise)}</Text>
                <View style={styles.quantityRow}>
                  <Pressable disabled={busy} accessibilityRole="button" accessibilityLabel={`Reduce ${line.name} quantity`} onPress={() => void changeQuantity(line.id, line.quantity - 1)} style={styles.quantityButton}><Text style={styles.quantitySymbol}>−</Text></Pressable>
                  <Text style={styles.quantity}>{line.quantity}</Text>
                  <Pressable disabled={busy} accessibilityRole="button" accessibilityLabel={`Increase ${line.name} quantity`} onPress={() => void changeQuantity(line.id, line.quantity + 1)} style={styles.quantityButton}><Text style={styles.quantitySymbol}>+</Text></Pressable>
                  <Pressable disabled={busy} accessibilityRole="button" accessibilityLabel={`Remove ${line.name}`} onPress={() => void changeQuantity(line.id, 0)} style={styles.remove}><Text style={styles.removeText}>REMOVE</Text></Pressable>
                </View>
              </View>
            </View>;
          })}
          <View style={styles.summary}><Text style={styles.summaryLabel}>SUBTOTAL</Text><Text style={styles.summaryPrice}>{formatPrice(cart.subtotalPaise)}</Text></View>
          <Text style={styles.note}>Delivery and applicable taxes are confirmed at checkout.</Text>
          <Link href="/checkout" asChild><Pressable style={styles.checkoutButton}><Text style={styles.checkoutButtonText}>CONTINUE TO CHECKOUT  ↗</Text></Pressable></Link>
          <Text style={styles.checkoutNote}>Demo checkout saves a pending order. No payment is collected.</Text>
        </> : !error && !loading && <View style={styles.empty}>
          <Text style={styles.emptyTitle}>Your bag is waiting for a little wonder.</Text>
          <Text style={styles.emptyCopy}>Explore the collection and find the style that feels like you.</Text>
          <Link href="/(tabs)/explore" asChild><Pressable style={styles.shopButton}><Text style={styles.shopButtonText}>EXPLORE STYLES  ↗</Text></Pressable></Link>
        </View>}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper }, content: { paddingHorizontal: 21, paddingTop: 29, paddingBottom: 44 },
  eyebrow: { color: Brand.red, fontSize: 8, letterSpacing: 1.6 }, title: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 37, marginTop: 8 },
  count: { color: Brand.muted, fontSize: 8, letterSpacing: 1, marginTop: 25, marginBottom: 5 }, loader: { marginTop: 80 },
  line: { flexDirection: 'row', gap: 14, paddingVertical: 17, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: Brand.line }, image: { width: 96, height: 124, backgroundColor: '#DED3C7', overflow: 'hidden' }, lineCopy: { flex: 1, justifyContent: 'center' },
  productName: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 16 }, category: { color: Brand.muted, fontSize: 8, marginTop: 5 }, price: { color: Brand.ink, fontSize: 10, marginTop: 8 }, quantityRow: { flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 10 },
  quantityButton: { width: 29, height: 29, borderWidth: 1, borderColor: Brand.line, alignItems: 'center', justifyContent: 'center' }, quantitySymbol: { color: Brand.ink, fontSize: 16 }, quantity: { minWidth: 14, color: Brand.ink, textAlign: 'center', fontSize: 10 }, remove: { marginLeft: 'auto', padding: 7 }, removeText: { color: Brand.red, fontSize: 7, letterSpacing: 1 },
  summary: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingTop: 22, marginTop: 4, borderTopWidth: 1, borderTopColor: Brand.line }, summaryLabel: { color: Brand.ink, fontSize: 9, letterSpacing: 1.1 }, summaryPrice: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 21 }, note: { color: Brand.muted, fontSize: 9, marginTop: 10 }, checkoutNote: { color: Brand.red, fontSize: 9, marginTop: 18 },
  empty: { marginTop: 80, paddingHorizontal: 20, alignItems: 'center' }, emptyTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 23, textAlign: 'center' }, emptyCopy: { color: Brand.muted, fontSize: 11, lineHeight: 18, textAlign: 'center', marginTop: 11 },
  shopButton: { marginTop: 25, minHeight: 45, alignSelf: 'stretch', alignItems: 'center', justifyContent: 'center', backgroundColor: Brand.red }, shopButtonText: { color: Brand.white, fontSize: 9, letterSpacing: 1 },
  checkoutButton: { marginTop: 22, minHeight: 48, alignItems: 'center', justifyContent: 'center', backgroundColor: Brand.red }, checkoutButtonText: { color: Brand.white, fontSize: 9, letterSpacing: 1 },
  errorBox: { marginTop: 16, padding: 13, backgroundColor: '#F2E5E3' }, error: { color: Brand.red, fontSize: 10 }, retry: { color: Brand.red, marginTop: 10, fontSize: 8, letterSpacing: 1 },
});