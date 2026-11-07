use serde_json::{json, Value};
use std::{
    collections::{BTreeMap, BTreeSet},
    fs,
    path::{Path, PathBuf},
};

fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../..")
}

fn canonical(value: Value) -> Value {
    match value {
        Value::Object(values) => Value::Object(
            values
                .into_iter()
                .map(|(key, value)| (key, canonical(value)))
                .collect::<BTreeMap<_, _>>()
                .into_iter()
                .collect(),
        ),
        Value::Array(values) => Value::Array(values.into_iter().map(canonical).collect()),
        value => value,
    }
}

fn block<'a>(code: &'a str, declaration: &str) -> &'a str {
    code.split_once(declaration)
        .unwrap()
        .1
        .split_once("\n}")
        .unwrap()
        .0
}

fn go_files(directory: &Path) -> Vec<PathBuf> {
    let mut paths: Vec<_> = fs::read_dir(directory)
        .unwrap()
        .map(|entry| entry.unwrap().path())
        .filter(|path| {
            path.extension().is_some_and(|ext| ext == "go")
                && !path
                    .file_name()
                    .unwrap()
                    .to_str()
                    .unwrap()
                    .ends_with("_test.go")
        })
        .collect();
    paths.sort();
    paths
}

fn code(paths: &[PathBuf]) -> String {
    paths
        .iter()
        .map(|path| fs::read_to_string(path).unwrap())
        .collect::<Vec<_>>()
        .join("\n")
}

fn receiver_names<'a>(code: &'a str, call: &str) -> BTreeSet<&'a str> {
    code.split(call)
        .collect::<Vec<_>>()
        .into_iter()
        .rev()
        .skip(1)
        .map(|prefix| {
            prefix
                .rsplit(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_')
                .next()
                .unwrap()
        })
        .collect()
}

pub fn declared_abi_inventory() -> Value {
    let embedded: Value = serde_json::from_str(include_str!("precompiles.json")).unwrap();
    let setup = fs::read_to_string(root().join("precompiles/setup.go")).unwrap();
    let sources: Vec<_> = block(&setup, "var precompileInfoSources =")
        .lines()
        .filter(|line| line.trim_start().starts_with('"'))
        .map(|row| {
            let fields: Vec<_> = row.split('"').collect();
            (fields[1].to_owned(), fields[3].to_owned())
        })
        .collect();
    assert!(!sources.is_empty(), "declared ABI registry must be present");
    let packages: BTreeSet<_> = sources
        .iter()
        .map(|(_, path)| path.split('/').next().unwrap())
        .collect();
    for package in receiver_names(
        block(&setup, "func GetCustomPrecompilesE("),
        ".GetVersioned(",
    ) {
        assert!(
            packages.contains(package),
            "registered producer {package} has no ABI inventory"
        );
    }
    let merge: Vec<_> = block(&setup, "var MergeAddressTable =")
        .lines()
        .filter(|line| line.contains("HexToAddress"))
        .map(|row| {
            let fields: Vec<_> = row.split('"').collect();
            (fields[3].to_owned(), fields[1].to_ascii_lowercase())
        })
        .collect();
    assert!(!merge.is_empty(), "merge ABI registry must be present");
    let inline: BTreeSet<_> = block(&setup, "var mergeAgentInfoABIs =")
        .lines()
        .filter(|line| line.trim_start().starts_with('"') && line.contains(": `"))
        .map(|line| line.split('"').nth(1).unwrap())
        .collect();
    assert!(!inline.is_empty());
    assert!(inline
        .iter()
        .all(|name| merge.iter().any(|(candidate, _)| candidate == name)));

    let app = fs::read_to_string(root().join("node/app.go")).unwrap();
    let switch = block(&app, "func (app *App) v6120Contracts()");
    let node_code = code(&go_files(&root().join("node")));
    let imports: BTreeMap<_, _> = node_code
        .lines()
        .filter_map(|line| {
            let (alias, remainder) = line.split_once('"')?;
            let path = remainder.split('"').next()?;
            path.strip_prefix("github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/")
                .map(|path| (alias.trim(), path))
        })
        .collect();
    let mut historical = BTreeSet::new();
    for alias in receiver_names(switch, ".NewPrecompile(")
        .union(&receiver_names(switch, ".NewPrecompileWithKeeper("))
    {
        let package = imports[alias];
        if package.contains("/legacy/") {
            historical.insert(format!("precompiles/{package}/abi.json"));
        } else {
            assert!(
                packages.contains(package),
                "versioned producer {package} has no ABI inventory"
            );
        }
    }
    assert!(
        switch.contains("spotprecompile.NewServedSet(keepers)"),
        "spot producer registry changed"
    );

    let mut inventory = Vec::new();
    let mut expected = BTreeSet::new();
    let mut add = |name: &str, origin: &str, raw: String, status: &str| {
        let abi: Vec<Value> = serde_json::from_str(&raw).unwrap();
        expected.extend(
            abi.into_iter()
                .filter(|item| item["type"] == "function" || item["type"] == "event")
                .map(|item| canonical(item).to_string()),
        );
        inventory.push(json!({"name":name,"origin":origin,"status":status}));
    };
    for (name, path) in &sources {
        let origin = format!("precompiles/{path}");
        add(
            name,
            &origin,
            fs::read_to_string(root().join(&origin)).unwrap(),
            if path == "layerxexchange/abi.json" {
                "historical"
            } else {
                "versioned"
            },
        );
    }
    for (name, address) in merge {
        if inline.contains(name.as_str()) {
            let mut packages: Vec<_> = fs::read_dir(root().join("precompiles/layerxagent"))
                .unwrap()
                .map(|entry| entry.unwrap().path())
                .filter(|path| path.is_dir())
                .collect();
            packages.sort();
            let matches: Vec<_> = packages
                .into_iter()
                .map(|path| go_files(&path))
                .filter(|files| {
                    let source = code(files);
                    source.to_ascii_lowercase().contains(&address)
                        && source.contains("RegisterMergeConstructor")
                })
                .collect();
            assert_eq!(matches.len(), 1, "no unique served ABI producer for {name}");
            let files = &matches[0];
            let source = code(files);
            let readers: BTreeSet<_> = source
                .split("abi.JSON(strings.NewReader(")
                .skip(1)
                .map(|tail| tail.split(')').next().unwrap())
                .collect();
            let mut found = BTreeSet::new();
            for path in files {
                let source = fs::read_to_string(path).unwrap();
                let chunks: Vec<_> = source.split('`').collect();
                for pair in chunks.chunks_exact(2) {
                    let Some((prefix, _)) = pair[0].rsplit_once('=') else {
                        continue;
                    };
                    let symbol = prefix.split_whitespace().last().unwrap();
                    if readers.contains(symbol) {
                        let origin =
                            format!("{}#{symbol}", path.strip_prefix(root()).unwrap().display());
                        add(&name, &origin, pair[1].to_owned(), "merge");
                        found.insert(symbol.to_owned());
                    }
                }
            }
            assert!(!readers.is_empty());
            assert_eq!(
                readers
                    .into_iter()
                    .map(str::to_owned)
                    .collect::<BTreeSet<_>>(),
                found,
                "unresolved served ABI constants for {name}"
            );
        } else {
            let origin = format!("precompiles/{name}/abi.json");
            add(
                &name,
                &origin,
                fs::read_to_string(root().join(&origin)).unwrap(),
                "merge",
            );
        }
    }
    for origin in historical {
        add(
            "version-switch predecessor",
            &origin,
            fs::read_to_string(root().join(&origin)).unwrap(),
            "historical",
        );
    }
    add(
        "audit-22.30.135 snapshot",
        "precompiles_historical.json",
        include_str!("precompiles_historical.json").to_owned(),
        "historical",
    );
    assert_eq!(
        embedded["inventory"],
        Value::Array(inventory),
        "embedded inventory must exactly match declared served producers"
    );
    let actual: BTreeSet<_> = embedded["abi"]
        .as_array()
        .unwrap()
        .iter()
        .cloned()
        .map(|item| canonical(item).to_string())
        .collect();
    assert_eq!(
        actual, expected,
        "embedded ABI must exactly match the declared producer and historical union"
    );
    Value::Array(
        expected
            .into_iter()
            .map(|item| serde_json::from_str(&item).unwrap())
            .collect(),
    )
}
