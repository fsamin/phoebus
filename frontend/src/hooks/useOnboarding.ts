import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import { useAuth } from '../contexts/AuthContext';

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
  const { user, loading: authLoading } = useAuth();
  const [seen, setSeen] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);
  const [forced, setForced] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading || !user) {
      setLoading(false);
      return;
    }
    setLoading(true);
    api.getOnboarding()
      .then(setSeen)
      .catch(() => setSeen({}))
      .finally(() => setLoading(false));
  }, [user, authLoading]);

  const shouldRun = useCallback(
    (tour: TourName) => {
      if (loading || authLoading || !user) return false;
      if (forced === tour) return true;
      return !seen[tour];
    },
    [seen, loading, forced, authLoading, user],
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
