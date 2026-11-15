import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

const mergeSurfaceClasses = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [ { text: [ 'caption', 'label', 'body-sm', 'body', 'title-sm', 'title', 'display', 'unit', 'icon-2xs', 'icon-xs', 'icon-sm', 'icon-md' ] } ],
    },
  },
});

export function cn(...inputs: Array<ClassValue>) {
  return mergeSurfaceClasses(clsx(inputs));
}
