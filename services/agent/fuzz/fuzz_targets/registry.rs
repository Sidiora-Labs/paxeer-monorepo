use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

const GENESIS: &str = include_str!("../../../../src/protocol/lxp_genesis.c");
const MODULES: &str = include_str!("../../../../include/layerx/lxp_module.h");

// Embed the production definitions so fuzzing needs neither a C toolchain nor a
// native library at runtime. The genesis table selects the interface/version;
// the selected interface supplies its exact (possibly sparse) activity array.
const SOURCES: &[(&str, &str, &str)] = &[
    (
        "LXP_MODULE_ASSET",
        include_str!("../../../../src/modules/asset/lx_asset_registry.c"),
        include_str!("../../../../include/layerx/lx_asset.h"),
    ),
    (
        "LXP_MODULE_ESCROW",
        include_str!("../../../../src/modules/escrow/lx_escrow_module.c"),
        include_str!("../../../../include/layerx/lx_escrow.h"),
    ),
    (
        "LXP_MODULE_BUDGET",
        include_str!("../../../../src/modules/budget/lx_budget_module.c"),
        include_str!("../../../../include/layerx/lx_budget.h"),
    ),
    (
        "LXP_MODULE_STREAM",
        include_str!("../../../../src/modules/stream/lx_stream_open.c"),
        include_str!("../../../../include/layerx/lx_stream.h"),
    ),
    (
        "LXP_MODULE_SERVICE",
        include_str!("../../../../src/modules/service/lx_service_module.c"),
        include_str!("../../../../include/layerx/lx_service.h"),
    ),
    (
        "LXP_MODULE_PERPS",
        include_str!("../../../../src/modules/perps/lx_perps_dispatch.c"),
        include_str!("../../../../include/layerx/lx_perps.h"),
    ),
    (
        "LXP_MODULE_GOVERNANCE",
        include_str!("../../../../src/modules/governance/lxp_governance.c"),
        include_str!("../../../../include/layerx/lxp_governance.h"),
    ),
    (
        "LXP_MODULE_PROGRAMS",
        include_str!("../../../../src/modules/programs/registration.c"),
        include_str!("../../../../include/layerx/programs.h"),
    ),
    (
        "LXP_MODULE_SPOT",
        include_str!("../../../../src/modules/spot/lx_spot_dispatch.c"),
        include_str!("../../../../include/layerx/lx_spot.h"),
    ),
];

fn number(token: &str) -> u32 {
    let token = token.trim().trim_end_matches(['U', 'L', 'u', 'l']);
    if let Some(hex) = token.strip_prefix("0x") {
        u32::from_str_radix(hex, 16).expect("production hexadecimal constant")
    } else {
        token.parse().expect("production integer constant")
    }
}

fn expression(header: &str, token: &str) -> u32 {
    let token = token.trim();
    if let Some((module, ordinal)) = token.split_once('|') {
        let (name, shift) = module
            .trim()
            .strip_prefix('(')
            .and_then(|v| v.strip_suffix(')'))
            .expect("packed activity expression")
            .split_once("<<")
            .expect("packed module shift");
        return constant(MODULES, name.trim())
            .checked_shl(number(shift))
            .expect("activity module shift width")
            | number(ordinal);
    }
    if token.starts_with("LXP_") || token.starts_with("LX_") {
        return constant(header, token);
    }
    number(token)
}

fn constant(header: &str, name: &str) -> u32 {
    let values: Vec<_> = header
        .lines()
        .filter_map(|line| {
            let (left, right) = line.trim().split_once('=')?;
            (left.trim() == name)
                .then(|| expression(header, right.split(',').next().expect("constant value")))
        })
        .collect();
    assert_eq!(
        values.len(),
        1,
        "missing or ambiguous production constant {name}"
    );
    values[0]
}

fn initializer<'a>(source: &'a str, name: &str) -> &'a str {
    let marker = format!("{name}[]");
    let (_, rest) = source
        .split_once(&marker)
        .expect("production registration array");
    let (_, rest) = rest.split_once('{').expect("array initializer");
    rest.split_once("};").expect("array terminator").0
}

fn function_body<'a>(source: &'a str, name: &str) -> &'a str {
    let marker = format!("*{name}(");
    let (_, rest) = source
        .split_once(&marker)
        .expect("production module interface function");
    let (_, rest) = rest.split_once('{').expect("function body");
    let mut depth = 1;
    for (index, ch) in rest.char_indices() {
        match ch {
            '{' => depth += 1,
            '}' => depth -= 1,
            _ => (),
        }
        if depth == 0 {
            return &rest[..index];
        }
    }
    panic!("unterminated module interface function")
}

fn activity_array<'a>(source: &'a str, function: &str) -> &'a str {
    let body = function_body(source, function);
    if let Some((_, rest)) = body.split_once("static const lxp_module_iface ") {
        let (_, rest) = rest.split_once('{').expect("interface initializer");
        let fields: Vec<_> = rest
            .split_once('}')
            .expect("interface terminator")
            .0
            .split(',')
            .collect();
        return initializer(source, fields[3].trim());
    }
    let (_, call) = body
        .split_once("return ")
        .expect("interface forwarding function");
    let (callee, _) = call.split_once('(').expect("interface forwarding call");
    assert_ne!(callee.trim(), function, "recursive interface forwarding");
    activity_array(source, callee.trim())
}

pub fn production_registry() -> ModuleRegistry {
    let mut registrations = Vec::new();
    for entry in initializer(GENESIS, "module_table").split('{').skip(1) {
        let (entry, _) = entry.split_once('}').expect("genesis module row");
        let fields: Vec<_> = entry.split(',').map(str::trim).collect();
        assert_eq!(fields.len(), 4, "production genesis module row shape");
        let module_name = fields[0];
        let module = ModuleId::from_u16(
            u16::try_from(constant(MODULES, module_name)).expect("module id width"),
        )
        .expect("known production module");
        let (_, source, header) = SOURCES
            .iter()
            .find(|(name, _, _)| *name == module_name)
            .expect("new production module needs an embedded definition");
        let activities: Vec<_> = activity_array(source, fields[3])
            .split(',')
            .map(str::trim)
            .filter(|token| !token.is_empty())
            .map(|token| {
                let value = if token.starts_with("0x") {
                    number(token)
                } else {
                    constant(header, token)
                };
                ActivityType::from_u32(value).expect("production activity type")
            })
            .collect();
        registrations.push(
            ModuleRegistration::new(module, &activities)
                .expect("exact production activity registration"),
        );
    }
    assert!(
        !registrations.is_empty(),
        "production genesis module table is empty"
    );
    registrations.sort_by_key(ModuleRegistration::module);
    ModuleRegistry::new(&registrations).expect("unique production modules")
}
