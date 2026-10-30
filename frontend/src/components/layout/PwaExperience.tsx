import { useEffect, useState } from 'react';
import { registerSW } from 'virtual:pwa-register';

type InstallPrompt = Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
};

export function PwaExperience() {
  const [online, setOnline] = useState(() => navigator.onLine);
  const [installPrompt, setInstallPrompt] = useState<InstallPrompt | null>(null);
  const [updateReady, setUpdateReady] = useState(false);
  const [applyUpdate, setApplyUpdate] = useState<(() => Promise<void>) | null>(null);
  const [offlineReady, setOfflineReady] = useState(false);

  useEffect(() => {
    const updateOnline = () => setOnline(navigator.onLine);
    const captureInstall = (event: Event) => {
      event.preventDefault();
      setInstallPrompt(event as InstallPrompt);
    };
    window.addEventListener('online', updateOnline);
    window.addEventListener('offline', updateOnline);
    window.addEventListener('beforeinstallprompt', captureInstall);
    const updateServiceWorker = registerSW({
      immediate: true,
      onNeedRefresh: () => setUpdateReady(true),
      onOfflineReady: () => setOfflineReady(true),
    });
    setApplyUpdate(() => () => updateServiceWorker(true));
    return () => {
      window.removeEventListener('online', updateOnline);
      window.removeEventListener('offline', updateOnline);
      window.removeEventListener('beforeinstallprompt', captureInstall);
    };
  }, []);

  const install = async () => {
    if (!installPrompt) return;
    await installPrompt.prompt();
    await installPrompt.userChoice;
    setInstallPrompt(null);
  };

  return (
    <div className="pwa-notices" aria-live="polite">
      {!online && <p className="pwa-notice" role="status">You’re offline. You can browse saved styles; bag updates, checkout and payments need a connection.</p>}
      {online && offlineReady && <p className="pwa-notice" role="status">Aramya is ready for offline browsing.</p>}
      {installPrompt && <button className="pwa-notice pwa-action" type="button" onClick={() => void install()}>Install Aramya</button>}
      {updateReady && <button className="pwa-notice pwa-action" type="button" onClick={() => void applyUpdate?.()}>A new version is ready. Refresh</button>}
    </div>
  );
}
