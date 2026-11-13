# config

Configuration files kept beside the wallet app.

- [`env/schema.ts`](env/schema.ts) declares `EnvSchema` and reads values through `validateEnv(source)`, which defaults to `process.env`. It also exports the result as `env`. The app does not import this module; the variables the app reads are listed in the [app README](../README.md#configuration).
- [`typescript/base.json`](typescript/base.json) holds base TypeScript compiler options. The app's own [`tsconfig.json`](../tsconfig.json) does not extend it.
