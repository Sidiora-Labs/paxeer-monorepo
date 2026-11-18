import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

const mergeThemeClasses = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [
        { text: ['display', ...[8, 9, 10, 11, 13, 15, 22, 32, 45].map((size) => `size-${size}`)] },
      ],
      shadow: [{ shadow: ['card', 'menu', 'sheet', 'popover'] }],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return mergeThemeClasses(clsx(inputs));
}
