// Task 22.33.476 repair record:
// env-anchor-substring -> env_anchor_substring_selects_only_the_requested_table
// env-anchor-order-panic -> env_anchor_order_panic_returns_errors_without_writing
// env-markdown-cell-escaping -> env_markdown_cell_escaping_round_trips_real_settings
// Baseline failures were not measured: the explicit gate order prohibits verification
// before implementation and requires the first verify invocation through hf-done.
// No out-of-path wiring is required; revision and gate evidence are recorded by hf-done.

use env_collector::{EnvCollector, EnvCollectorOptions, PrefixFilter, ReportedVariable};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use std::{fs, panic::AssertUnwindSafe, path::PathBuf};

#[derive(Serialize, Deserialize)]
struct BasicSettings {
    value: String,
}

#[derive(Serialize, Deserialize)]
struct TextSettings {
    value: String,
    #[serde(default = "text_default")]
    fallback: String,
}

fn text_default() -> String {
    "default|first\nsecond\r\n💡 & &#124; &#10; &amp;".to_string()
}

struct Fixture {
    _directory: tempfile::TempDir,
    markdown: PathBuf,
    config: PathBuf,
}

impl Fixture {
    fn new(markdown: &str, config: serde_json::Value) -> Self {
        let directory = tempfile::tempdir().unwrap();
        let markdown_path = directory.path().join("README.md");
        let config_path = directory.path().join("settings.json");
        fs::write(&markdown_path, markdown).unwrap();
        fs::write(&config_path, serde_json::to_vec(&config).unwrap()).unwrap();
        Self {
            _directory: directory,
            markdown: markdown_path,
            config: config_path,
        }
    }

    fn collector<S: Serialize + DeserializeOwned>(
        &self,
        postfix: Option<&str>,
        format_markdown: bool,
        filter: PrefixFilter,
    ) -> EnvCollector<S> {
        EnvCollector::new(
            "SERVICE".to_string(),
            self.markdown.clone(),
            self.config.clone(),
            filter,
            postfix.map(str::to_string),
            format_markdown,
        )
    }

    fn read(&self) -> String {
        fs::read_to_string(&self.markdown).unwrap()
    }
}

#[test]
fn env_anchor_substring_selects_only_the_requested_table() {
    for postfix in [None, Some("target"), Some("💡")] {
        let suffix = postfix.map(|s| format!(".{s}")).unwrap_or_default();
        let prefix = format!(
            "前言💡\r\n[anchor]: <> (anchors.envs.start{suffix}.other)\r\n\
             | `OTHER__VALUE` | true | preserve this | |\r\n\
             [anchor]: <> (anchors.envs.end{suffix}.other)\r\n\
             prose mentioning anchors.envs.start{suffix} and anchors.envs.end{suffix}\r\n\
             prose [anchor]: <> (anchors.envs.start{suffix}) is not a marker\r\n\
             [anchor]: <> (anchors.envs.start{suffix})\r\n"
        );
        let suffix_content = format!("[anchor]: <> (anchors.envs.end{suffix})\r\n尾声💡\r\n\r\n");
        let correct_row = "| `SERVICE__VALUE` | true | retained description | |\n";
        let fixture = Fixture::new(
            &format!("{prefix}{correct_row}{suffix_content}"),
            serde_json::json!({"value": "example"}),
        );
        let collector = fixture.collector::<BasicSettings>(postfix, true, PrefixFilter::Empty);
        let options = EnvCollectorOptions::default();
        assert!(collector.verify_markdown(&options).unwrap().is_empty());
        collector.update_markdown(&options).unwrap();
        let updated = fixture.read();
        assert!(updated.starts_with(&prefix));
        assert!(updated.ends_with(&suffix_content));
        assert!(updated.contains("retained description"));
        assert!(collector.verify_markdown(&options).unwrap().is_empty());

        fs::write(
            &fixture.markdown,
            format!("{prefix}| `SERVICE__VALUE` | | retained description | |\n{suffix_content}"),
        )
        .unwrap();
        let reports = collector.verify_markdown(&options).unwrap();
        assert_eq!(reports.len(), 1);
        assert!(
            matches!(&reports[0], ReportedVariable::Incorrect(var) if var.key == "SERVICE__VALUE")
        );
        collector.update_markdown(&options).unwrap();
        assert_eq!(fixture.read(), updated);
        let reopened = fixture.collector::<BasicSettings>(postfix, true, PrefixFilter::Empty);
        assert!(reopened.verify_markdown(&options).unwrap().is_empty());
    }
}

#[test]
fn env_anchor_order_panic_returns_errors_without_writing() {
    let start = "[anchor]: <> (anchors.envs.start)";
    let end = "[anchor]: <> (anchors.envs.end)";
    let malformed = [
        format!("{end}\n💡\n{start}\n"),
        format!("💡{end}\n{start}\n"),
        format!("{start}\n{start}\n{end}\n"),
        format!("{start}\n{end}\n{end}\n"),
        format!("{start}\n{end}\n{start}\n{end}\n"),
        format!("{start}\n"),
        format!("{end}\n"),
        "[anchor]: <> (anchors.envs.start.other)\n[anchor]: <> (anchors.envs.end.other)\n"
            .to_string(),
        "anchors.envs.end\n💡anchors.envs.start\n".to_string(),
        format!("prose {start}\n{end}\n"),
        format!("{start}\n{end} trailing prose\n"),
    ];
    for content in malformed {
        let fixture = Fixture::new(&content, serde_json::json!({"value": "example"}));
        let collector = fixture.collector::<BasicSettings>(None, true, PrefixFilter::Empty);
        let options = EnvCollectorOptions::default();
        let verification =
            std::panic::catch_unwind(AssertUnwindSafe(|| collector.verify_markdown(&options)))
                .expect("validation must return an error rather than panic");
        assert!(verification.unwrap_err().to_string().contains("marker"));
        let update =
            std::panic::catch_unwind(AssertUnwindSafe(|| collector.update_markdown(&options)))
                .expect("update must return an error rather than panic");
        assert!(update.unwrap_err().to_string().contains("marker"));
        assert_eq!(fs::read(&fixture.markdown).unwrap(), content.as_bytes());
    }

    let fixture = Fixture::new(
        &format!("💡\n{start}\n{end}"),
        serde_json::json!({"value": "example"}),
    );
    let collector = fixture.collector::<BasicSettings>(None, false, PrefixFilter::Empty);
    let options = EnvCollectorOptions::default();
    assert_eq!(collector.verify_markdown(&options).unwrap().len(), 1);
    collector.update_markdown(&options).unwrap();
    assert!(collector.verify_markdown(&options).unwrap().is_empty());
    assert!(fixture.read().starts_with(&format!("💡\n{start}\n")));
    assert!(fixture.read().ends_with(end));
}

#[test]
fn env_markdown_cell_escaping_round_trips_real_settings() {
    let example = "example|first\nsecond\r\n💡 & &#124; &#10; &amp;";
    for format_markdown in [false, true] {
        let fixture = Fixture::new(
            "前言\n[anchor]: <> (anchors.envs.start)\n[anchor]: <> (anchors.envs.end)\n尾声\n",
            serde_json::json!({"value": example, "fallback": example}),
        );
        let collector =
            fixture.collector::<TextSettings>(None, format_markdown, PrefixFilter::Empty);
        let options = EnvCollectorOptions::default();
        assert_eq!(collector.verify_markdown(&options).unwrap().len(), 2);
        collector.update_markdown(&options).unwrap();
        let generated = fixture.read();
        let rows: Vec<_> = generated
            .lines()
            .filter(|line| line.starts_with('|'))
            .collect();
        assert_eq!(rows.len(), 4, "two header rows and two variables");
        for row in rows {
            assert_eq!(row.split('|').count(), 6, "exactly four cells: {row}");
        }
        assert!(generated.contains("&#124;"));
        assert!(generated.contains("&#10;"));
        assert!(generated.contains("&#13;"));
        assert!(generated.contains("&amp;#124;"));
        assert!(collector.verify_markdown(&options).unwrap().is_empty());

        let reader = fixture.collector::<TextSettings>(
            None,
            format_markdown,
            PrefixFilter::whitelist(&["NOT_THIS_SERVICE"]),
        );
        let recovered = reader.verify_markdown(&options).unwrap();
        assert_eq!(recovered.len(), 2);
        for report in recovered {
            let ReportedVariable::Unused(var) = report else {
                panic!("the reader must report the generated variables as unused");
            };
            assert_eq!(var.description, format!("e.g. `{example}`"));
            match var.key.as_str() {
                "SERVICE__VALUE" => {
                    assert!(var.required);
                    assert_eq!(var.default_value, None);
                }
                "SERVICE__FALLBACK" => {
                    assert!(!var.required);
                    assert_eq!(var.default_value, Some(format!("`{}`", text_default())));
                }
                key => panic!("unexpected recovered key: {key}"),
            }
        }
        collector.update_markdown(&options).unwrap();
        assert_eq!(
            fixture.read(),
            generated,
            "encoding must be stable across updates"
        );
        let reopened =
            fixture.collector::<TextSettings>(None, format_markdown, PrefixFilter::Empty);
        assert!(reopened.verify_markdown(&options).unwrap().is_empty());
    }
}
