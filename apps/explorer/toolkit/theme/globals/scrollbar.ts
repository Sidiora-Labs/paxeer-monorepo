const scrollbar = {
  'body *::-webkit-scrollbar': {
    width: 'var(--chakra-spacing-5)',
  },
  'body *::-webkit-scrollbar-track': {
    backgroundColor: 'transparent',
  },
  'body *::-webkit-scrollbar-thumb': {
    backgroundColor: '{colors.global.scrollbar.thumb}',
    borderRadius: 'var(--chakra-radii-scrollbar)',
    border: 'var(--chakra-spacing-2) solid transparent',
    backgroundClip: 'content-box',
    minHeight: 'var(--chakra-spacing-8)',
  },
  'body *::-webkit-scrollbar-button': {
    display: 'none',
  },
  'body *::-webkit-scrollbar-corner': {
    backgroundColor: 'transparent',
  },
  'body *::-webkit-resizer': {
    // FIXME for dark mode we need to use a different image - /static/resizer_dark.png
    backgroundImage: 'url(/explorer/static/resizer_light.png)',
    backgroundSize: 'var(--chakra-spacing-5)',
  },
  'body *': {
    scrollbarWidth: 'thin',
    scrollbarColor: `{colors.global.scrollbar.thumb} transparent`,
  },
};

export default scrollbar;
