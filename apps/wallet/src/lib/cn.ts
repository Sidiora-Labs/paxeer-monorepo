import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

const mergeThemeClasses = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [{ text: ['display'] }],
      shadow: [{ shadow: ['card', 'menu', 'sheet', 'popover'] }],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return mergeThemeClasses(clsx(inputs));
}
