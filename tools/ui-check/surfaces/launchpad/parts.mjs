import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(scriptDirectory, '../../../..');
const wholePath = path.join(repositoryRoot, 'tools/ui-check/surfaces/launchpad.json');
const partsDirectory = path.join(scriptDirectory, 'parts');
const partNames = ['home', 'community', 'coin', 'explore', 'rewards', 'live', 'terminal'];

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]));
  }
  return value;
}

function equal(left, right) {
  return JSON.stringify(canonical(left)) === JSON.stringify(canonical(right));
}

function fail(message) {
  console.error(`launchpad surface partition invalid: ${message}`);
  process.exitCode = 1;
}

const whole = readJson(wholePath);
const parts = partNames.map((name) => ({ name, file: path.join(partsDirectory, `${name}.json`), surface: readJson(path.join(partsDirectory, `${name}.json`)) }));
const seen = new Map();
const sharedKeys = ['build', 'buildMs', 'start', 'address', 'readyPath', 'typeScale'];

for (const part of parts) {
  const { surface } = part;
  for (const key of sharedKeys) {
    if (!equal(surface[key], whole[key])) fail(`${part.name} part differs in ${key}`);
  }
  if (!equal(surface.answers?.map((answer) => ({ ...answer, file: undefined })), whole.answers?.map((answer) => ({ ...answer, file: undefined })))) {
    fail(`${part.name} part differs in answers`);
  }
  const wholeAnswers = whole.answers ?? [];
  const partAnswers = surface.answers ?? [];
  if (partAnswers.length !== wholeAnswers.length) fail(`${part.name} part has a different answer count`);
  for (let index = 0; index < wholeAnswers.length; index += 1) {
    const wholeAnswer = wholeAnswers[index];
    const partAnswer = partAnswers[index];
    const withoutFile = (answer) => {
      const { file, ...rest } = answer;
      return rest;
    };
    if (!equal(withoutFile(partAnswer), withoutFile(wholeAnswer))) fail(`${part.name} answer ${index} differs from the whole surface answer`);
    const wholeFile = path.resolve(path.dirname(wholePath), wholeAnswer.file);
    const partFile = path.resolve(path.dirname(part.file), partAnswer.file);
    if (!fs.existsSync(partFile) || fs.readFileSync(partFile, 'utf8') !== fs.readFileSync(wholeFile, 'utf8')) {
      fail(`${part.name} answer ${index} does not contain the whole surface answer file`);
    }
  }
  const resolvedWholeDirectory = path.resolve(path.dirname(wholePath), whole.directory);
  const resolvedPartDirectory = path.resolve(path.dirname(part.file), surface.directory);
  if (resolvedPartDirectory !== resolvedWholeDirectory) fail(`${part.name} part directory resolves to ${resolvedPartDirectory}`);
  for (const view of surface.views) {
    if (seen.has(view.name)) fail(`view ${view.name} appears in ${seen.get(view.name)} and ${part.name}`);
    seen.set(view.name, part.name);
    const liveRoute = /^\/(live|pnl|profile)(\/|\?|$)/.test(view.path);
    if ((part.name === 'live') !== liveRoute) fail(`view ${view.name} does not belong to the ${part.name} route group`);
    const wholeView = whole.views.find((candidate) => candidate.name === view.name);
    if (!wholeView) fail(`${part.name} contains unknown view ${view.name}`);
    else if (!equal(view, wholeView)) fail(`view ${view.name} differs from the whole surface`);
  }
}

for (const view of whole.views) {
  if (!seen.has(view.name)) fail(`view ${view.name} is missing from all parts`);
}
for (const name of seen.keys()) {
  if (!whole.views.some((view) => view.name === name)) fail(`part view ${name} is not in the whole surface`);
}
if (seen.size !== whole.views.length) fail(`partition contains ${seen.size} views for ${whole.views.length} whole-surface views`);
if (process.exitCode) process.exit();
console.log(`launchpad surface partition valid: ${whole.views.length} views across ${parts.length} parts`);
for (const part of parts) console.log(`${part.name}: ${part.surface.views.length} views`);
