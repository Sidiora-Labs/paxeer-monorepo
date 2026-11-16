import plugin from "tailwindcss/plugin";
import { SURFACE_CSS_VARS } from "./tokens";

export const basePlugin = () =>
  plugin(function ({ addComponents, addBase }) {
    addBase({
      html: {
        backgroundColor: "rgb(var(--oui-color-base-10))",
        color: "rgb(var(--oui-color-base-foreground) / 0.98)",
        fontFamily: "var(--oui-font-family)",
        colorScheme: "dark",
        textSizeAdjust: "100%",
      },
      body: {
        backgroundColor: "rgb(var(--oui-color-base-10))",
        color: "rgb(var(--oui-color-base-foreground) / 0.98)",
        fontFamily: "var(--oui-font-family)",
        fontWeight: "500",
        fontFeatureSettings: '"ss02" 1, "ss03" 1, "ss05" 1, "ss06" 1',
        fontVariantNumeric: "tabular-nums",
      },
      "h1, h2, h3, h4, h5, h6": {
        textWrap: "balance",
      },
      "p, li, dd": {
        textWrap: "pretty",
      },
      "h1, h2, h3, h4, h5, h6, [role='heading']": {
        fontWeight: "600",
      },
      // Zero-specificity so explicit font-weight utilities still win.
      ":where(.oui-text-2xl, .oui-text-3xl, .oui-text-4xl, .oui-text-5xl)": {
        fontWeight: "600",
      },
      ":where(.oui-text-2xs, .oui-text-xs, .oui-text-sm)": {
        fontWeight: "500",
      },
      "*:focus-visible": {
        outline:
          "var(--oui-focus-ring-width) solid rgb(var(--oui-color-focus-ring) / 0.9)",
        outlineOffset: "var(--oui-focus-ring-offset)",
      },
      "::selection": {
        backgroundColor: "rgb(var(--oui-color-primary) / 0.28)",
        color: "rgb(var(--oui-color-base-foreground) / 0.98)",
      },
      "*, *::before, *::after": {
        boxSizing: "border-box",
      },
      "[data-oui-theme='light']": {
        ...SURFACE_CSS_VARS,
        colorScheme: "light",
      },
    });
  });
