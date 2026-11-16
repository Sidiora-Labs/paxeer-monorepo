import {
  FC,
  PropsWithChildren,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";
import { useLocalStorage } from "@paxeer-network/hooks";
import {
  PaxeerNetworkThemeProvider,
  type PaxeerNetworkThemeProviderProps,
  DARK_THEME_CSS_VARS,
  LIGHT_THEME_CSS_VARS,
  type ThemeCssVars,
} from "@paxeer-network/ui";

export type AppThemeProviderProps =
  PropsWithChildren<PaxeerNetworkThemeProviderProps>;

export const PAXEER_NETWORK_THEME_STORAGE_KEY = "paxeerNetwork_theme_id";

const useIsomorphicLayoutEffect =
  typeof window === "undefined" ? useEffect : useLayoutEffect;

export const AppThemeProvider: FC<AppThemeProviderProps> = (props) => {
  const { children, themes, ...rest } = props;
  const [systemMode, setSystemMode] = useState<"light" | "dark">("dark");
  useIsomorphicLayoutEffect(() => {
    if (!themes?.some((theme) => !theme.mode)) return;
    const preference = window.matchMedia("(prefers-color-scheme: light)");
    const update = () => setSystemMode(preference.matches ? "light" : "dark");
    update();
    preference.addEventListener("change", update);
    return () => preference.removeEventListener("change", update);
  }, [themes]);

  const defaultTheme = useMemo(() => {
    return themes?.find((theme) => theme.isDefault) ?? themes?.[0];
  }, [themes]);

  const [storedThemeId, setStoredThemeId] = useLocalStorage<string | undefined>(
    PAXEER_NETWORK_THEME_STORAGE_KEY,
    undefined,
  );

  const currentTheme = useMemo(() => {
    const selected =
      themes?.find((theme) => theme.id === storedThemeId) ?? defaultTheme;
    if (!selected || selected.mode) return selected;
    if (systemMode === "dark") return { ...selected, mode: systemMode };
    const brand = selected.cssVars?.["--oui-color-primary"]
      ?.trim()
      .split(/\s+/)
      .map(Number);
    const neutralBrand =
      brand?.length === 3 &&
      brand.every(
        (channel) => Number.isFinite(channel) && channel === brand[0],
      );
    const foreground = LIGHT_THEME_CSS_VARS["--oui-color-base-foreground"];
    return {
      ...selected,
      mode: systemMode,
      cssVars: {
        ...LIGHT_THEME_CSS_VARS,
        "--oui-font-family":
          selected.cssVars?.["--oui-font-family"] ??
          LIGHT_THEME_CSS_VARS["--oui-font-family"],
        ...(neutralBrand
          ? {
              "--oui-color-primary": foreground,
              "--oui-color-primary-light": foreground,
              "--oui-color-primary-darken": foreground,
              "--oui-color-primary-contrast":
                LIGHT_THEME_CSS_VARS["--oui-color-surface"],
              "--oui-color-secondary": foreground,
              "--oui-color-link": foreground,
              "--oui-color-link-light": foreground,
              "--oui-gradient-brand-start": foreground,
              "--oui-gradient-brand-end": foreground,
              "--oui-gradient-primary-start": foreground,
              "--oui-gradient-primary-end": foreground,
            }
          : {}),
      },
    };
  }, [themes, storedThemeId, defaultTheme, systemMode]);

  const currentThemeId = currentTheme?.id;

  // Persist the fallback when the stored theme no longer exists.
  useEffect(() => {
    if (!currentThemeId || currentThemeId === storedThemeId) {
      return;
    }

    setStoredThemeId(currentThemeId);
  }, [currentThemeId, storedThemeId, setStoredThemeId]);

  // Apply theme to DOM via data-oui-theme and optional cssVars.
  useIsomorphicLayoutEffect(() => {
    if (typeof document === "undefined") return;

    const root = document.documentElement;

    if (!currentThemeId) {
      root.removeAttribute("data-oui-theme");
      root.style.removeProperty("color-scheme");
      return;
    }

    root.setAttribute("data-oui-theme", currentThemeId);
    root.style.colorScheme = currentTheme?.mode ?? "dark";

    const baseThemeVars =
      currentTheme?.mode === "light"
        ? LIGHT_THEME_CSS_VARS
        : DARK_THEME_CSS_VARS;

    // override default theme css vars with current theme css vars
    Object.entries(baseThemeVars).forEach(([key, defaultValue]) => {
      const newValue =
        currentTheme?.cssVars?.[key as keyof ThemeCssVars] ?? defaultValue;
      root.style.setProperty(key, newValue);
    });
    const themeColor = document.querySelector<HTMLMetaElement>(
      'meta[name="theme-color"]',
    );
    if (themeColor)
      themeColor.content = getComputedStyle(root)
        .getPropertyValue("--oui-color-canvas")
        .trim()
        .replace(/^(.+)$/, "rgb($1)");
  }, [currentThemeId, currentTheme]);

  return (
    <PaxeerNetworkThemeProvider
      themes={themes}
      currentThemeId={currentThemeId}
      currentTheme={currentTheme}
      setCurrentThemeId={setStoredThemeId}
      {...rest}
    >
      {children}
    </PaxeerNetworkThemeProvider>
  );
};
