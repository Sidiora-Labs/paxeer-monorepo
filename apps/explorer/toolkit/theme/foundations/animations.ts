export const keyframes = {
  fromLeftToRight: {
    from: {
      left: '0%',
      transform: 'translateX(0%)',
    },
    to: {
      left: '100%',
      transform: 'translateX(-100%)',
    },
  },
  skeletonShimmer: {
    from: {
      backgroundPosition: '100% 0',
    },
    to: {
      backgroundPosition: '0% 0',
    },
  },
};
