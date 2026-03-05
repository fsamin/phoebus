import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';

export type TourName = 'dashboard' | 'catalog';

interface OnboardingState {
  seen: Record<string, boolean>;
  loading: boolean;
  shouldRun: (tour: TourName) => boolean;
  markSeen: (tour: TourName) => Promise<void>;
  resetAll: () => Promise<void>;
  forceRun: (tour: TourName) => void;
}

export function useOnboarding(): OnboardingState {
  const [seen, setSeen] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);
  const [forced, setForced] = useState<string | null>(null);

  useEffect(() => {
    api.getOnboarding()
      .then(setSeen)
      .catch(() => setSeen({}))
      .finally(() => setLoading(false));
  }, []);

  const shouldRun = useCallback(
    (tour: TourName) => {
      if (loading) return false;
      if (forced === tour) return true;
      return !seen[tour];
    },
    [seen, loading, forced],
  );

  const markSeen = useCallback(async (tour: TourName) => {
    setForced(null);
    setSeen((prev) => ({ ...prev, [tour]: true }));
    await api.markOnboardingSeen(tour).catch(() => {});
  }, []);

  const resetAll = useCallback(async () => {
    setSeen({});
    await api.resetOnboarding().catch(() => {});
  }, []);

  const forceRun = useCallback((tour: TourName) => {
    setForced(tour);
  }, []);

  return { seen, loading, shouldRun, markSeen, resetAll, forceRun };
}
