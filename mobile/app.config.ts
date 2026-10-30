import type { ExpoConfig, ConfigContext } from 'expo/config';

export default ({ config }: ConfigContext): ExpoConfig => {
  const apiBase = process.env.EXPO_PUBLIC_API_BASE_URL?.trim() ?? '';
  const productionBuild = process.env.EAS_BUILD_PROFILE === 'production';
  const isCleartextLocalDev = apiBase.startsWith('http://');

  if (productionBuild && (!apiBase.startsWith('https://') || isCleartextLocalDev)) {
    throw new Error('Production mobile builds require EXPO_PUBLIC_API_BASE_URL to use HTTPS.');
  }

  return {
    ...config,
    name: config.name ?? 'Aramya Fashion House',
    slug: config.slug ?? 'aramya-fashion-house',
    version: config.version ?? '0.1.0',
    plugins: [
      ...(config.plugins ?? []),
      ['expo-build-properties', { android: { usesCleartextTraffic: isCleartextLocalDev && !productionBuild } }],
      'expo-video',
    ],
  };
};
