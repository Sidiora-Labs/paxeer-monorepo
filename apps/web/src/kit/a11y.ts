import { createElement, type HTMLAttributes, type ReactNode } from "react";

export const MINIMUM_TEXT_CONTRAST = 4.5;

export const SEMANTIC_CONTRAST_PAIRS = [
  ["--foreground", "--background"],
  ["--foreground", "--surface"],
  ["--foreground-secondary", "--background"],
  ["--foreground-secondary", "--surface"],
  ["--muted-foreground", "--background"],
  ["--muted-foreground", "--surface"],
  ["--faint-foreground", "--background"],
  ["--faint-foreground", "--surface"],
  ["--primary-foreground", "--primary"],
  ["--accent-foreground", "--accent"],
  ["--accent", "--background"],
  ["--accent", "--surface"],
  ["--accent-strong", "--accent-soft"],
  ["--success", "--background"],
  ["--success", "--surface"],
  ["--success", "--success-soft"],
  ["--success-foreground", "--success"],
  ["--destructive", "--background"],
  ["--destructive", "--surface"],
  ["--destructive", "--destructive-soft"],
  ["--destructive-foreground", "--destructive"],
  ["--warning", "--background"],
  ["--warning", "--surface"],
  ["--warning", "--warning-soft"],
  ["--warning-foreground", "--warning"],
] as const;

const ACCESSIBILITY_CONTRACT = Object.freeze({
  minimumTextContrast: MINIMUM_TEXT_CONTRAST,
  semanticContrastPairs: SEMANTIC_CONTRAST_PAIRS,
  minimumTouchTargetCssPixels: 44,
  supportedTextExpansion: 1.5,
});

export function a11y() {
  return ACCESSIBILITY_CONTRACT;
}

export interface ExplicitCurrencyAmountOptions {
  readonly currency: string;
  readonly locale: string;
  readonly decimals?: number;
  readonly signed?: boolean;
}

export interface SemanticContrastResult {
  readonly background: string;
  readonly foreground: string;
  readonly ratio: number;
  readonly scheme: ColorScheme;
}

export type ColorScheme = "dark" | "light";

const COLOR_SCHEMES: readonly ColorScheme[] = ["light", "dark"];

function channelToLinear(value: number): number {
  const channel = value / 255;
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

function hexChannels(color: string): readonly [number, number, number] {
  const compact = color.trim().toLowerCase();
  const expanded = /^#[0-9a-f]{3}$/u.test(compact)
    ? `#${compact.slice(1, 2).repeat(2)}${compact.slice(2, 3).repeat(2)}${compact.slice(3, 4).repeat(2)}`
    : compact;
  if (!/^#[0-9a-f]{6}$/u.test(expanded)) {
    throw new Error(`Expected a hexadecimal color, received ${color}`);
  }
  return [
    Number.parseInt(expanded.slice(1, 3), 16),
    Number.parseInt(expanded.slice(3, 5), 16),
    Number.parseInt(expanded.slice(5, 7), 16),
  ];
}

function luminance(color: string): number {
  const [red, green, blue] = hexChannels(color);
  return (
    channelToLinear(red) * 0.2126 +
    channelToLinear(green) * 0.7152 +
    channelToLinear(blue) * 0.0722
  );
}

export function contrastRatio(foreground: string, background: string): number {
  const foregroundLuminance = luminance(foreground);
  const backgroundLuminance = luminance(background);
  return (
    (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) /
    (Math.min(foregroundLuminance, backgroundLuminance) + 0.05)
  );
}

interface SchemeTokenValues {
  light?: string;
  dark?: string;
}

function schemeForBlocks(blocks: readonly string[]): ColorScheme | undefined {
  const blockText = blocks.join(" ").toLowerCase();
  if (
    blockText.includes("prefers-color-scheme: dark") ||
    blockText.includes('[data-theme="dark"]') ||
    blockText.includes("[data-theme='dark']") ||
    blockText.includes(":root:not([data-theme=\"light\"])") ||
    blockText.includes(":root:not([data-theme='light'])")
  ) {
    return "dark";
  }
  if (
    blockText.includes('[data-theme="light"]') ||
    blockText.includes("[data-theme='light']")
  ) {
    return "light";
  }
  return undefined;
}

function setSchemeValue(values: SchemeTokenValues, scheme: ColorScheme | undefined, value: string) {
  if (scheme === undefined) {
    values.light = value;
    values.dark = value;
  } else {
    values[scheme] = value;
  }
}

function readSchemeTokenValues(stylesheet: string): ReadonlyMap<string, SchemeTokenValues> {
  const values = new Map<string, SchemeTokenValues>();
  const blocks: string[] = [];
  let buffer = "";

  const recordDeclaration = (declaration: string) => {
    const match = declaration.match(/(--[a-z-]+)\s*:\s*(.*?)\s*$/iu);
    const name = match?.[1];
    const rawValue = match?.[2];
    if (name === undefined || rawValue === undefined) {
      return;
    }

    const token = values.get(name) ?? {};
    const lightDark = rawValue.match(
      /^light-dark\(\s*(#[0-9a-f]{3,6})\s*,\s*(#[0-9a-f]{3,6})\s*\)$/iu,
    );
    if (lightDark?.[1] !== undefined && lightDark[2] !== undefined) {
      token.light = lightDark[1];
      token.dark = lightDark[2];
      values.set(name, token);
      return;
    }

    const value = rawValue.match(/^#[0-9a-f]{3,6}$/iu)?.[0];
    if (value !== undefined) {
      setSchemeValue(token, schemeForBlocks(blocks), value);
      values.set(name, token);
    }
  };

  for (const character of stylesheet.replaceAll(/\/\*[\s\S]*?\*\//gu, "")) {
    if (character === "{") {
      blocks.push(buffer.trim());
      buffer = "";
    } else if (character === "}") {
      buffer = "";
      blocks.pop();
    } else if (character === ";") {
      recordDeclaration(buffer);
      buffer = "";
    } else {
      buffer += character;
    }
  }
  recordDeclaration(buffer);
  return values;
}

export function semanticTokenValuesByScheme(
  stylesheet: string,
): ReadonlyMap<ColorScheme, ReadonlyMap<string, string>> {
  const values = new Map<ColorScheme, ReadonlyMap<string, string>>();
  const tokens = readSchemeTokenValues(stylesheet);
  for (const scheme of COLOR_SCHEMES) {
    const schemeValues = new Map<string, string>();
    for (const [name, token] of tokens) {
      const value = token[scheme] ?? token.light ?? token.dark;
      if (value !== undefined) {
        schemeValues.set(name, value);
      }
    }
    values.set(scheme, schemeValues);
  }
  return values;
}

export function semanticTokenValues(stylesheet: string): ReadonlyMap<string, string> {
  return semanticTokenValuesByScheme(stylesheet).get("light") ?? new Map();
}

export function semanticContrastResults(stylesheet: string): readonly SemanticContrastResult[] {
  const valuesByScheme = semanticTokenValuesByScheme(stylesheet);
  return COLOR_SCHEMES.flatMap((scheme) => {
    const tokens = valuesByScheme.get(scheme);
    if (tokens === undefined) {
      throw new Error(`Missing semantic token values for ${scheme} scheme`);
    }
    return SEMANTIC_CONTRAST_PAIRS.map(([foreground, background]) => {
      const foregroundValue = tokens.get(foreground);
      const backgroundValue = tokens.get(background);
      if (foregroundValue === undefined || backgroundValue === undefined) {
        throw new Error(`Missing semantic color token pair ${foreground} on ${background} in ${scheme} scheme`);
      }
      return {
        foreground,
        background,
        ratio: contrastRatio(foregroundValue, backgroundValue),
        scheme,
      };
    });
  });
}

export function assertSemanticTokenContrast(stylesheet: string): void {
  const failure = semanticContrastResults(stylesheet).find(
    ({ ratio }) => ratio < MINIMUM_TEXT_CONTRAST,
  );
  if (failure !== undefined) {
    throw new Error(
      `${failure.foreground} on ${failure.background} in ${failure.scheme} scheme has ${failure.ratio.toFixed(2)}:1 contrast`,
    );
  }
}

export function formatExplicitCurrencyAmount(
  value: number,
  { currency, locale, decimals = 2, signed = true }: ExplicitCurrencyAmountOptions,
): string {
  if (!Number.isFinite(value)) {
    throw new Error("Amounts must be finite numbers");
  }
  if (!/^[A-Z]{3}$/u.test(currency)) {
    throw new Error("Currency must be an explicit uppercase ISO 4217 code");
  }
  const [canonicalLocale] = Intl.getCanonicalLocales(locale);
  if (canonicalLocale === undefined) {
    throw new Error("A locale is required for amount formatting");
  }
  return new Intl.NumberFormat(canonicalLocale, {
    style: "currency",
    currency,
    currencyDisplay: "code",
    signDisplay: signed ? "always" : "auto",
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value);
}

export interface LiveRegionProps extends Omit<HTMLAttributes<HTMLSpanElement>, "role"> {
  readonly children?: ReactNode;
  readonly priority?: "assertive" | "polite";
}

export function LiveRegion({
  children,
  priority = "polite",
  className,
  ...props
}: LiveRegionProps) {
  return createElement(
    "span",
    {
      ...props,
      className: ["sr-only", className].filter(Boolean).join(" "),
      role: priority === "assertive" ? "alert" : "status",
      "aria-live": priority,
      "aria-atomic": true,
    },
    children,
  );
}
