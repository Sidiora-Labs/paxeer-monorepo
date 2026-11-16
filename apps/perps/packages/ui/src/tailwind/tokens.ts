import plugin from "tailwindcss/plugin";

export const SURFACE_CSS_VARS = {
  "--oui-space-0": "0rem",
  "--oui-space-1": "0.25rem",
  "--oui-space-2": "0.5rem",
  "--oui-space-3": "0.75rem",
  "--oui-space-4": "1rem",
  "--oui-space-5": "1.25rem",
  "--oui-space-6": "1.5rem",
  "--oui-space-8": "2rem",
  "--oui-space-10": "2.5rem",
  "--oui-space-12": "3rem",
  "--oui-space-16": "4rem",
  "--oui-space-20": "5rem",
  "--oui-space-24": "6rem",

  "--oui-font-size-3xs": "clamp(0.625rem, 0.59rem + 0.12vw, 0.6875rem)",
  "--oui-font-size-2xs": "clamp(0.75rem, 0.71rem + 0.14vw, 0.8125rem)",
  "--oui-font-size-xs": "clamp(0.8125rem, 0.77rem + 0.16vw, 0.875rem)",
  "--oui-font-size-sm": "clamp(0.875rem, 0.82rem + 0.2vw, 0.9375rem)",
  "--oui-font-size-base": "clamp(1rem, 0.94rem + 0.22vw, 1.125rem)",
  "--oui-font-size-lg": "clamp(1.125rem, 1.03rem + 0.34vw, 1.25rem)",
  "--oui-font-size-xl": "clamp(1.25rem, 1.1rem + 0.52vw, 1.5rem)",
  "--oui-font-size-2xl": "clamp(1.5rem, 1.28rem + 0.78vw, 1.875rem)",
  "--oui-font-size-3xl": "clamp(1.75rem, 1.42rem + 1.15vw, 2.25rem)",
  "--oui-font-size-4xl": "clamp(1.875rem, 1.48rem + 1.4vw, 2.625rem)",
  "--oui-font-size-5xl": "clamp(2.25rem, 1.72rem + 1.85vw, 3.25rem)",
  "--oui-font-size-6xl": "clamp(3rem, 2.15rem + 3vw, 4.5rem)",

  "--oui-radius-none": "0rem",
  "--oui-radius-sm": "0.125rem",
  "--oui-radius-md": "0.375rem",
  "--oui-radius-lg": "0.5rem",
  "--oui-radius-xl": "0.75rem",
  "--oui-radius-2xl": "1rem",
  "--oui-radius-full": "9999px",

  "--oui-shadow-sm": "0 1px 2px rgb(0 0 0 / 0.12)",
  "--oui-shadow-md": "0 4px 12px rgb(0 0 0 / 0.16)",
  "--oui-shadow-lg": "0 12px 32px rgb(0 0 0 / 0.2)",
  "--oui-shadow-focus": "0 0 0 3px rgb(var(--oui-color-focus-ring) / 0.42)",

  "--oui-duration-fast": "120ms",
  "--oui-duration-normal": "180ms",
  "--oui-duration-slow": "280ms",
  "--oui-duration-gradient": "5s",
  "--oui-duration-reduced-motion": "0.001ms",
  "--oui-ease-standard": "cubic-bezier(0.2, 0, 0, 1)",
  "--oui-ease-emphasized": "cubic-bezier(0.2, 0.8, 0.2, 1)",
  "--oui-focus-ring-width": "2px",
  "--oui-focus-ring-offset": "2px",
  "--oui-border-width": "1px",
  "--oui-scrollbar-size": "0.375rem",
  "--oui-font-family":
    '"Manrope", "PingFang SC", "Noto Sans CJK SC", "Noto Sans", system-ui, sans-serif',
} as const;

export const tokensPlugin = () =>
  plugin(function ({ addBase }) {
    addBase({
      ":root": SURFACE_CSS_VARS,
    });
  });
