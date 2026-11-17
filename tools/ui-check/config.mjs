import { lstat, readFile, readlink, realpath, stat } from "node:fs/promises";
import { dirname, isAbsolute, join, relative, resolve, sep, win32 } from "node:path";
import { fileURLToPath } from "node:url";

export const LIMITS = Object.freeze({
  touch: 44,
  touchWidth: 834,
  textContrast: 4.5,
  largeContrast: 3,
  controlContrast: 3,
  shift: 0.05,
  motionMs: 100,
  readyMs: 15000,
  navigationMs: 10000,
  renderMs: 300000,
  buildMs: 120000,
  settleMs: 600,
});
export const DEVICES = Object.freeze([
  ...[360, 390, 430, 768, 834, 1024, 1280, 1440, 1920, 2560].map(
    (width) => Object.freeze({ width, height: 900 }),
  ),
  Object.freeze({ width: 844, height: 390 }),
]);
export const REPOSITORY = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

export const MODES = Object.freeze(["default", "reduced-motion", "double-text", "keyboard"]);
export const FINDING_KINDS = Object.freeze(["load", "sideways", "cut", "contrast", "touch", "focus", "shift", "motion", "scale", "geometry"]);
export const GEOMETRY_WIDTHS = Object.freeze([390, 1280, 1440, 1920, 2560]);

export function renderMatrix(modes = MODES) {
  return DEVICES.flatMap((size) =>
    ["light", "dark"].flatMap((scheme) =>
      ["default", ...([390, 1440].includes(size.width)
        ? ["reduced-motion", "double-text", "keyboard"] : [])].map(
        (mode) => ({ ...size, scheme, mode }),
      ),
    ),
  ).filter((configuration) => modes.includes(configuration.mode));
}

function object(value, keys, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  for (const key of Object.keys(value)) {
    if (!keys.includes(key)) throw new Error(`${label}: unknown field ${key}`);
  }
}
function text(value, label) {
  if (typeof value !== "string" || !value.trim()) throw new Error(`${label} must be a nonempty string`);
}
function array(value, label, nonempty = false) {
  if (!Array.isArray(value) || (nonempty && !value.length)) throw new Error(`${label} must be an array${nonempty ? " with entries" : ""}`);
}
function localPath(value, label) {
  text(value, label);
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\")) {
    throw new Error(`${label} must be an origin-relative path`);
  }
}
function uniqueNames(values, label) {
  const names = values.map((value) => value.name);
  if (new Set(names).size !== names.length) throw new Error(`${label} names must be unique`);
}
export function within(parent, child) {
  const rel = relative(parent, child);
  return rel === "" || (!rel.startsWith(`..${sep}`) && rel !== ".." && !isAbsolute(rel));
}
async function canonicalDestination(path) {
  try {
    return await realpath(path);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    try {
      if ((await lstat(path)).isSymbolicLink()) {
        return canonicalDestination(resolve(dirname(path), await readlink(path)));
      }
    } catch (entryError) {
      if (entryError.code !== "ENOENT") throw entryError;
    }
    const parent = dirname(path);
    if (parent === path) throw error;
    return join(await canonicalDestination(parent), path.slice(parent.length + (parent.endsWith(sep) ? 0 : 1)));
  }
}

export async function surfaceOutput(directory, path, name) {
  const label = `Surface ${name} outputs`;
  text(path, label);
  const destination = resolve(directory, path);
  if (isAbsolute(path) || win32.isAbsolute(path) || !within(directory, destination) || destination === directory) {
    throw new Error(`${label} must name relative paths inside the surface directory, not the directory itself`);
  }
  if (path.split(/[\\/]/).includes("node_modules")) {
    throw new Error(`${label} must not contain node_modules`);
  }
  let canonical;
  try {
    canonical = await canonicalDestination(destination);
  } catch (error) {
    throw new Error(`${label}: ${error.message}`, { cause: error });
  }
  if (!within(directory, canonical) || canonical === directory) {
    throw new Error(`${label} must stay inside the surface directory after resolving links, not name the directory itself`);
  }
  if (relative(directory, canonical).split(sep).includes("node_modules")) {
    throw new Error(`${label} must not resolve into node_modules`);
  }
  return destination;
}
export async function outputDirectory(path) {
  const destination = await canonicalDestination(resolve(path));
  if (within(await realpath(REPOSITORY), destination)) {
    throw new Error("Output directory must be outside the repository");
  }
  return destination;
}

export function addressPattern(pattern, label = "answer.pattern") {
  text(pattern, label);
  if (!/^https?:\/\//.test(pattern)) throw new Error(`${label} must be an HTTP address pattern`);
  return new RegExp(`^${pattern.split("*").map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join(".*")}$`);
}

function expectations(value, label) {
  if (value === undefined) return [];
  array(value, label);
  return value.map((entry) => {
    object(entry, ["method", "pattern", "status"], `${label} entry`);
    text(entry.method, `${label}.method`);
    if (!/^[A-Z]+$/.test(entry.method)) throw new Error(`${label}.method must be uppercase`);
    const matcher = addressPattern(entry.pattern);
    if (!Number.isInteger(entry.status) || entry.status < 400 || entry.status > 599) {
      throw new Error(`${label}.status must be an HTTP status from 400 to 599`);
    }
    return { ...entry, matcher };
  });
}

function readModes(value, inherited, label) {
  if (value === undefined) return [...inherited];
  array(value, label, true);
  if (!value.includes("default") || new Set(value).size !== value.length || value.some((mode) => !MODES.includes(mode))) {
    throw new Error(`${label} must contain default and unique names from ${MODES.join(", ")}`);
  }
  return MODES.filter((mode) => value.includes(mode));
}

async function readAnswers(value, label, base, required = false) {
  if (value === undefined) {
    if (required) array(value, label);
    return [];
  }
  array(value, label);
  const answers = [];
  for (const answer of value) {
    const fieldLabel = label === "surface.answers" ? "answer" : `${label} entry`;
    object(answer, ["method", "pattern", "file", "status", "headers"], fieldLabel);
    text(answer.method, `${fieldLabel}.method`);
    if (!/^[A-Z]+$/.test(answer.method)) throw new Error(`${fieldLabel}.method must be uppercase`);
    const pattern = addressPattern(answer.pattern, `${fieldLabel}.pattern`);
    text(answer.file, `${fieldLabel}.file`);
    const path = await realpath(resolve(base, answer.file));
    if (!within(base, path)) {
      const message = "Answer files must be beside the surface file or in its subdirectories";
      throw new Error(label === "surface.answers" ? message : `${label}: ${message}`);
    }
    if (answer.status !== undefined && (!Number.isInteger(answer.status) || answer.status < 200 || answer.status > 599)) {
      throw new Error(`${fieldLabel}.status must be an HTTP status from 200 to 599`);
    }
    if (answer.headers !== undefined) {
      if (!answer.headers || typeof answer.headers !== "object" || Array.isArray(answer.headers) || Object.values(answer.headers).some((v) => typeof v !== "string")) {
        throw new Error(`${fieldLabel}.headers must contain string values`);
      }
    }
    answers.push({ ...answer, pattern, body: await readFile(path) });
  }
  return answers;
}

export async function readSurface(filename) {
  const file = await realpath(resolve(filename));
  const base = dirname(file);
  const value = JSON.parse(await readFile(file, "utf8"));
  object(value, ["name", "directory", "build", "buildMs", "outputs", "start", "address", "readyPath", "views", "answers", "typeScale", "geometry", "reportedKinds"], "surface");
  text(value.name, "surface.name");
  if (value.reportedKinds !== undefined) {
    array(value.reportedKinds, "surface.reportedKinds");
    if (new Set(value.reportedKinds).size !== value.reportedKinds.length || value.reportedKinds.some((kind) => !FINDING_KINDS.includes(kind) || ["load", "geometry"].includes(kind))) {
      throw new Error("surface.reportedKinds must contain unique known kinds other than load and geometry");
    }
  }
  if (value.geometry !== undefined) {
    object(value.geometry, ["file", "widths"], "surface.geometry");
    text(value.geometry.file, "surface.geometry.file");
    if (isAbsolute(value.geometry.file) || win32.isAbsolute(value.geometry.file)) {
      throw new Error("surface.geometry.file must be relative to the surface file");
    }
    const destination = await canonicalDestination(resolve(base, value.geometry.file));
    if (destination === file) throw new Error("surface.geometry.file must differ from the surface file");
    const widths = value.geometry.widths === undefined ? [...GEOMETRY_WIDTHS] : value.geometry.widths;
    array(widths, "surface.geometry.widths", true);
    if (new Set(widths).size !== widths.length || widths.some((width) => !Number.isSafeInteger(width) || width <= 0)) {
      throw new Error("surface.geometry.widths must contain unique positive whole pixel widths");
    }
    value.geometry = { file: destination, widths: [...widths].sort((a, b) => a - b) };
  }
  text(value.directory, "surface.directory");
  const directory = await realpath(resolve(base, value.directory));
  if (!(await stat(directory)).isDirectory()) throw new Error("surface.directory must be a directory");
  for (const key of ["build", "start"]) {
    if (value[key] !== undefined) text(value[key], `surface.${key}`);
  }
  if (value.buildMs !== undefined && (!Number.isInteger(value.buildMs) || value.buildMs < 1000 || value.buildMs > 1800000)) {
    throw new Error(`Surface ${value.name} buildMs must be a whole number from 1000 to 1800000`);
  }
  if (value.outputs !== undefined) {
    const label = `Surface ${value.name} outputs`;
    if (!value.build) throw new Error(`${label} requires a build command`);
    if (!Array.isArray(value.outputs) || value.outputs.length < 1 || value.outputs.length > 16) {
      throw new Error(`${label} must be a list of one to sixteen paths`);
    }
    for (const path of value.outputs) await surfaceOutput(directory, path, value.name);
  }
  text(value.address, "surface.address");
  const address = new URL(value.address);
  if (!["http:", "https:"].includes(address.protocol) || address.username || address.password || address.pathname !== "/" || address.search || address.hash) {
    throw new Error("surface.address must be an HTTP origin");
  }
  if (!value.start && (address.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(address.hostname))) {
    throw new Error("The file server requires a loopback HTTP address");
  }
  localPath(value.readyPath, "surface.readyPath");
  array(value.views, "surface.views", true);
  for (const view of value.views) {
    object(view, ["name", "path", "states", "expects", "answers", "modes", "parts"], "view");
    text(view.name, "view.name");
    if (view.parts !== undefined) {
      array(view.parts, `View ${view.name} parts`);
      for (const part of view.parts) {
        object(part, ["name", "selector"], `View ${view.name} part`);
        text(part.name, `View ${view.name} part.name`);
        text(part.selector, `View ${view.name} part.selector`);
      }
      uniqueNames(view.parts, `View ${view.name} part`);
    }
    localPath(view.path, "view.path");
    view.modes = readModes(view.modes, MODES, `View ${view.name} modes`);
    view.expects = expectations(view.expects, "view.expects");
    view.answers = await readAnswers(view.answers, `View ${view.name} answers`, base);
    if (view.states !== undefined) {
      array(view.states, "view.states");
      for (const state of view.states) {
        object(state, ["name", "steps", "expects", "answers", "modes"], "state");
        text(state.name, "state.name");
        state.modes = readModes(state.modes, view.modes, `View ${view.name} state ${state.name} modes`);
        if (state.name === "default") throw new Error("State name default is reserved");
        state.expects = expectations(state.expects, "state.expects");
        state.answers = await readAnswers(state.answers, `View ${view.name} state ${state.name} answers`, base);
        array(state.steps, "state.steps", true);
        for (const step of state.steps) {
          const keys = {
            click: ["action", "selector"], fill: ["action", "selector", "value"],
            press: ["action", "selector", "key"], wait: ["action", "text"],
          }[step?.action];
          if (!keys) throw new Error("State action must be click, fill, press or wait");
          object(step, keys, "step");
          for (const key of keys.filter((key) => key !== "action")) {
            if (key === "value" && step[key] === "") continue;
            text(step[key], `step.${key}`);
          }
        }
      }
      uniqueNames(view.states, "State");
    }
  }
  uniqueNames(value.views, "View");
  array(value.typeScale, "surface.typeScale", true);
  for (const step of value.typeScale) {
    if (typeof step === "number" && Number.isFinite(step) && step > 0) continue;
    object(step, ["min", "max"], "typeScale step");
    if (!Number.isFinite(step.min) || !Number.isFinite(step.max) || step.min <= 0 || step.max < step.min) {
      throw new Error("A fluid type step requires positive min and max CSS pixels");
    }
  }
  const answers = await readAnswers(value.answers, "surface.answers", base, true);
  return { ...value, directory, address: address.origin, answers };
}
