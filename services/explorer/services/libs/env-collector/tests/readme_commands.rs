// Task 22.33.477 repair record:
// env-readme-binary-name -> env_readme_binary_name_matches_commands_and_release
//                      -> env_readme_binary_name_drives_real_generation_and_validation
// The starting README already used check-envs and package version 0.3.1.
// Baseline failures were not measured: the explicit gate order requires complete
// implementation before hf-build and the first verification through hf-done.
// No out-of-path wiring is required; hf-done records the revision and gate evidence.

use clap::Parser;
use config::{Config, File, FileFormat};
use env_collector::{EnvCollector, EnvCollectorOptions, EnvCollectorSettingsBuilder, PrefixFilter};
use serde::{Deserialize, Serialize};
use std::{fs, path::Path};

const README: &str = include_str!("../README.md");

fn code_block(language: &str) -> &str {
    README
        .split_once(&format!("```{language}\n"))
        .expect("documented code block")
        .1
        .split_once("```")
        .expect("closed code block")
        .0
}

fn documented_commands() -> Vec<Vec<&'static str>> {
    ["just", "sh", "yaml"]
        .into_iter()
        .flat_map(|language| {
            code_block(language).lines().filter_map(|line| {
                line.split_once("cargo run ")
                    .map(|(_, command)| command.split_whitespace().collect())
            })
        })
        .collect()
}

fn command_options(command: &[&str]) -> EnvCollectorOptions {
    let separator = command.iter().position(|arg| *arg == "--");
    let args = separator.map(|index| &command[index + 1..]).unwrap_or(&[]);
    EnvCollectorOptions::try_parse_from(
        std::iter::once(command[1]).chain(args.iter().copied().filter(|arg| *arg != "{{args}}")),
    )
    .expect("documented flags must be accepted by the production CLI parser")
}

#[test]
fn env_readme_binary_name_matches_commands_and_release() {
    let source = README
        .split_once("2. Create `")
        .unwrap()
        .1
        .split_once('`')
        .unwrap()
        .0;
    let source = Path::new(source);
    assert_eq!(source.parent().unwrap(), Path::new("src/bin"));
    assert_eq!(source.extension().unwrap(), "rs");
    let binary = source.file_stem().unwrap().to_str().unwrap();
    assert_eq!(binary, "check-envs");

    let commands = documented_commands();
    assert_eq!(
        commands.len(),
        4,
        "two just recipes, direct invocation and CI"
    );
    for command in &commands {
        assert_eq!(&command[..2], ["--bin", binary]);
        command_options(command);
    }
    assert_eq!(
        commands[0],
        ["--bin", binary, "--", "--validate-only", "{{args}}"]
    );
    assert_eq!(commands[1], ["--bin", binary, "--", "{{args}}"]);
    assert_eq!(commands[2], ["--bin", binary]);
    assert_eq!(commands[3], ["--bin", binary, "--", "--validate-only"]);

    let dependency = Config::builder()
        .add_source(File::from_str(code_block("toml"), FileFormat::Toml))
        .build()
        .unwrap();
    assert_eq!(
        dependency
            .get_string("dependencies.env-collector.version")
            .unwrap(),
        env!("CARGO_PKG_VERSION")
    );
    assert_eq!(
        dependency
            .get_string("dependencies.env-collector.path")
            .unwrap(),
        "../../libs/env-collector"
    );
    assert!(README.contains(&format!("version **{}**", env!("CARGO_PKG_VERSION"))));
}

#[derive(Serialize, Deserialize)]
struct Settings {
    endpoint: String,
    #[serde(default = "default_attempts")]
    attempts: u32,
    server: SharedSettings,
}

#[derive(Serialize, Deserialize)]
struct SharedSettings {
    enabled: bool,
}

fn default_attempts() -> u32 {
    3
}

#[test]
fn env_readme_binary_name_drives_real_generation_and_validation() {
    let directory = tempfile::tempdir().unwrap();
    let markdown = directory.path().join("README.md");
    let config = directory.path().join("config/base.toml");
    fs::create_dir(config.parent().unwrap()).unwrap();
    fs::write(
        &config,
        "endpoint = 'loopback'\nattempts = 7\n[server]\nenabled = true\n",
    )
    .unwrap();
    fs::write(&markdown, code_block("markdown")).unwrap();

    // Exercise the documented builder and the same collector path the CLI calls,
    // in process so validation failures cannot exit the test runner.
    let settings = EnvCollectorSettingsBuilder::default()
        .service_name("MY_SERVICE")
        .markdown_path(&markdown)
        .config_path(&config)
        .vars_filter(PrefixFilter::blacklist(&[
            "MY_SERVICE__SERVER",
            "MY_SERVICE__JAEGER",
            "MY_SERVICE__METRICS",
            "MY_SERVICE__TRACING",
        ]))
        .anchor_postfix(Some("service".to_string()))
        .build()
        .unwrap();
    let _cli: fn(env_collector::EnvCollectorSettings) =
        env_collector::run_env_collector_cli::<Settings>;
    let collector = EnvCollector::<Settings>::new(
        settings.service_name,
        settings.markdown_path,
        settings.config_path,
        settings.vars_filter,
        settings.anchor_postfix,
        settings.format_markdown,
    );
    let commands = documented_commands();
    let generation = command_options(&commands[2]);
    let validation = command_options(&commands[3]);
    let missing = collector.verify_markdown(&validation).unwrap();
    assert_eq!(missing.len(), 2);
    let endpoint = missing
        .iter()
        .find(|var| var.inner().key == "MY_SERVICE__ENDPOINT")
        .unwrap()
        .inner();
    assert!(endpoint.required);
    assert_eq!(endpoint.default_value, None);
    let attempts = missing
        .iter()
        .find(|var| var.inner().key == "MY_SERVICE__ATTEMPTS")
        .unwrap()
        .inner();
    assert!(!attempts.required);
    assert_eq!(attempts.default_value.as_deref(), Some("`3`"));

    collector.update_markdown(&generation).unwrap();
    let generated = fs::read(&markdown).unwrap();
    for command in &commands {
        assert!(collector
            .verify_markdown(&command_options(command))
            .unwrap()
            .is_empty());
        assert_eq!(
            fs::read(&markdown).unwrap(),
            generated,
            "validation must not write"
        );
    }
    let generated_text = String::from_utf8(generated).unwrap();
    assert!(generated_text.contains("`MY_SERVICE__ENDPOINT`"));
    assert!(generated_text.contains("`MY_SERVICE__ATTEMPTS`"));
    assert!(!generated_text.contains("MY_SERVICE__SERVER"));

    let stale = generated_text.replace("`3`", "`99`");
    assert_ne!(stale, generated_text);
    fs::write(&markdown, &stale).unwrap();
    let incorrect = collector.verify_markdown(&validation).unwrap();
    assert_eq!(incorrect.len(), 1);
    assert_eq!(incorrect[0].inner().key, "MY_SERVICE__ATTEMPTS");
    assert_eq!(fs::read_to_string(&markdown).unwrap(), stale);
    collector.update_markdown(&generation).unwrap();
    assert_eq!(fs::read_to_string(&markdown).unwrap(), generated_text);
    assert!(collector.verify_markdown(&validation).unwrap().is_empty());
}
