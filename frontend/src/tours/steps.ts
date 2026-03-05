import type { Step } from 'react-joyride';

export const dashboardSteps: Step[] = [
  {
    target: '[data-tour="dashboard-welcome"]',
    content: 'Welcome to Phœbus! This is your personal dashboard where you can track your learning progress.',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour="dashboard-continue"]',
    content: 'When you\'re working on a course, you\'ll see a quick-resume card here to jump right back in.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="dashboard-stats"]',
    content: 'These cards show your overall progress: steps completed, exercises attempted, and competencies earned.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="dashboard-paths"]',
    content: 'Here you\'ll find all the learning paths you\'ve started, with progress bars for each one.',
    placement: 'right',
  },
  {
    target: '[data-tour="dashboard-competencies"]',
    content: 'Competencies are skills you unlock by completing learning paths. They also unlock prerequisites for advanced paths.',
    placement: 'right',
  },
  {
    target: '[data-tour="dashboard-activity"]',
    content: 'Your recent activity timeline shows what you\'ve been working on lately.',
    placement: 'left',
  },
];

export const catalogSteps: Step[] = [
  {
    target: '[data-tour="catalog-title"]',
    content: 'The Catalog lists all available learning paths. Let\'s explore the tools to find what you need!',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour="catalog-search"]',
    content: 'Use the search bar to quickly find learning paths by name or description.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="catalog-filters"]',
    content: 'Filter by tags, competencies, or progress status to narrow down your choices.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="catalog-cards"]',
    content: 'Each card represents a learning path. Click on one to see its modules and start learning!',
    placement: 'bottom',
  },
];
