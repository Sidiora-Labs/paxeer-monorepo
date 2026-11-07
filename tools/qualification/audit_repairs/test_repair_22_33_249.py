"""Offline source proof for finding 018; no Rust/Wasm execution is claimed."""

import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "programs/sdk/rust"
QUICKSTART = SDK / "quickstart"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_249.kvx"


def source(path):
    """Exclude comments so documentation cannot satisfy executable facts."""
    return re.sub(r"//[^\n]*|/\*.*?\*/", "", path.read_text(), flags=re.S)


def compact(text):
    return re.sub(r"\s+", "", text)


def body(text, declaration):
    """Extract balanced Rust declaration bodies from these production sources."""
    match = re.search(declaration + r"[^{}]*\{", text)
    assert match is not None, declaration
    start = match.end()
    depth = 1
    for offset in range(start, len(text)):
        depth += (text[offset] == "{") - (text[offset] == "}")
        if depth == 0:
            return text[start:offset]
    raise AssertionError(f"Unclosed declaration: {declaration}")


def manifest():
    return json.loads((QUICKSTART / "layerx-program.json").read_text())


def sdk_capacity():
    match = re.search(
        r"pub const MAX_CALL_INPUT_BYTES: usize = ([0-9_]+);",
        source(SDK / "src/abi.rs"),
    )
    assert match is not None
    return int(match.group(1).replace("_", ""))


def test_018_metadata_matches_real_sdk_buffer_capacity():
    metadata = manifest()
    entry = compact(source(SDK / "src/entry.rs"))
    capacity = sdk_capacity()
    assert type(metadata["entry"]["call_input_capacity"]) is int
    assert metadata["entry"]["call_input_capacity"] == capacity == 1_048_576
    assert "usecrate::abi::MAX_CALL_INPUT_BYTES;" in entry
    assert "pubconstCALL_INPUT_CAPACITY:usize=MAX_CALL_INPUT_BYTES;" in entry
    assert "structCallBuffer(UnsafeCell<[u8;CALL_INPUT_CAPACITY]>);" in entry
    assert (
        "staticCALL_INPUT:CallBuffer="
        "CallBuffer(UnsafeCell::new([0;CALL_INPUT_CAPACITY]));"
    ) in entry
    assert compact(body(source(SDK / "src/entry.rs"), r"fn reservation\b")) == (
        "CALL_INPUT.0.get().cast::<u8>()"
    )
    assert "const_:()=assert!(CALL_INPUT_CAPACITY<=i32::MAXasusize);" in entry


def test_018_quickstart_exports_delegate_to_sdk_reservation_and_consumer():
    cargo = tomllib.loads((QUICKSTART / "Cargo.toml").read_text())
    assert cargo["lib"]["crate-type"] == ["cdylib"]
    dependency = cargo["dependencies"]["layerx-program-sdk"]
    assert (QUICKSTART / dependency["path"]).resolve() == SDK
    assert "layerx_program_sdk::entrypoint!(note_call);" in source(
        QUICKSTART / "src/lib.rs"
    )
    macro = body(source(SDK / "src/macros.rs"), r"macro_rules! entrypoint\b")
    assert re.search(
        r'#\[no_mangle\]\s*pub extern "C" fn layerx_reserve\(length: i32\) -> i32',
        macro,
    )
    assert compact(body(macro, r"fn layerx_reserve\b")) == (
        "$crate::entry::reserve_call_input(length)"
    )
    assert re.search(
        r'#\[no_mangle\]\s*pub extern "C" fn layerx_call'
        r'\(input_pointer: i32, input_length: i32\) -> i32',
        macro,
    )
    call = compact(body(macro, r"fn layerx_call\b"))
    assert "$crate::entry::with_call_input(input_pointer,input_length,handler)" in call
    assert "= $handler;" in macro
    metadata = manifest()
    assert metadata["entry"]["reserve"] == "layerx_reserve"
    assert metadata["entry"]["call"] == "layerx_call"
    exports = {export["name"]: export for export in metadata["exports"]}
    assert exports["layerx_reserve"] == {
        "name": "layerx_reserve", "params": ["i32"], "results": ["i32"]
    }
    assert exports["layerx_call"] == {
        "name": "layerx_call", "params": ["i32", "i32"], "results": ["i32"]
    }


def test_018_reservation_source_admits_capacity_and_refuses_one_byte_above():
    entry = source(SDK / "src/entry.rs")
    reserve = compact(body(entry, r"fn reserve_call_input\b"))
    # Pin the complete production branch, including signed-length refusal and
    # pointer conversion. The strict comparison proves the inclusive source bound.
    assert reserve == compact("""
        let Ok(requested) = usize::try_from(length) else {
            return RESERVATION_REFUSED;
        };
        if requested > CALL_INPUT_CAPACITY {
            return RESERVATION_REFUSED;
        }
        i32::try_from(reservation() as usize).unwrap_or(RESERVATION_REFUSED)
    """)
    assert "constRESERVATION_REFUSED:i32=-1;" in compact(entry)
    declared = manifest()["entry"]["call_input_capacity"]
    assert declared == sdk_capacity()
    assert declared + 1 > sdk_capacity()
    assert 0 < declared < declared + 1 <= 2**31 - 1


def test_018_call_consumer_preserves_capacity_and_pointer_checks():
    consumer = compact(body(source(SDK / "src/entry.rs"), r"fn with_call_input\b"))
    assert consumer == compact("""
        let requested = usize::try_from(length)
            .map_err(|_| ProgramError::value(Field::CallInput, Reason::Malformed))?;
        if requested > CALL_INPUT_CAPACITY {
            return Err(ProgramError::value(Field::CallInput, Reason::TooLarge));
        }
        let base = reservation();
        let offset = usize::try_from(pointer)
            .map_err(|_| ProgramError::value(Field::CallInput, Reason::Malformed))?;
        if requested > 0 && offset != base as usize {
            return Err(ProgramError::value(Field::CallInput, Reason::Malformed));
        }
        let input = unsafe { core::slice::from_raw_parts(base.cast_const(), requested) };
        Ok(handler(input))
    """)


def test_018_record_maps_source_proof_and_discloses_runtime_limitations():
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["018"]
    assert record["task"]["id"] == "22.33.249"
    assert finding["audit"] == "22.30.129"
    assert finding["file"] == "programs/sdk/rust/quickstart/layerx-program.json"
    assert finding["tests"] == [
        test_018_metadata_matches_real_sdk_buffer_capacity.__name__,
        test_018_quickstart_exports_delegate_to_sdk_reservation_and_consumer.__name__,
        test_018_reservation_source_admits_capacity_and_refuses_one_byte_above.__name__,
        test_018_call_consumer_preserves_capacity_and_pointer_checks.__name__,
    ]
    assert finding["status"] == "source-repaired"
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert record["qualification"]["baseline_executed"] is False
    assert "not executed" in record["proof"]["review_only"]
    assert "domain 13" in record["proof"]["wiring_request"]
    assert "target/wasm32-unknown-unknown/release/layerx_quickstart.wasm" in (
        record["proof"]["wiring_request"]
    )
