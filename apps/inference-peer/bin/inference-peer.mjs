#!/usr/bin/env node
import { runCLI } from "../src/cli.mjs";

try {
  await runCLI(process.argv.slice(2));
} catch (error) {
  console.error(`inference-peer: ${error.message}`);
  process.exitCode = 1;
}
