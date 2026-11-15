import { readFile, realpath, stat } from "node:fs/promises";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
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

export function renderMatrix() {
  return DEVICES.flatMap((size) =>
    ["light", "dark"].flatMap((scheme) =>
      ["default", ...([390, 1440].includes(size.width)
        ? ["reduced-motion", "double-text", "keyboard"] : [])].map(
        (mode) => ({ ...size, scheme, mode }),
      ),
    ),
  );
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
    const parent = dirname(path);
    if (parent === path) throw error;
    return join(await canonicalDestination(parent), path.slice(parent.length + (parent.endsWith(sep) ? 0 : 1)));
  }
}
export async function outputDirectory(path) {
  const destination = await canonicalDestination(resolve(path));
  if (within(await realpath(REPOSITORY), destination)) {
    throw new Error("Output directory must be outside the repository");
  }
  return destination;
}

export function addressPattern(pattern) {
  text(pattern, "answer.pattern");
  if (!/^https?:\/\//.test(pattern)) throw new Error("answer.pattern must be an HTTP address pattern");
  return new RegExp(`^${pattern.split("*").map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join(".*")}$`);
}

export async function readSurface(filename) {
  const file = await realpath(resolve(filename));
  const base = dirname(file);
  const value = JSON.parse(await readFile(file, "utf8"));
  object(value, ["name", "directory", "build", "start", "address", "readyPath", "views", "answers", "typeScale"], "surface");
  text(value.name, "surface.name");
  text(value.directory, "surface.directory");
  const directory = await realpath(resolve(base, value.directory));
  if (!(await stat(directory)).isDirectory()) throw new Error("surface.directory must be a directory");
  for (const key of ["build", "start"]) {
    if (value[key] !== undefined) text(value[key], `surface.${key}`);
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
    object(view, ["name", "path", "states"], "view");
    text(view.name, "view.name");
    localPath(view.path, "view.path");
    if (view.states !== undefined) {
      array(view.states, "view.states");
      for (const state of view.states) {
        object(state, ["name", "steps"], "state");
        text(state.name, "state.name");
        if (state.name === "default") throw new Error("State name default is reserved");
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
  array(value.answers, "surface.answers");
  const answers = [];
  for (const answer of value.answers) {
    object(answer, ["method", "pattern", "file", "status", "headers"], "answer");
    text(answer.method, "answer.method");
    if (!/^[A-Z]+$/.test(answer.method)) throw new Error("answer.method must be uppercase");
    const pattern = addressPattern(answer.pattern);
    text(answer.file, "answer.file");
    const path = await realpath(resolve(base, answer.file));
    if (!within(base, path)) throw new Error("Answer files must be beside the surface file or in its subdirectories");
    if (answer.status !== undefined && (!Number.isInteger(answer.status) || answer.status < 200 || answer.status > 599)) {
      throw new Error("answer.status must be an HTTP status from 200 to 599");
    }
    if (answer.headers !== undefined) {
      if (!answer.headers || typeof answer.headers !== "object" || Array.isArray(answer.headers) || Object.values(answer.headers).some((v) => typeof v !== "string")) {
        throw new Error("answer.headers must contain string values");
      }
    }
    answers.push({ ...answer, pattern, body: await readFile(path) });
  }
  return { ...value, directory, address: address.origin, answers };
}
