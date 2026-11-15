import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const mergeSurfaceClasses = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        { text: ["2xs", "md", "num-sm", "num-md", "display"] },
      ],
      shadow: [{ shadow: ["pop", "modal"] }],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return mergeSurfaceClasses(clsx(inputs));
}
