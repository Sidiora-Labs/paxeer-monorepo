import { useEffect, useState } from "react";
import { PaxSpinner } from "@/ui/atoms/PaxSpinner";

const SPLASH_SESSION_KEY = "sidiora.splashShown";
const VISIBLE_MS = 3000;
const FADE_MS = 400;

export function startSplashSession(
  storage: Pick<Storage, "getItem" | "setItem">,
  render: (shown: boolean) => void,
  show: (visible: boolean) => void,
): () => void {
  try {
    if (storage.getItem(SPLASH_SESSION_KEY) === "1") return () => {};
    storage.setItem(SPLASH_SESSION_KEY, "1");
  } catch {}
  render(true);
  show(true);
  const fadeTimer = setTimeout(() => show(false), VISIBLE_MS);
  const unmountTimer = setTimeout(() => render(false), VISIBLE_MS + FADE_MS);
  return () => {
    clearTimeout(fadeTimer);
    clearTimeout(unmountTimer);
  };
}

export function SplashScreen() {
  const [shouldRender, setShouldRender] = useState(false);
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    // Storage access stays after hydration, including browsers that deny it.
    return startSplashSession({
      getItem: (key) => window.sessionStorage.getItem(key),
      setItem: (key, value) => window.sessionStorage.setItem(key, value),
    }, setShouldRender, setVisible);
  }, []);

  if (!shouldRender) return null;

  return (
    <div
      aria-hidden="true"
      className="fixed inset-0 z-[100] flex items-center justify-center bg-background"
      style={{
        opacity: visible ? 1 : 0,
        transition: `opacity ${FADE_MS}ms ease-out`,
        pointerEvents: visible ? "auto" : "none",
      }}
    >
      <PaxSpinner
        speed={0.9}
        active={visible}
        style={{
          width: "min(80vw, 520px)",
          height: "min(80vh, 520px)",
        }}
      />
    </div>
  );
}
