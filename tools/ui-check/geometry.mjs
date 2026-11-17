import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import { measureGeometry } from "./browser.mjs";

const MEASURES = Object.freeze(["left", "top", "width", "height", "fontSize", "lineHeight"]);

function fields(value, names, label) {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).some((key) => !names.includes(key)) || names.some((key) => !(key in value))) {
    throw new Error(`${label} must contain exactly ${names.join(", ")}`);
  }
}

export async function readGeometry(file, optional = false) {
  let source;
  try {
    source = await readFile(file, "utf8");
  } catch (error) {
    if (optional && error.code === "ENOENT") return { version: 1, entries: [] };
    throw error;
  }
  const geometry = JSON.parse(source);
  fields(geometry, ["version", "entries"], "Geometry file");
  if (geometry.version !== 1 || !Array.isArray(geometry.entries)) throw new Error("Geometry file requires version 1 and an entries array");
  const seen = new Set();
  for (const entry of geometry.entries) {
    fields(entry, ["view", "width", "part", "selector", "values"], "Geometry entry");
    if ([entry.view, entry.part, entry.selector].some((value) => typeof value !== "string" || !value.trim()) || !Number.isSafeInteger(entry.width) || entry.width <= 0) {
      throw new Error("Geometry entry requires view, part, selector and a positive whole pixel width");
    }
    fields(entry.values, MEASURES, "Geometry values");
    for (const key of MEASURES) {
      const value = entry.values[key];
      if (key === "lineHeight" && value === "normal") continue;
      if (!Number.isFinite(value) || (["width", "height", "fontSize", "lineHeight"].includes(key) && value < 0)) {
        throw new Error(`Geometry ${key} must be a finite CSS pixel measurement`);
      }
    }
    const key = JSON.stringify([entry.view, entry.width, entry.part]);
    if (seen.has(key)) throw new Error("Geometry entries must have unique view, width and part names");
    seen.add(key);
  }
  return geometry;
}

export async function measureParts(page, parts) {
  const measurements = [];
  for (const part of [...parts].sort((a, b) => compareNames(a.name, b.name))) {
    const measurement = await page.locator(part.selector).evaluateAll(measureGeometry);
    measurements.push({ part: part.name, selector: part.selector, ...measurement });
  }
  return measurements;
}

function compareNames(a, b) {
  return a < b ? -1 : Number(a > b);
}

export function geometryFindings(measured, stored, writing = false) {
  return measured.flatMap((part) => {
    const previous = stored.find((entry) => entry.part === part.part);
    let message;
    if (!part.values) message = `Part must match one visible element; matched ${part.matches}`;
    else if (!writing && !previous) message = "Part is absent from the geometry file";
    else if (!writing && MEASURES.some((key) => {
      const before = previous.values[key];
      const after = part.values[key];
      return typeof before === "number" && typeof after === "number" ? Math.abs(before - after) > 1 : before !== after;
    })) message = "Part differs from the stored geometry by more than one CSS pixel";
    if (!message) return [];
    return [{ kind: "geometry", part: part.part, selector: part.selector, values: { message, stored: previous?.values ?? null, measured: part.values, matches: part.matches } }];
  });
}

export async function writeGeometry(file, renders, previous, selected) {
  const entries = renders.flatMap((render) => render.geometry.map((part) => ({
    view: render.view, width: render.size.width, part: part.part, selector: part.selector, values: part.values,
  })));
  if (selected) entries.push(...previous.entries.filter((entry) => entry.view !== selected));
  entries.sort((a, b) => compareNames(a.view, b.view) || a.width - b.width || compareNames(a.part, b.part));
  await mkdir(dirname(file), { recursive: true });
  await writeFile(file, `${JSON.stringify({ version: 1, entries }, null, 2)}\n`);
}
