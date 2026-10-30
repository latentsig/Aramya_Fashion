import { Link } from 'expo-router';
import { Image } from 'expo-image';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Brand } from '@/constants/brand';
import { formatPrice, mediaUrl, type Product } from '@/lib/api';

export function ProductCard({ product, compact = false }: { product: Product; compact?: boolean }) {
  const uri = mediaUrl(product.imageKey);
  return (
    <Link href={`/product/${product.slug}`} asChild>
      <Pressable accessibilityRole="link" accessibilityLabel={`${product.name}, ${formatPrice(product.pricePaise)}`} style={[styles.card, compact && styles.compact]}>
        <View style={[styles.imageFrame, compact && styles.compactImage]}>
          {uri ? <Image source={{ uri }} style={StyleSheet.absoluteFill} contentFit="cover" transition={180} /> : <View style={[StyleSheet.absoluteFill, styles.imageFallback]} />}
          <View style={styles.imageScrim} /><Text style={styles.category}>{product.categoryName}</Text>
        </View>
        <View style={styles.copy}><Text numberOfLines={1} style={styles.name}>{product.name}</Text><Text style={styles.price}>{formatPrice(product.pricePaise)}</Text></View>
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  card: { width: '100%' }, compact: { width: 190 },
  imageFrame: { height: 258, backgroundColor: '#DED3C7', overflow: 'hidden', justifyContent: 'flex-end' }, compactImage: { height: 236 },
  imageFallback: { backgroundColor: '#DED3C7' }, imageScrim: { height: 64, backgroundColor: '#211B1A55' },
  category: { position: 'absolute', left: 12, bottom: 12, color: Brand.white, fontSize: 8, letterSpacing: 1.2, textTransform: 'uppercase' },
  copy: { gap: 5, paddingTop: 10, paddingBottom: 13, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: Brand.line },
  name: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 16 }, price: { color: Brand.muted, fontSize: 11 },
});
