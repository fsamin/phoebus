import React, { createContext, useContext } from 'react';
import { useOnboarding, type TourName } from '../hooks/useOnboarding';

interface OnboardingContextType {
  seen: Record<string, boolean>;
  loading: boolean;
  shouldRun: (tour: TourName) => boolean;
  markSeen: (tour: TourName) => Promise<void>;
  resetAll: () => Promise<void>;
  forceRun: (tour: TourName) => void;
}

const OnboardingContext = createContext<OnboardingContextType | null>(null);

export const OnboardingProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const state = useOnboarding();
  return <OnboardingContext.Provider value={state}>{children}</OnboardingContext.Provider>;
};

export function useOnboardingContext(): OnboardingContextType {
  const ctx = useContext(OnboardingContext);
  if (!ctx) throw new Error('useOnboardingContext must be used within OnboardingProvider');
  return ctx;
}
