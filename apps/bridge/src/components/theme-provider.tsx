import * as React from "react";

type Theme = "light" | "dark" | "system";
type Accent = "ink" | "dx-gradient";

type ThemeContextValue = {
  theme: Theme;
  resolvedTheme: "light" | "dark";
  accent: Accent;
  setTheme: (t: Theme) => void;
  setAccent: (a: Accent) => void;
  toggleTheme: () => void;
};

const ThemeContext = React.createContext<ThemeContextValue | null>(null);

function isTheme(value: string | null): value is Theme {
  return value === "light" || value === "dark" || value === "system";
}

function isAccent(value: string | null): value is Accent {
  return value === "ink" || value === "dx-gradient";
}

function readPreference(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch (error) {
    if (error instanceof DOMException && error.name === "SecurityError")
      return null;
    throw error;
  }
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = React.useState<Theme>("system");
  const [accent, setAccentState] = React.useState<Accent>("ink");
  const [system, setSystem] = React.useState<"light" | "dark">("light");
  const [initialized, setInitialized] = React.useState(false);

  React.useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const fn = () => setSystem(mq.matches ? "dark" : "light");
    const storedTheme = readPreference("dx-theme");
    const storedAccent = readPreference("dx-accent");
    setThemeState(isTheme(storedTheme) ? storedTheme : "system");
    setAccentState(isAccent(storedAccent) ? storedAccent : "ink");
    fn();
    setInitialized(true);
    mq.addEventListener("change", fn);
    return () => mq.removeEventListener("change", fn);
  }, []);

  // Keep every same-origin document (e.g. block preview iframes) in sync.
  React.useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === "dx-theme" && isTheme(e.newValue))
        setThemeState(e.newValue);
      if (e.key === "dx-accent" && isAccent(e.newValue))
        setAccentState(e.newValue);
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const resolvedTheme = theme === "system" ? system : theme;

  React.useEffect(() => {
    if (!initialized) return;
    const root = document.documentElement;
    root.classList.toggle("dark", resolvedTheme === "dark");
    root.classList.toggle("accent-dx-gradient", accent === "dx-gradient");
  }, [initialized, resolvedTheme, accent]);

  const setTheme = React.useCallback((t: Theme) => {
    localStorage.setItem("dx-theme", t);
    setThemeState(t);
  }, []);
  const setAccent = React.useCallback((a: Accent) => {
    localStorage.setItem("dx-accent", a);
    setAccentState(a);
  }, []);
  const toggleTheme = React.useCallback(
    () => setTheme(resolvedTheme === "dark" ? "light" : "dark"),
    [resolvedTheme, setTheme],
  );

  return (
    <ThemeContext.Provider
      value={{ theme, resolvedTheme, accent, setTheme, setAccent, toggleTheme }}
    >
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = React.useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}
