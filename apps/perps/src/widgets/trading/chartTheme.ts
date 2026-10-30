import type { ThemeName } from "../../../public/static/charting_library/charting_library";

const CHART_THEMES = {
  dark: "#18191d",
  light: "#ffffff",
  greenify: "#0a0e18",
  hotline: "#0f0c2c",
  oled: "#000000",
} as const;

export function resolveChartTheme(theme: string | undefined) {
  const name =
    theme && Object.hasOwn(CHART_THEMES, theme)
      ? (theme as keyof typeof CHART_THEMES)
      : "dark";
  const baseTheme: ThemeName = name === "light" ? "light" : "dark";
  return {
    name,
    baseTheme,
    background: CHART_THEMES[name],
    customCssUrl: `../${name}.css`,
  };
}

export function chartThemeTransition(
  previous: string | undefined,
  next: string | undefined,
) {
  const from = resolveChartTheme(previous);
  const to = resolveChartTheme(next);
  if (from.name === to.name) return "none";
  return from.customCssUrl !== to.customCssUrl ? "rebuild" : "runtime";
}

export interface ChartSnapshot {
  selectedPair: string;
  state: object;
}

export function chartStateForPair(
  snapshot: ChartSnapshot | null,
  selectedPair: string,
) {
  return snapshot?.selectedPair === selectedPair ? snapshot.state : undefined;
}
