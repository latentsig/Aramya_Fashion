import { Image } from 'expo-image';
import { Link } from 'expo-router';
import { useEvent } from 'expo';
import { useVideoPlayer, VideoView } from 'expo-video';
import { useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Brand } from '@/constants/brand';
import { BrandHeader } from '@/components/BrandHeader';
import { ProductCard } from '@/components/ProductCard';
import { mediaUrl } from '@/lib/api';
import { useCatalog } from '@/lib/catalog';

export default function HomeScreen() {
  const { categories, products, loading, error, refresh } = useCatalog();
  const [imageFailed, setImageFailed] = useState(false);
  const heroImage = mediaUrl('image assests/saree1.jpg');
  const player = useVideoPlayer('https://cdn.pixabay.com/video/2022/10/22/136039-764371293_large.mp4', (videoPlayer) => {
    videoPlayer.loop = true;
    videoPlayer.muted = true;
    videoPlayer.play();
  });
  const { status: videoStatus } = useEvent(player, 'statusChange', { status: player.status });

  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <View style={styles.announcement}><Text style={styles.announcementText}>MADE FOR YOUR MOMENTS · CRAFTED TO BE YOURS</Text></View>
      <BrandHeader />
      <ScrollView refreshControl={<RefreshControl refreshing={loading} onRefresh={() => void refresh()} tintColor={Brand.red} />} contentContainerStyle={styles.content}>
        <View style={styles.hero}>
          <View style={styles.heroCopy}>
            <Text style={styles.eyebrow}>ARAMYA · BY SHUBHA BHARAT</Text>
            <Text style={styles.heroTitle}>Wear the{'\n'}<Text style={styles.italic}>moment.</Text></Text>
            <Text style={styles.heroIntro}>Indian occasionwear, imagined with intention and made to feel entirely your own.</Text>
            <Link href="/(tabs)/explore" asChild><Pressable accessibilityRole="button" style={styles.heroButton}><Text style={styles.heroButtonText}>EXPLORE COLLECTIONS  ↗</Text></Pressable></Link>
          </View>
          <View style={styles.heroImage}>
            {(videoStatus !== 'readyToPlay' || imageFailed) && !imageFailed && heroImage ? <Image source={{ uri: heroImage }} style={StyleSheet.absoluteFill} contentFit="cover" transition={250} onError={() => setImageFailed(true)} /> : null}
            {!imageFailed && !heroImage && <View style={[StyleSheet.absoluteFill, styles.heroImageFallback]} />}
            {videoStatus !== 'error' && <VideoView accessibilityLabel="A model presenting a saree in a fashion clip" player={player} nativeControls={false} contentFit="cover" surfaceType="textureView" style={StyleSheet.absoluteFill} />}
            <Text style={styles.heroImageNote}>A little wonder{'\n'}in every detail.</Text>
          </View>
        </View>
        {!error && <View style={styles.marquee}><Text style={styles.marqueeText}>MADE TO MEASURE     ✳     ROOTED IN CULTURE     ✳     READY FOR YOUR MOMENT</Text></View>}
        <View style={styles.section}>
          <Text style={styles.eyebrowRed}>A HOUSE OF PERSONAL STYLE</Text>
          <Text style={styles.sectionTitle}>Tradition, with{'\n'}<Text style={styles.italic}>room to be you.</Text></Text>
          <Text style={styles.body}>Thoughtful silhouettes and expressive colour, designed for celebrations big and small.</Text>
        </View>
        <View style={styles.sectionHeading}>
          <View><Text style={styles.eyebrowRed}>THE ARAMYA COLLECTIONS</Text><Text style={styles.sectionTitleLeft}>Find your <Text style={styles.italic}>occasion.</Text></Text></View>
          <Link href="/(tabs)/explore" style={styles.viewAll}>VIEW ALL ↗</Link>
        </View>
        {loading && categories.length === 0 ? <ActivityIndicator color={Brand.red} style={styles.loading} /> : error ? <View style={styles.error}><Text style={styles.errorTitle}>We couldn’t load the collection.</Text><Text style={styles.body}>{error}</Text><Pressable onPress={() => void refresh()} style={styles.retry}><Text style={styles.retryText}>TRY AGAIN</Text></Pressable></View> : (
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.categoryRail}>
            {categories.map((category) => {
              const categoryImage = mediaUrl(category.imageKey);
              return <Link key={category.id} href={{ pathname: '/(tabs)/explore', params: { category: category.slug } }} asChild>
                <Pressable accessibilityRole="link" style={styles.categoryCard}>
                  <View style={styles.categoryImage}>{categoryImage && <Image source={{ uri: categoryImage }} style={StyleSheet.absoluteFill} contentFit="cover" transition={160} />}</View>
                  <Text style={styles.categoryName}>{category.name}</Text><Text style={styles.categorySubtitle}>{category.subtitle}</Text>
                </Pressable>
              </Link>;
            })}
          </ScrollView>
        )}
        {products.length > 0 && <>
          <View style={styles.sectionHeading}><View><Text style={styles.eyebrowRed}>THE STUDIO EDIT</Text><Text style={styles.sectionTitleLeft}>Made for <Text style={styles.italic}>your moment.</Text></Text></View></View>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.productRail}>{products.slice(0, 8).map((product) => <ProductCard key={product.id} product={product} compact />)}</ScrollView>
        </>}
        <View style={styles.footer}><Text style={styles.footerBrand}>ARAMYA</Text><Text style={styles.footerLine}>Fashion House · Ankola, Karnataka</Text></View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: Brand.paper },
  announcement: { minHeight: 25, backgroundColor: Brand.red, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 12 },
  announcementText: { color: Brand.white, fontSize: 8, letterSpacing: 1.1 }, content: { paddingBottom: 40 },
  hero: { backgroundColor: Brand.red, paddingHorizontal: 20, paddingTop: 30, paddingBottom: 16 }, heroCopy: { paddingHorizontal: 2, paddingBottom: 24 },
  eyebrow: { color: '#F1DFE0', fontSize: 8, fontWeight: '600', letterSpacing: 1.8, marginBottom: 17 },
  heroTitle: { color: Brand.white, fontFamily: 'Georgia', fontSize: 52, lineHeight: 51, letterSpacing: -2.6 }, italic: { fontStyle: 'italic', fontWeight: '400' },
  heroIntro: { color: '#F1DFE0', fontSize: 12, lineHeight: 20, maxWidth: 255, marginTop: 13, marginBottom: 17 },
  heroButton: { minHeight: 42, alignSelf: 'flex-start', alignItems: 'center', justifyContent: 'center', paddingHorizontal: 15, backgroundColor: Brand.paper },
  heroButtonText: { color: Brand.red, fontSize: 8, letterSpacing: 1 }, heroImage: { height: 395, backgroundColor: '#CDB89F', overflow: 'hidden' }, heroImageFallback: { backgroundColor: '#CDB89F' },
  heroImageNote: { position: 'absolute', right: 14, bottom: 14, color: Brand.white, fontFamily: 'Georgia', fontStyle: 'italic', fontSize: 18, lineHeight: 21, textAlign: 'right', textShadowColor: '#0008', textShadowRadius: 9 },
  marquee: { minHeight: 39, justifyContent: 'center', paddingHorizontal: 14, backgroundColor: Brand.sand }, marqueeText: { color: Brand.muted, fontSize: 8, letterSpacing: 1, textAlign: 'center' },
  section: { paddingHorizontal: 23, paddingVertical: 40, alignItems: 'center' }, eyebrowRed: { color: Brand.red, fontSize: 8, fontWeight: '600', letterSpacing: 1.5, marginBottom: 13 },
  sectionTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 34, lineHeight: 39, textAlign: 'center' }, body: { color: Brand.muted, fontSize: 11, lineHeight: 18, textAlign: 'center', marginTop: 13, maxWidth: 310 },
  sectionHeading: { paddingHorizontal: 22, paddingTop: 20, paddingBottom: 15, flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-end' }, sectionTitleLeft: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 26, lineHeight: 31 }, viewAll: { color: Brand.red, fontSize: 8, letterSpacing: 1, paddingBottom: 5 },
  loading: { padding: 34 }, error: { margin: 22, padding: 20, backgroundColor: Brand.sand, alignItems: 'center' }, errorTitle: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 19 }, retry: { marginTop: 16, padding: 12 }, retryText: { color: Brand.red, fontSize: 9, letterSpacing: 1 },
  categoryRail: { paddingHorizontal: 20, gap: 12, paddingBottom: 18 }, categoryCard: { width: 218 }, categoryImage: { height: 255, backgroundColor: '#DED3C7', overflow: 'hidden' }, categoryName: { color: Brand.ink, fontFamily: 'Georgia', fontSize: 18, marginTop: 9 }, categorySubtitle: { color: Brand.muted, fontSize: 9, marginTop: 3 },
  productRail: { paddingHorizontal: 20, gap: 12, paddingBottom: 20 }, footer: { marginTop: 23, paddingVertical: 27, alignItems: 'center', backgroundColor: Brand.ink }, footerBrand: { color: Brand.paper, fontFamily: 'Georgia', fontSize: 21, letterSpacing: 3 }, footerLine: { color: '#B9AEAA', fontSize: 9, marginTop: 7 },
});
