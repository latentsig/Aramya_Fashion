import { Image } from 'expo-image';
import { Stack, useLocalSearchParams, useRouter } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Brand } from '@/constants/brand';
import { BrandHeader } from '@/components/BrandHeader';
import { addCartItem, fetchProduct, formatPrice, mediaUrl, type Product } from '@/lib/api';

type ProductDetail = Product & { variants: { id: string; optionName: string; optionValue: string; available: boolean; priceAdjustmentPaise: number }[] };

export default function ProductScreen() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const router = useRouter();
  const [product, setProduct] = useState<ProductDetail | null>(null);
  const [selectedVariant, setSelectedVariant] = useState('');
  const [loadedSlug, setLoadedSlug] = useState('');
  const loading = loadedSlug !== slug;
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    let current = true;
    void fetchProduct(slug).then((detail) => {
      if (current) { setProduct(detail); setSelectedVariant(detail.defaultVariantId); setError(null); }
    }).catch((cause: unknown) => { if (current) setError(cause instanceof Error ? cause.message : 'Could not load this style.'); })
      .finally(() => { if (current) setLoadedSlug(slug); });
    return () => { current = false; };
  }, [slug]);

  const variant = useMemo(() => product?.variants.find((item) => item.id === selectedVariant), [product, selectedVariant]);
  const price = product ? product.pricePaise + (variant?.priceAdjustmentPaise ?? 0) : 0;

  const add = async () => {
    if (!variant || !variant.available) return;
    setAdding(true); setError(null); setNotice(null);
    try {
      await addCartItem(variant.id);
      setNotice('Added to your bag.');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'This style could not be added to your bag.');
    } finally { setAdding(false); }
  };

  if (loading) return <SafeAreaView style={styles.safe}><ActivityIndicator color={Brand.red} style={styles.loader} /><Text style={styles.loadingText}>Finding this style…</Text></SafeAreaView>;
  if (!product) return <SafeAreaView style={styles.safe}><Stack.Screen options={{ headerShown: false }} /><BrandHeader /><View style={styles.message}><Text style={styles.title}>This style isn’t available.</Text><Text style={styles.body}>{error}</Text><Pressable onPress={() => router.back()} style={styles.back}><Text style={styles.backText}>GO BACK</Text></Pressable></View></SafeAreaView>;

  const image = mediaUrl(product.imageKey);
  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <BrandHeader />
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.gallery}>
          {image && <Image source={{ uri: image }} style={StyleSheet.absoluteFill} contentFit="cover" transition={200} />}
          <Pressable accessibilityRole="button" accessibilityLabel="Go back" onPress={() => router.back()} style={styles.backCircle}><Text style={styles.backCircleText}>‹</Text></Pressable>
        </View>
        <View style={styles.details}>
          <Text style={styles.category}>{product.categoryName.toUpperCase()}</Text>
          <Text style={styles.title}>{product.name}</Text>
          <Text style={styles.price}>{formatPrice(price, product.currency)}</Text>
          <Text style={styles.body}>{product.description || 'Thoughtfully made for the moments you will always remember.'}</Text>
          {product.variants.length > 0 && <>
            <Text style={styles.optionLabel}>SELECT {product.variants[0].optionName.toUpperCase()}</Text>
            <View style={styles.options}>{product.variants.map((item) => {
              const selected = item.id === selectedVariant;
              return <Pressable key={item.id} disabled={!item.available} accessibilityRole="radio" accessibilityState={{ checked: selected, disabled: !item.available }} onPress={() => setSelectedVariant(item.id)} style={[styles.option, selected && styles.optionSelected, !item.available && styles.optionDisabled]}><Text style={[styles.optionText, selected && styles.optionTextSelected]}>{item.optionValue}</Text></Pressable>;
            })}</View>
          </>}
          {(error || notice) && <Text accessibilityRole="alert" style={error ? styles.error : styles.success}>{error ?? notice}</Text>}
          <Pressable accessibilityRole="button" disabled={!variant?.available || adding} onPress={() => void add()} style={[styles.addButton, (!variant?.available || adding) && styles.addDisabled]}>
            {adding ? <ActivityIndicator color={Brand.white} /> : <Text style={styles.addText}>{variant?.available ? 'ADD TO BAG' : 'CURRENTLY UNAVAILABLE'}  ·  {formatPrice(price, product.currency)}</Text>}
          </Pressable>
          {notice && <Pressable onPress={() => router.push('/(tabs)/cart')} style={styles.viewBag}><Text style={styles.viewBagText}>VIEW YOUR BAG ↗</Text></Pressable>}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper }, loader: { marginTop: 100 }, loadingText: { marginTop: 12, color: Brand.muted, textAlign: 'center', fontSize: 11 }, content: { paddingBottom: 30 },
  gallery: { height: 480, backgroundColor: '#DED3C7', overflow: 'hidden' }, backCircle: { position: 'absolute', top: 14, left: 14, width: 37, height: 37, borderRadius: 20, backgroundColor: Brand.paper, alignItems: 'center', justifyContent: 'center' }, backCircleText: { color: Brand.ink, fontSize: 27, lineHeight: 30 },
  details: { paddingHorizontal: 22, paddingTop: 22 }, category: { color: Brand.red, fontSize: 8, letterSpacing: 1.4 }, title: { marginTop: 8, color: Brand.ink, fontFamily: 'Georgia', fontSize: 28 }, price: { marginTop: 9, color: Brand.ink, fontSize: 13 },
  body: { marginTop: 14, color: Brand.muted, fontSize: 11, lineHeight: 18 }, optionLabel: { marginTop: 23, color: Brand.ink, fontSize: 8, letterSpacing: 1.2 }, options: { flexDirection: 'row', flexWrap: 'wrap', gap: 9, marginTop: 11 }, option: { minWidth: 44, minHeight: 39, paddingHorizontal: 10, borderWidth: 1, borderColor: Brand.line, alignItems: 'center', justifyContent: 'center' }, optionSelected: { borderColor: Brand.red }, optionDisabled: { opacity: 0.4 }, optionText: { color: Brand.ink, fontSize: 10 }, optionTextSelected: { color: Brand.red },
  addButton: { minHeight: 50, marginTop: 22, backgroundColor: Brand.red, alignItems: 'center', justifyContent: 'center' }, addDisabled: { backgroundColor: Brand.muted }, addText: { color: Brand.white, fontSize: 9, letterSpacing: 1 },
  error: { color: Brand.red, fontSize: 10, marginTop: 14 }, success: { color: '#2E6C48', fontSize: 10, marginTop: 14 }, viewBag: { paddingVertical: 13, alignItems: 'center' }, viewBagText: { color: Brand.red, fontSize: 9, letterSpacing: 1 },
  message: { flex: 1, padding: 26, justifyContent: 'center', alignItems: 'center' }, back: { padding: 15 }, backText: { color: Brand.red, fontSize: 9, letterSpacing: 1 },
});
