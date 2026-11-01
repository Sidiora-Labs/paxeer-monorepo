import { createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { ThemeProvider, useTheme } from "next-themes";
import { SettingsModal } from "../src/app/modals/SettingsModal";

const options = [
  ["greenify", "Paxify"],
  ["dark", "Dark"],
  ["hotline", "Hotline"],
  ["oled", "OLED"],
] as const;
const themes = ["light", ...options.map(([value]) => value)];
const pause = () => new Promise<void>((resolve) => setTimeout(resolve, 50));
let checks = 0;

function assert(condition: unknown, message: string): asserts condition {
  if (!condition) throw new Error(message);
  checks += 1;
}

function ThemeControls() {
  const { setTheme } = useTheme();
  return createElement(
    "div",
    null,
    ...["system", "light"].map((value) =>
      createElement(
        "button",
        { id: value, key: value, onClick: () => setTheme(value) },
        value,
      ),
    ),
  );
}

function cards() {
  return Array.from(
    document.querySelectorAll<HTMLButtonElement>("button[aria-pressed]"),
  );
}

function assertSelection(value: string, storedValue = value) {
  const buttons = cards();
  assert(buttons.length === options.length, "all four theme cards render");
  for (const [index, [theme, label]] of options.entries()) {
    const button = buttons[index];
    const selected = theme === value;
    assert(button.textContent?.includes(label), `card ${index} is ${label}`);
    assert(
      button.getAttribute("aria-pressed") === String(selected),
      `${label}: selected state`,
    );
    assert(
      button.classList.contains("!border-accent") === selected,
      `${label}: selection border`,
    );
    assert(
      button.classList.contains("ring-2") === selected,
      `${label}: selection ring`,
    );
    assert(
      Boolean(button.querySelector(".lucide-check")) === selected,
      `${label}: check mark`,
    );
  }
  assert(
    document.documentElement.classList.contains(value),
    `${value}: applied theme`,
  );
  assert(
    localStorage.getItem("theme") === storedValue,
    `${storedValue}: persisted theme`,
  );
}

async function run() {
  let root: Root | undefined;
  let container: HTMLDivElement | undefined;
  const unmount = () => {
    root?.unmount();
    container?.remove();
    root = undefined;
  };
  const render = (isOpen: boolean) => {
    root!.render(
      createElement(
        ThemeProvider,
        {
          attribute: "class",
          defaultTheme: "dark",
          enableSystem: true,
          themes,
        },
        createElement(ThemeControls),
        createElement(SettingsModal, { isOpen, onClose: () => render(false) }),
      ),
    );
  };
  const mount = async () => {
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    render(true);
    await pause();
  };

  try {
    for (const [value] of options) {
      localStorage.setItem("theme", value);
      await mount();
      assertSelection(value);
      unmount();
    }

    localStorage.setItem("theme", "dark");
    await mount();
    for (const [index, [value]] of options.entries()) {
      cards()[index].click();
      await pause();
      assertSelection(value);
      cards()[index].click();
      await pause();
      assertSelection(value);
    }

    const close = document.querySelector(".lucide-x")?.closest("button");
    assert(close, "Settings close button renders");
    close.click();
    for (let attempt = 0; attempt < 40 && cards().length > 0; attempt += 1)
      await pause();
    assert(cards().length === 0, "closing removes the theme cards");
    render(true);
    await pause();
    assertSelection("oled");
    unmount();
    await mount();
    assertSelection("oled");

    document.getElementById("light")!.click();
    await pause();
    assertSelection("light");
    document.getElementById("system")!.click();
    await pause();
    const systemTheme = window.matchMedia("(prefers-color-scheme: dark)")
      .matches
      ? "dark"
      : "light";
    assertSelection(systemTheme, "system");
    unmount();
    await mount();
    assertSelection(systemTheme, "system");
  } finally {
    unmount();
  }
  return checks;
}

void run().then(
  (count) => {
    const result = document.createElement("pre");
    result.id = "settings-theme-result";
    result.textContent = `PASS ${count}`;
    document.body.append(result);
  },
  (error: unknown) => {
    const result = document.createElement("pre");
    result.id = "settings-theme-result";
    result.textContent = `FAIL ${String(error)}`;
    document.body.append(result);
  },
);
