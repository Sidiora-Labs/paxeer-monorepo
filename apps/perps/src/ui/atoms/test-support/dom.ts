import { createRequire } from "node:module";

// The declared gate installs the wallet demo's real DOM fixture dependency.
const fixtureRequire = createRequire(
  new URL("../../../../../../services/wallet/demo/package.json", import.meta.url),
);

export const JSDOM: new (
  html: string,
  options: { url: string; pretendToBeVisual: boolean },
) => { window: Window & typeof globalThis } = fixtureRequire("jsdom").JSDOM;
