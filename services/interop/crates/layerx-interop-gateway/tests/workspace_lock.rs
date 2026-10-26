use std::collections::{BTreeMap, BTreeSet};

const WORKSPACE: &str = include_str!("../../../Cargo.toml");
const LOCK: &str = include_str!("../../../Cargo.lock");

macro_rules! member_manifests {
    ($($name:literal),+ $(,)?) => {
        &[$((concat!("crates/", $name), include_str!(concat!("../../", $name, "/Cargo.toml")))),+]
    };
}

const MEMBERS: &[(&str, &str)] = member_manifests!(
    "layerx-ap2",
    "layerx-bridge-relayer",
    "layerx-committed-proof",
    "layerx-gas-station",
    "layerx-interop-gateway",
    "layerx-interop-service",
    "layerx-intent-ingester",
    "layerx-fiat",
    "layerx-migrate",
    "layerx-mirror",
    "layerx-mirror-signer",
    "layerx-portable",
    "layerx-ucp",
    "layerx-visa-tap",
    "layerx-x402",
    "x-websearch",
);

fn table_lines(document: &str) -> impl Iterator<Item = (&str, &str)> {
    let mut table = "";
    document.lines().filter_map(move |line| {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            None
        } else if line.starts_with('[') {
            assert!(line.ends_with(']'), "unsupported TOML table: {line}");
            table = line.trim_start_matches('[').trim_end_matches(']');
            None
        } else {
            Some((table, line))
        }
    })
}

fn assignment(line: &str) -> (&str, &str) {
    let Some((key, value)) = line.split_once('=') else {
        panic!("unsupported TOML assignment: {line}");
    };
    (key.trim(), value.trim())
}

fn quoted(value: &str) -> &str {
    let Some(value) = value.strip_prefix('"').and_then(|s| s.strip_suffix('"')) else {
        panic!("expected a TOML string: {value}");
    };
    assert!(
        !value.contains(['"', '\\']),
        "unsupported TOML string: {value}"
    );
    value
}

fn inline_fields(value: &str) -> BTreeMap<&str, &str> {
    let Some(body) = value.strip_prefix('{').and_then(|s| s.strip_suffix('}')) else {
        panic!("expected an inline dependency table: {value}");
    };
    let mut fields = BTreeMap::new();
    let mut start = 0;
    let mut depth = 0;
    let mut in_string = false;
    let mut escaped = false;
    for (index, ch) in body.char_indices() {
        if in_string {
            if escaped {
                escaped = false;
            } else if ch == '\\' {
                escaped = true;
            } else if ch == '"' {
                in_string = false;
            }
        } else {
            match ch {
                '"' => in_string = true,
                '[' | '{' => depth += 1,
                ']' | '}' => {
                    assert!(depth > 0, "unbalanced dependency table: {value}");
                    depth -= 1;
                }
                ',' if depth == 0 => {
                    let (key, value) = assignment(&body[start..index]);
                    assert!(
                        fields.insert(key, value).is_none(),
                        "duplicate field: {key}"
                    );
                    start = index + 1;
                }
                _ => {}
            }
        }
    }
    assert!(
        !in_string && depth == 0,
        "unbalanced dependency table: {value}"
    );
    if !body[start..].trim().is_empty() {
        let (key, value) = assignment(&body[start..]);
        assert!(
            fields.insert(key, value).is_none(),
            "duplicate field: {key}"
        );
    }
    fields
}

fn dependency_fields<'a>(key: &'a str, value: &'a str) -> (&'a str, BTreeMap<&'a str, &'a str>) {
    if let Some(name) = key.strip_suffix(".workspace") {
        assert_eq!(value, "true", "workspace dependency must be inherited");
        (name, BTreeMap::from([("workspace", value)]))
    } else if value.starts_with('"') {
        quoted(value);
        (key, BTreeMap::new())
    } else {
        (key, inline_fields(value))
    }
}

#[test]
fn lock_lists_every_member_dependency() {
    let mut member_lines = table_lines(WORKSPACE)
        .filter(|(table, _)| *table == "workspace")
        .map(|(_, line)| line);
    assert_eq!(member_lines.next(), Some("members = ["));
    let mut declared_members = BTreeSet::new();
    loop {
        let Some(line) = member_lines.next() else {
            panic!("workspace members array is not closed");
        };
        if line == "]" {
            break;
        }
        let member = quoted(line.trim_end_matches(','));
        assert!(
            declared_members.insert(member),
            "duplicate member: {member}"
        );
    }
    let embedded_members: BTreeSet<_> = MEMBERS.iter().map(|(path, _)| *path).collect();
    assert_eq!(
        declared_members, embedded_members,
        "embed every workspace member manifest"
    );
    assert_eq!(embedded_members.len(), MEMBERS.len());

    let packages: BTreeSet<_> = table_lines(LOCK)
        .filter(|(table, line)| *table == "package" && line.starts_with("name = "))
        .map(|(_, line)| quoted(assignment(line).1))
        .collect();
    assert!(!packages.is_empty(), "lockfile has no package entries");
    let workspace_dependencies: BTreeMap<_, _> = table_lines(WORKSPACE)
        .filter(|(table, _)| *table == "workspace.dependencies")
        .map(|(_, line)| assignment(line))
        .collect();

    for (path, manifest) in MEMBERS {
        let mut dependency_count = 0;
        for (table, line) in table_lines(manifest) {
            let is_dependency = |part| {
                matches!(
                    part,
                    "dependencies" | "dev-dependencies" | "build-dependencies"
                )
            };
            if !table.split('.').any(is_dependency) {
                continue;
            }
            assert!(
                table.rsplit('.').next().is_some_and(is_dependency),
                "{path}: dependency subtables require parser support: {table}"
            );
            let (key, value) = assignment(line);
            let (name, fields) = dependency_fields(key, value);
            let mut package = name;
            if let Some(optional) = fields.get("optional") {
                assert!(
                    matches!(*optional, "true" | "false"),
                    "invalid optional flag"
                );
                if *optional == "true" {
                    continue;
                }
            }
            if let Some(workspace) = fields.get("workspace") {
                assert_eq!(*workspace, "true", "invalid workspace flag");
                let Some(inherited) = workspace_dependencies.get(name) else {
                    panic!("{path}: {name} is missing from workspace.dependencies");
                };
                let (_, inherited_fields) = dependency_fields(name, inherited);
                if let Some(renamed) = inherited_fields.get("package") {
                    package = quoted(renamed);
                }
            }
            if let Some(renamed) = fields.get("package") {
                package = quoted(renamed);
            }
            assert!(
                packages.contains(package),
                "{path}: {table} dependency {name} ({package}) has no Cargo.lock package entry"
            );
            dependency_count += 1;
        }
        assert!(
            dependency_count > 0,
            "{path}: no non-optional dependencies were checked"
        );
    }
}
