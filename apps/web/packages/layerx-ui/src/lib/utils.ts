import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const mergeThemeClasses = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        { text: ["nano", "micro", "label", "body-sm", "body", "title-sm", "title", "display"] },
      ],
      shadow: [{ shadow: ["sm", "card", "overlay"] }],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return mergeThemeClasses(clsx(inputs));
}
