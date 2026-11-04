# Env collector

`env-collector` generates and validates a Markdown environment-variable table from a
service's Serde `Settings` type and an example configuration file. This checkout
contains version **0.3.1**. It derives variable names from the serialized settings
hierarchy, using the service prefix and `__` separators; it does not inspect a
running node or discover variables from the process environment.

## Usage

Run the following commands from the server crate directory. The example assumes
that directory is `services/explorer/services/<service>/<service>-server`, so
the dependency path resolves to this library.

1. Add the dependency to the server crate's `Cargo.toml`:

   ```toml
   [dependencies]
   env-collector = { path = "../../libs/env-collector", version = "0.3.1" }
   ```

2. Create `src/bin/check-envs.rs`. Replace `your_server_crate::Settings` with the
   actual settings import and `MY_SERVICE` with its environment prefix. The type
   must implement both `serde::Serialize` and `serde::de::DeserializeOwned`.
   Point `config_path` at a configuration that can deserialize into that type.
   Cargo discovers this source file as the `check-envs` binary used by the just
   recipes, direct invocation and CI below. Keep the dependency version in step 1
   aligned with this library's package version when upgrading.

   ```rust
   use env_collector::{run_env_collector_cli, EnvCollectorSettingsBuilder, PrefixFilter};
   use your_server_crate::Settings;

   fn main() {
       run_env_collector_cli::<Settings>(
           EnvCollectorSettingsBuilder::default()
               .service_name("MY_SERVICE")
               .markdown_path("README.md")
               .config_path("config/base.toml")
               .vars_filter(PrefixFilter::blacklist(&[
                   "MY_SERVICE__SERVER",
                   "MY_SERVICE__JAEGER",
                   "MY_SERVICE__METRICS",
                   "MY_SERVICE__TRACING",
               ]))
               .anchor_postfix(Some("service".to_string()))
               .build()
               .expect("failed to build env collector settings"),
       );
   }
   ```

   The filter excludes shared settings from this example's table. Omit
   `vars_filter` to include all settings, or use `PrefixFilter::whitelist` to
   select prefixes. Paths are relative to the command's working directory.

3. Add the matching anchor pair to the target README, with the start before the
   end. Use one unique pair for the chosen postfix:

   ```markdown
   ## Envs

   [anchor]: <> (anchors.envs.start.service)
   [anchor]: <> (anchors.envs.end.service)
   ```

   If `anchor_postfix` is omitted, use `anchors.envs.start` and `anchors.envs.end`
   without a suffix. Existing descriptions are retained when the table updates;
   write operator-facing descriptions in its `Description` column.

4. Optionally add these recipes to the server crate's `justfile`:

   ```just
   check-envs *args:
       cargo run --bin check-envs -- --validate-only {{args}}

   generate-envs *args:
       cargo run --bin check-envs -- {{args}}
   ```

5. Generate the table with `just generate-envs`, or directly:

   ```sh
   cargo run --bin check-envs
   ```

   When differences exist, the CLI lists them and waits for an input line before
   updating the file. Press Enter to continue. If the table already matches, it
   reports that all variables are documented correctly and leaves it alone.

6. Validate without an interactive prompt in CI. For a job running from the
   server crate directory, use:

   ```yaml
   - name: Verify ENVs in README
     run: cargo run --bin check-envs -- --validate-only
   ```

   Validation exits with code 1 when the table differs. Generation, just recipes
   and CI all invoke the `check-envs` binary created in step 2.

## Validation behavior

The collector checks names, required markers and default values. It infers a
field's default by removing that field from serialized settings and attempting
to deserialize again; failure marks it required. Example-only values can become
suggested descriptions. The example configuration and Serde defaults therefore
determine the generated table, rather than providing a complete schema of every
possible configuration variant.

`--ignore-defaults` preserves and skips comparison of documented defaults.
`--ignore-unused` retains entries absent from the example configuration. Markdown
table formatting is enabled by default and can be disabled with the builder's
`format_markdown(false)` setting.

## Related service documentation

The [sig-provider configuration](../../sig-provider/README.md#envs) documents its
local signature source. The [verifier configuration](../../smart-contract-verifier/smart-contract-verifier-server/README.md#configuration)
describes compiler fetchers and settings omitted from its shipped examples. Check
each service's actual settings type and launcher version when maintaining its table.
