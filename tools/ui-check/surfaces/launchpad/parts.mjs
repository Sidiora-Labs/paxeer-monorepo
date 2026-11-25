import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(scriptDirectory, '../../../..');
const wholePath = path.join(repositoryRoot, 'tools/ui-check/surfaces/launchpad.json');
const partsDirectory = path.join(scriptDirectory, 'parts');
const partNames = ['home', 'community', 'coin', 'create', 'explore', 'rewards', 'live', 'terminal'];

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
function checkRoutes(name, roots, surface) {
  const appDirectory = path.join(repositoryRoot, 'apps/launchpad/web/src/app');
  for (const root of roots) {
    const directory = path.join(appDirectory, root);
    const pages = fs.readdirSync(directory, { recursive: true, withFileTypes: true })
      .filter((entry) => entry.isFile() && entry.name === 'page.tsx');
    for (const page of pages) {
      const route = path.relative(appDirectory, page.parentPath).split(path.sep);
      const covered = surface.views.some((view) => {
        const segments = new URL(view.path, surface.address).pathname.split('/').filter(Boolean);
        return route.length === segments.length && route.every((segment, index) =>
          /^\[[^\]]+\]$/.test(segment) ? segments[index].length > 0 : segment === segments[index]);
      });
      if (!covered) fail(`${name} part is missing route /${route.join('/')}`);
    }
  }
  const modes = ['default', 'reduced-motion', 'double-text', 'keyboard'];
  for (const view of surface.views) {
    for (const state of [view, ...(view.states ?? [])]) {
      const selected = state.modes ?? view.modes ?? modes;
      if (modes.some((mode) => !selected.includes(mode))) {
        fail(`${name} view ${view.name}, state ${state === view ? 'default' : state.name} is missing a render mode`);
      }
    }
  }
}

checkRoutes('coin', ['coin', 'token', 'tokens'], parts.find((part) => part.name === 'coin').surface);
checkRoutes('create', ['create'], parts.find((part) => part.name === 'create').surface);

const seen = new Map();
const sharedKeys = ['build', 'buildMs', 'start', 'address', 'readyPath', 'typeScale'];
const communityPaths = [
  '/bounties', '/bounties?render=empty', '/bounties?render=error', '/bounties?render=loading',
  '/bounties/8e894db0-87e7-4d2a-9f1c-3a192a239ba0', '/bounties/new',
  '/chat', '/chat/7f3b2a10-5c4d-4e8f-9a1b-2c3d4e5f6a7b',
  '/docs', '/docs/dmca', '/docs/do-not-sell', '/docs/privacy', '/docs/terms', '/docs/trademark-guidelines',
];
const community = parts.find(part => part.name === 'community').surface;
for (const route of communityPaths) {
  const view = community.views.find(candidate => candidate.path === route);
  if (!view) fail(`community is missing ${route}`);
  else {
    const modes = view.modes ?? ['default', 'reduced-motion', 'double-text', 'keyboard'];
    for (const mode of ['default', 'reduced-motion', 'double-text', 'keyboard']) {
      if (!modes.includes(mode)) fail(`community view ${view.name} is missing ${mode}`);
    }
  }
}

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
    const root = new URL(view.path, surface.address).pathname.split('/')[1];
    if ((part.name === 'coin') !== ['coin', 'token', 'tokens'].includes(root)) fail(`view ${view.name} does not belong to the ${part.name} route group`);
    if ((part.name === 'create') !== (root === 'create')) fail(`view ${view.name} does not belong to the ${part.name} route group`);
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
