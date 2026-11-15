export const motionDurations = {
  stagger: 0.06,
  quick: 0.3,
  fade: 0.4,
  intro: 0.7,
  progress: 0.8,
  scan: 2,
  fast: 0.12,
  normal: 0.2,
  slow: 0.35,
  settle: 0.5,
  shimmer: 1.5,
  splash: 3,
  exit: 0.6,
} as const;

export const motionEase: [number, number, number, number] = [0.2, 0, 0, 1];

export const springMotion = {
  type: 'spring' as const,
  stiffness: 320,
  damping: 28,
};

export const drawerSpringMotion = {
  type: 'spring' as const,
  stiffness: 300,
  damping: 30,
  mass: 0.8,
};
