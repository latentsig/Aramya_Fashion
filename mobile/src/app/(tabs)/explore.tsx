import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMemo } from 'react';
import { Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { BrandHeader } from '@/components/BrandHeader';
import { ProductCard } from '@/components/ProductCard';
import { Brand } from '@/constants/brand';
import { useCatalog } from '@/lib/catalog';

export default function CollectionsScreen() {
  const params = useLocalSearchParams<{ category?: string }>();
  const router = useRouter();
  const { categories, products, loading, error, refresh } = useCatalog();
  const activeCategory = typeof params.category === 'string' ? params.category : '';
  const visibleProducts = useMemo(() => products.filter((product) => !activeCategory || product.categorySlug === activeCategory), [products, activeCategory]);
  const categoryTitle = categories.find((category) => category.slug === activeCategory)?.name;

  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <BrandHeader />
      <ScrollView refreshControl={<RefreshControl refreshing={loading} onRefresh={() => void refresh()} tintColor={Brand.red} />} contentContainerStyle={styles.content}>
        <View style={styles.heading}>
          <Text style={styles.eyebrow}>THE ARAMYA COLLECTIONS</Text>
          <Text style={styles.title}>{categoryTitle ?? 'Find your'} <Text style={styles.italic}>{categoryTitle ? 'edit.' : 'occasion.'}</Text></Text>
          <Text style={styles.intro}>Considered occasionwear, imagined with intention and made to feel entirely your own.</Text>
        </View>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.filters}>
          {[{ slug: '', name: 'All styles' }, ...categories].map((category) => {
            const active = category.slug === activeCategory;
            return <Pressable key={category.slug || 'all'} accessibilityRole="button" accessibilityState={{ selected: active }} onPress={() => router.setParams({ category: category.slug || undefined })} style={[styles.filter, active && styles.filterActive]}><Text style={[styles.filterLabel, active && styles.filterLabelActive]}>{category.name}</Text></Pressable>;
          })}
        </ScrollView>
        {error ? <View style={styles.status}><Text style={styles.statusTitle}>Styles are unavailable right now.</Text><Text style={styles.intro}>{error}</Text><Pressable onPress={() => void refresh()}><Text style={styles.retry}>TRY AGAIN</Text></Pressable></View> : (
          <View style={styles.grid}>
            {visibleProducts.map((product) => <View key={product.id} style={styles.card}><ProductCard product={product} /></View>)}
            {!loading && visibleProducts.length === 0 && <View style={styles.empty}><Text style={styles.statusTitle}>No styles in this collection just yet.</Text></View>}
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper }, content: { paddingBottom: 36 },
  heading: { paddingHorizontal: 21, paddingTop: 33, paddingBottom: 20, backgroundColor: Brand.sand, alignItems: 'center' },
  eyebrow: { color: Brand.red, fontSize: 8, fontWeight: '600', letterSpacing: 1.7, marginBottom: 12 },
  title: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 39, textAlign: 'center' }, italic: { fontStyle: 'italic' },
  intro: { color: Brand.muted, fontSize: 11, lineHeight: 18, textAlign: 'center', maxWidth: 310, marginTop: 12 },
  filters: { paddingHorizontal: 17, paddingVertical: 16, gap: 8 }, filter: { minHeight: 34, borderWidth: 1, borderColor: Brand.line, borderRadius: 18, paddingHorizontal: 13, justifyContent: 'center' },
  filterActive: { backgroundColor: Brand.red, borderColor: Brand.red }, filterLabel: { color: Brand.ink, fontSize: 9 }, filterLabelActive: { color: Brand.white },
  grid: { flexDirection: 'row', flexWrap: 'wrap', paddingHorizontal: 12, gap: 10 }, card: { width: '48%', minWidth: 150, flexGrow: 1, flexBasis: '45%' },
  status: { padding: 30, alignItems: 'center' }, statusTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 19, textAlign: 'center' }, retry: { color: Brand.red, fontSize: 9, letterSpacing: 1.2, padding: 14 },
  empty: { flex: 1, padding: 32 },
});
