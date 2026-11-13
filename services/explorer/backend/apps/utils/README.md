# Utils

The Blockscout `Utils` application provides shared configuration and media helpers.

- [`Utils.CompileTimeEnvHelper`](lib/utils/compile_time_env_helper.ex) creates module attributes from application configuration and a `__mix_recompile__?/0` callback that compares the captured values with current application settings.
- [`Utils.TokenInstanceHelper`](lib/utils/token_instance_helper.ex) determines media types from file extensions, uses an HTTP HEAD request when an extension is absent, and recognizes image/video data URLs when enabled by the caller.
- [`Utils.Credo.Checks.CompileEnvUsage`](lib/credo/checks/compile_env_usage.ex) reports direct `Application.compile_env` and `Application.compile_env!` AST references, except in the compile-time helper file.
