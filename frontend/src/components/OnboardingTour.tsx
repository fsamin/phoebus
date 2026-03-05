import React, { useCallback } from 'react';
import Joyride, { type CallBackProps, STATUS, type Step } from 'react-joyride';
import { useOnboardingContext } from '../contexts/OnboardingContext';
import type { TourName } from '../hooks/useOnboarding';
import { useTheme } from '../contexts/ThemeContext';

interface OnboardingTourProps {
  tour: TourName;
  steps: Step[];
}

const OnboardingTour: React.FC<OnboardingTourProps> = ({ tour, steps }) => {
  const { shouldRun, markSeen } = useOnboardingContext();
  const { isDark } = useTheme();
  const run = shouldRun(tour);

  const handleCallback = useCallback(
    (data: CallBackProps) => {
      const { status } = data;
      if (status === STATUS.FINISHED || status === STATUS.SKIPPED) {
        markSeen(tour);
      }
    },
    [tour, markSeen],
  );

  if (!run) return null;

  return (
    <Joyride
      steps={steps}
      run={run}
      continuous
      showSkipButton
      showProgress
      disableOverlayClose
      callback={handleCallback}
      locale={{
        back: 'Back',
        close: 'Close',
        last: 'Done',
        next: 'Next',
        skip: 'Skip tour',
      }}
      styles={{
        options: {
          primaryColor: '#ff7a45',
          zIndex: 10000,
          backgroundColor: isDark ? '#1f1f1f' : '#fff',
          textColor: isDark ? '#e0e0e0' : '#333',
          arrowColor: isDark ? '#1f1f1f' : '#fff',
        },
        spotlight: {
          borderRadius: 8,
        },
        tooltip: {
          borderRadius: 12,
          padding: 20,
        },
        buttonNext: {
          borderRadius: 6,
          padding: '8px 16px',
        },
        buttonBack: {
          color: isDark ? '#aaa' : '#666',
        },
        buttonSkip: {
          color: isDark ? '#888' : '#999',
        },
      }}
    />
  );
};

export default OnboardingTour;
