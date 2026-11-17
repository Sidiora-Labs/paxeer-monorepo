const { TextDecoder, TextEncoder } = require("node:util");

globalThis.TextDecoder = TextDecoder;
globalThis.TextEncoder = TextEncoder;
