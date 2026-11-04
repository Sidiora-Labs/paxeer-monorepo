use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, OP_CALL, OP_END, OP_LOCAL_GET, TYPE_I32,
};

pub fn program_importing(import_module: &str, import_name: &str, arity: u8) -> Vec<u8> {
    let params = vec![TYPE_I32; usize::from(arity)];
    let mut call = Vec::new();
    for index in 0..arity {
        call.extend_from_slice(&[OP_LOCAL_GET, index % 2]);
    }
    call.extend_from_slice(&[OP_CALL, 0, OP_END]);

    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_call", 0, 1),
        ("layerx_reserve", 0, 2),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index));
    }

    module(&[
        type_section(&[
            (&params, &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
        ]),
        import_section(&[(import_module, import_name, 0)]),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &call),
            func_body(&[], &[OP_LOCAL_GET, 0, OP_END]),
        ]),
    ])
}
