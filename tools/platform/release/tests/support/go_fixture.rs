use crate::release::{registries_declarations, workflow};

pub const ARCHIVE: &[u8] = include_bytes!("go-module-v0.1.0.zip");

pub struct GoFixture {
    pub module: String,
    pub location: String,
    pub tag: String,
}

impl GoFixture {
    pub fn load(version: &str) -> Self {
        let module = include_str!("../../../../../sdk/platform/go/go.mod")
            .lines()
            .find_map(|line| line.strip_prefix("module "))
            .expect("canonical Go module declaration")
            .trim();
        let declarations = registries_declarations(include_str!("../../registries.kvx"))
            .expect("committed registry declarations");
        let registry = declarations
            .registries
            .iter()
            .find(|registry| registry.name == "go-modules")
            .expect("Go registry declaration");
        assert_eq!(registry.packages, [module]);
        let distribution = registry
            .declarations
            .iter()
            .find(|(key, _)| key == "distribution")
            .map(|(_, value)| value)
            .expect("Go registry distribution");
        let escaped = module
            .chars()
            .flat_map(|character| {
                if character.is_ascii_uppercase() {
                    vec!['!', character.to_ascii_lowercase()]
                } else {
                    vec![character]
                }
            })
            .collect::<String>();
        let directory = module.split('/').skip(3).collect::<Vec<_>>().join("/");
        assert!(
            !directory.is_empty(),
            "Go module must name its repository subdirectory"
        );
        let tag = format!("{directory}/v{version}");
        let document = workflow::parse(include_str!(
            "../../../../../.github/workflows/platform.yml"
        ))
        .expect("committed publication workflow");
        let job = document
            .path(&["jobs", "publish-go-modules"])
            .expect("Go publication job");
        assert_eq!(
            job.path(&["env", "LAYERX_RELEASE_PACKAGES"])
                .and_then(workflow::Node::as_str),
            Some(module)
        );
        assert_eq!(
            job.path(&["env", "LAYERX_RELEASE_DISTRIBUTION"])
                .and_then(workflow::Node::as_str),
            Some(distribution.as_str())
        );
        let scripts = job
            .get("steps")
            .expect("publication steps")
            .items()
            .iter()
            .filter_map(|step| step.get("run").and_then(workflow::Node::as_str))
            .collect::<Vec<_>>();
        let signing = scripts
            .iter()
            .find(|script| script.contains("git tag -s"))
            .expect("signed module tag producer");
        assert!(signing.contains(&format!("tag={directory}/v$LAYERX_RELEASE_VERSION")));
        assert!(signing.contains("git tag -s \"$tag\""));
        assert!(signing.contains(&format!("refs/tags/{directory}/v$LAYERX_RELEASE_VERSION")));
        let download = scripts
            .iter()
            .find(|script| script.contains("go mod download -json"))
            .expect("module archive producer");
        assert!(download.contains("go mod download -json \"$module@v$LAYERX_RELEASE_VERSION\""));
        assert!(download
            .contains("${LAYERX_RELEASE_TARGET%/}/$escaped/@v/v$LAYERX_RELEASE_VERSION.zip"));
        Self {
            module: module.to_owned(),
            location: format!(
                "{}/{escaped}/@v/v{version}.zip",
                distribution.trim_end_matches('/')
            ),
            tag,
        }
    }
}
