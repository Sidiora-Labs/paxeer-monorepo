import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { cn } from "./utils";

const themeSource = readFileSync(
  fileURLToPath(new URL("../app/tokens.css", import.meta.url)),
  "utf8",
);

function themeNames(prefix: string): string[] {
  const theme = themeSource.slice(themeSource.indexOf("@theme inline {"));
  return [...theme.matchAll(new RegExp(`^\\s*--${prefix}-([^: ]+):`, "gm"))]
    .map((match) => match[1])
    .filter((name) => !name.endsWith("--line-height"));
}

const fontSizeNames = themeNames("text");
const shadowNames = themeNames("shadow");

describe("cn", () => {
  it("keeps every theme font size with a text colour in either order", () => {
    for (const name of fontSizeNames) {
      const size = `text-${name}`;
      expect(cn("text-text", size)).toContain(`text-text ${size}`);
      expect(cn(size, "text-text")).toContain(`${size} text-text`);
    }
  });

  it("keeps the last of two font sizes", () => {
    expect(cn("text-2xs", "text-md")).toBe("text-md");
  });

  it("keeps the last of two text colours", () => {
    expect(cn("text-text", "text-text-faint")).toBe("text-text-faint");
  });

  it("keeps every theme shadow with a shadow colour in either order", () => {
    for (const name of shadowNames) {
      const shadow = `shadow-${name}`;
      expect(cn("shadow-text", shadow)).toContain(`shadow-text ${shadow}`);
      expect(cn(shadow, "shadow-text")).toContain(`${shadow} shadow-text`);
    }
  });

  it("keeps numeric cell sizing beside both text colours", () => {
    expect(cn("text-num-sm", "text-text")).toContain("text-num-sm text-text");
    expect(cn("text-text-faint", "text-num-sm")).toContain(
      "text-text-faint text-num-sm",
    );
  });
});
