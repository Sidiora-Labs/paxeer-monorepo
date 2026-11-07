//! Repair record 22.31.265: findings 004 through 009 are mapped to the tests below.
//! Baseline verification was not run: the task's ordered gate contract forbids it.
//! No out-of-scope wiring changes are needed; the runtime host is exercised unchanged.

use layerx_program_sdk::{call::CallInput, BindingGenerator};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::OnceLock;

const LIMIT: usize = 1_048_576;
const MAX_VALUE: usize = LIMIT - 10;
const SCHEMA_MAX: u32 = 1_048_570;

fn write(path: impl AsRef<Path>, contents: impl AsRef<[u8]>) {
    fs::write(path.as_ref(), contents)
        .unwrap_or_else(|error| panic!("write {}: {error}", path.as_ref().display()));
}

fn run(command: &mut Command) {
    let output = command
        .output()
        .unwrap_or_else(|error| panic!("start {command:?}: {error}"));
    assert!(
        output.status.success(),
        "{command:?}: {}\nstdout:\n{}\nstderr:\n{}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr),
    );
}

fn interface() -> Vec<u8> {
    let mut bytes = b"LayerX/program-interface/v1\0".to_vec();
    bytes.extend_from_slice(&[0x5a; 32]);
    bytes.extend_from_slice(&2_u16.to_be_bytes());
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&4_u16.to_be_bytes());
    bytes.extend_from_slice(b"blob");
    bytes.extend_from_slice(&[1, 2, 3, 4]);
    for _ in 0..2 {
        bytes.extend_from_slice(&[1, 0x20]);
        bytes.extend_from_slice(&SCHEMA_MAX.to_be_bytes());
    }
    // No grants or preconditions; one typed failure with the same output schema.
    bytes.extend_from_slice(&[0, 0, 0, 0, 0, 1]);
    bytes.extend_from_slice(&7_u32.to_be_bytes());
    bytes.extend_from_slice(&7_u16.to_be_bytes());
    bytes.extend_from_slice(b"refused");
    bytes.extend_from_slice(&[1, 0x20]);
    bytes.extend_from_slice(&SCHEMA_MAX.to_be_bytes());
    bytes
}

struct Fixture {
    root: PathBuf,
    generator: BindingGenerator,
    host: PathBuf,
}

fn fixture() -> &'static Fixture {
    static FIXTURE: OnceLock<Fixture> = OnceLock::new();
    FIXTURE.get_or_init(|| {
        let root = std::env::temp_dir().join(format!("layerx-repair-22-31-265-{}", std::process::id()));
        fs::create_dir_all(root.join("host/src")).unwrap();
        let programs = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../..")
            .canonicalize()
            .unwrap();
        // A separate harness avoids adding a runtime dependency to the guest SDK.
        write(root.join("host/Cargo.toml"), format!(
            "[package]\nname = \"repair-265-host\"\nversion = \"0.0.0\"\nedition = \"2021\"\n[workspace]\n[dependencies]\nlayerx-programs-runtime = {{ path = {:?} }}\n[profile.dev]\ndebug = 0\n",
            programs.join("crates/layerx-programs-runtime")
        ));
        write(root.join("host/src/main.rs"), HOST);
        run(Command::new("cargo")
            .current_dir(&programs)
            .args(["build", "--offline", "-j", "2", "--manifest-path"])
            .arg(root.join("host/Cargo.toml"))
            .arg("--target-dir")
            .arg(root.join("host/target")));
        let generator = BindingGenerator::from_interface(&interface()).unwrap();
        write(root.join("client.py"), generator.generate_python());
        run(Command::new("python3").current_dir(&root).args(["-c", "import client as c; from pathlib import Path; Path('output.bin').write_bytes(c._frame(c._Entry_01020304_OUTPUT, bytes([0x5a])*1048570))"]));
        Fixture {
            host: root.join("host/target/debug/repair-265-host"),
            root,
            generator,
        }
    })
}

fn directory(language: &str) -> (&'static Fixture, PathBuf) {
    let fixture = fixture();
    let directory = fixture.root.join(language);
    fs::create_dir_all(&directory).unwrap();
    (fixture, directory)
}

fn transport(fixture: &Fixture, directory: &Path) {
    let bytes = fs::read(directory.join("call.bin")).unwrap();
    assert_eq!(bytes.len(), LIMIT);
    assert_eq!(&bytes[..6], &[1, 2, 3, 4, 1, 0x20]);
    assert_eq!(&bytes[6..10], &(MAX_VALUE as u32).to_be_bytes());
    assert!(bytes[10..].iter().all(|byte| *byte == 0x5a));
    assert_eq!(CallInput::new(&bytes).unwrap().bytes(), bytes);
    let mut too_long = bytes.clone();
    too_long.push(0x5a);
    assert!(CallInput::new(&too_long).is_err());
    run(Command::new(&fixture.host).arg(directory.join("call.bin")));
    let output = fs::read(directory.join("output.bin")).unwrap();
    assert_eq!(output.len(), LIMIT);
    assert_eq!(&output[..2], &[1, 0x20]);
    assert_eq!(&output[2..6], &SCHEMA_MAX.to_be_bytes());
    assert!(output[6..].iter().all(|byte| *byte == 0x5a));
}

// Finding 004, with TypeScript as the already-correct cross-language reference.
#[test]
fn finding_004_rust_and_typescript_complete_call_budget() {
    let (fixture, directory) = directory("rust");
    write(
        directory.join("main.rs"),
        format!("{}\n{}", fixture.generator.generate_rust(), RUST),
    );
    run(Command::new("rustc").current_dir(&directory).args([
        "--edition=2021",
        "main.rs",
        "-o",
        "consumer",
    ]));
    run(Command::new(directory.join("consumer")).current_dir(&directory));
    transport(fixture, &directory);

    let (_, directory) = self::directory("typescript");
    write(
        directory.join("main.ts"),
        format!(
            "{}\n{}",
            fixture.generator.generate_typescript(),
            TYPESCRIPT
        ),
    );
    run(Command::new("tsc").current_dir(&directory).args([
        "--strict", "--target", "ES2022", "--module", "commonjs", "main.ts",
    ]));
    run(Command::new("node").current_dir(&directory).arg("main.js"));
    transport(fixture, &directory);
    assert_eq!(
        fs::read(directory.join("call.bin")).unwrap(),
        fs::read(fixture.root.join("rust/call.bin")).unwrap()
    );
}

#[test]
fn finding_005_go_complete_call_budget() {
    let (fixture, directory) = directory("go");
    write(directory.join("go.mod"), "module repair265\n\ngo 1.23\n");
    write(directory.join("client.go"), fixture.generator.generate_go());
    write(directory.join("client_test.go"), GO);
    run(Command::new("go").current_dir(&directory).args([
        "test",
        "-count=1",
        "-run",
        "^TestCompleteCallBudget$",
    ]));
    transport(fixture, &directory);
}

#[test]
fn finding_006_java_and_kotlin_complete_call_budget() {
    let (fixture, directory) = directory("java");
    write(
        directory.join("ProgramBindings.java"),
        fixture.generator.generate_java(),
    );
    write(directory.join("Consumer.java"), JAVA);
    run(Command::new("javac")
        .current_dir(&directory)
        .args(["ProgramBindings.java", "Consumer.java"]));
    run(Command::new("java")
        .current_dir(&directory)
        .args(["-cp", ".", "Consumer"]));
    transport(fixture, &directory);

    let (_, directory) = self::directory("kotlin");
    write(
        directory.join("Client.kt"),
        fixture.generator.generate_kotlin(),
    );
    write(directory.join("Consumer.kt"), KOTLIN);
    run(Command::new("kotlinc").current_dir(&directory).args([
        "Client.kt",
        "Consumer.kt",
        "-include-runtime",
        "-d",
        "consumer.jar",
    ]));
    run(Command::new("java")
        .current_dir(&directory)
        .args(["-jar", "consumer.jar"]));
    transport(fixture, &directory);
}

#[test]
fn finding_007_python_complete_call_budget() {
    let (fixture, directory) = directory("python");
    write(
        directory.join("client.py"),
        fixture.generator.generate_python(),
    );
    write(directory.join("consumer.py"), PYTHON);
    run(Command::new("python3")
        .current_dir(&directory)
        .arg("consumer.py"));
    transport(fixture, &directory);
}

#[test]
fn finding_008_swift_complete_call_budget() {
    let (fixture, directory) = directory("swift");
    write(
        directory.join("main.swift"),
        format!("{}\n{}", fixture.generator.generate_swift(), SWIFT),
    );
    run(Command::new("swiftc")
        .current_dir(&directory)
        .args(["main.swift", "-o", "consumer"]));
    run(Command::new(directory.join("consumer")).current_dir(&directory));
    transport(fixture, &directory);
}

#[test]
fn finding_009_csharp_complete_call_budget() {
    let (fixture, directory) = directory("csharp");
    write(
        directory.join("Client.cs"),
        fixture.generator.generate_csharp(),
    );
    write(directory.join("Program.cs"), CSHARP);
    write(directory.join("Consumer.csproj"), "<Project Sdk=\"Microsoft.NET.Sdk\"><PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net8.0</TargetFramework><EnableDefaultCompileItems>true</EnableDefaultCompileItems></PropertyGroup></Project>");
    run(Command::new("dotnet").current_dir(&directory).args([
        "run",
        "--project",
        "Consumer.csproj",
    ]));
    transport(fixture, &directory);
}

const RUST: &str = r#"
fn main() {
    let input = blob::Input::new(vec![0x5a; MAX_CALLDATA_BYTES - 10]).unwrap();
    let call = blob::call(&input, CODE_HASH, INTERFACE_DIGEST).unwrap();
    assert_eq!(call.as_bytes().len(), MAX_CALLDATA_BYTES);
    assert_eq!(blob::decode_output(&call.as_bytes()[4..]).unwrap().as_slice(), input.as_slice());
    std::fs::write("call.bin", call.as_bytes()).unwrap();
    for extra in 1..=4 {
        let input = blob::Input::new(vec![0x5a; MAX_CALLDATA_BYTES - 10 + extra]).unwrap();
        assert_eq!(blob::call(&input, CODE_HASH, INTERFACE_DIGEST).unwrap_err(), BindingRefusal::Encode(CodecError::TooLong));
    }
    let output = blob::Output::new(vec![0x5a; MAX_CALLDATA_BYTES - 6]).unwrap();
    let mut frame = vec![1];
    output.encode(&mut frame).unwrap();
    assert_eq!(frame.len(), MAX_CALLDATA_BYTES);
    assert_eq!(blob::decode_output(&frame).unwrap(), output);
    assert!(matches!(blob::decode_failure(7, &frame).unwrap(), blob::Failure::Refused(_)));
    std::fs::write("output.bin", frame).unwrap();
}
"#;

const TYPESCRIPT: &str = r#"
declare function require(name:string): { writeFileSync(path:string, bytes:Uint8Array):void; readFileSync(path:string):Uint8Array };
const fs = require('fs');
const input = boundedBytes(1048570, new Uint8Array(1048566).fill(0x5a));
const call = encodeBlob(input, CODE_HASH, INTERFACE_DIGEST);
if (call.bytes.length !== 1048576 || decodeBlobOutput(call.bytes.slice(4)).length !== input.length) throw new Error('maximum call');
fs.writeFileSync('call.bin', call.bytes);
for (let extra=1; extra<=4; extra++) {
    let refused=false;
    try { encodeBlob(boundedBytes(1048570,new Uint8Array(1048566+extra).fill(0x5a)),CODE_HASH,INTERFACE_DIGEST); }
    catch(error) { if (!(error instanceof BindingRefusal) || error.code !== 'INVALID_VALUE') throw error; refused=true; }
    if (!refused) throw new Error('oversized call accepted');
}
const outputFrame=fs.readFileSync('../output.bin');
if (outputFrame.length!==1048576 || decodeBlobOutput(outputFrame).length!==1048570) throw new Error('output budget');
decodeBlobFailure(7,outputFrame);
fs.writeFileSync('output.bin',outputFrame);
"#;

const GO: &str = r#"
package bindings
import ("testing"; "os"; "bytes")
func TestCompleteCallBudget(t *testing.T) {
    input,e:=NewLxblobD01020304Input(bytes.Repeat([]byte{0x5a},MaxCalldataBytes-10)); if e!=nil {t.Fatal(e)}
    call,e:=EncodeLxblobD01020304(input,CodeHash(),InterfaceDigest()); if e!=nil {t.Fatal(e)}
    data,e:=call.Bytes(); if e!=nil {t.Fatal(e)}; if len(data)!=MaxCalldataBytes {t.Fatal("maximum call")}
    output,e:=DecodeLxblobD01020304Output(data[4:]); if e!=nil || !bytes.Equal(output.Value(),input.Value()) {t.Fatal("roundtrip",e)}
    if e=os.WriteFile("call.bin",data,0600);e!=nil {t.Fatal(e)}
    for extra:=1;extra<=4;extra++ {
        input,e:=NewLxblobD01020304Input(bytes.Repeat([]byte{0x5a},MaxCalldataBytes-10+extra));if e!=nil {t.Fatal(e)}
        call,e:=EncodeLxblobD01020304(input,CodeHash(),InterfaceDigest()); if e!=ErrInvalidValue || call!=nil {t.Fatal("oversized call accepted",e)}
    }
    output,e=NewLxblobD01020304Output(bytes.Repeat([]byte{0x5a},MaxCalldataBytes-6));if e!=nil {t.Fatal(e)}
    w:=writer{};if e=w.byte(1);e!=nil {t.Fatal(e)};if e=encodeLxblobD01020304Output(&w,output);e!=nil {t.Fatal(e)}
    if len(w.data)!=MaxCalldataBytes {t.Fatal("output frame size")}
    decoded,e:=DecodeLxblobD01020304Output(w.data);if e!=nil || !bytes.Equal(decoded.Value(),output.Value()) {t.Fatal("output budget",e)}
    if _,e=DecodeLxblobD01020304Failure(7,w.data);e!=nil {t.Fatal(e)}
    if e=os.WriteFile("output.bin",w.data,0600);e!=nil {t.Fatal(e)}
}
"#;

const JAVA: &str = r#"
import java.nio.file.*;
import java.util.Arrays;
public class Consumer {
    static byte[] value(int n) { byte[] b=new byte[n];Arrays.fill(b,(byte)0x5a);return b; }
    public static void main(String[] args) throws Exception {
        var input=new ProgramBindings.Entry01020304.Input(value(1048566));
        var call=ProgramBindings.Entry01020304.call(input,ProgramBindings.CODE_HASH,ProgramBindings.INTERFACE_DIGEST);
        if(call.bytes().length!=1048576 || !Arrays.equals(ProgramBindings.Entry01020304.decodeOutput(Arrays.copyOfRange(call.bytes(),4,call.bytes().length)).value(),input.value()))throw new AssertionError("maximum call");
        Files.write(Path.of("call.bin"),call.bytes());
        for(int extra=1;extra<=4;extra++) {
            input=new ProgramBindings.Entry01020304.Input(value(1048566+extra));
            boolean refused=false;
            try {ProgramBindings.Entry01020304.call(input,ProgramBindings.CODE_HASH,ProgramBindings.INTERFACE_DIGEST);}
            catch(ProgramBindings.BindingRefusal e) {if(!e.code.equals("INVALID_VALUE"))throw e;refused=true;}
            if(!refused)throw new AssertionError("oversized call accepted");
        }
        var output=new ProgramBindings.Entry01020304.Output(value(1048570));
        byte[] frame=Files.readAllBytes(Path.of("../output.bin"));
        if(frame.length!=1048576 || !Arrays.equals(ProgramBindings.Entry01020304.decodeOutput(frame).value(),output.value()))throw new AssertionError("output budget");
        ProgramBindings.Entry01020304.decodeFailure(7,frame);
        Files.write(Path.of("output.bin"),frame);
    }
}
"#;

const KOTLIN: &str = r#"
import java.nio.file.Files
import java.nio.file.Path
fun main() {
    fun value(n:Int)=ByteArray(n){0x5a.toByte()}
    val input=ProgramBindings.Entry01020304.Input(value(1048566))
    val call=ProgramBindings.Entry01020304.call(input,ProgramBindings.CODE_HASH,ProgramBindings.INTERFACE_DIGEST)
    check(call.bytes.size==1048576)
    check(ProgramBindings.Entry01020304.decodeOutput(call.bytes.copyOfRange(4,call.bytes.size)).value.contentEquals(input.value))
    Files.write(Path.of("call.bin"),call.bytes)
    for(extra in 1..4) {
        val oversized=ProgramBindings.Entry01020304.Input(value(1048566+extra))
        var refused=false
        try {ProgramBindings.Entry01020304.call(oversized,ProgramBindings.CODE_HASH,ProgramBindings.INTERFACE_DIGEST)}
        catch(e:ProgramBindings.BindingRefusal) {check(e.code=="INVALID_VALUE");refused=true}
        check(refused){"oversized call accepted"}
    }
    val output=ProgramBindings.Entry01020304.Output(value(1048570))
    val frame=Files.readAllBytes(Path.of("../output.bin"))
    check(frame.size==1048576)
    check(ProgramBindings.Entry01020304.decodeOutput(frame).value.contentEquals(output.value))
    ProgramBindings.Entry01020304.decodeFailure(7,frame)
    Files.write(Path.of("output.bin"),frame)
}
"#;

const PYTHON: &str = r#"
from pathlib import Path
import client as c
input=c.Entry_01020304Input(bytes([0x5a])*1048566)
call=c.encode_entry_blob_01020304(input,c.CODE_HASH,c.INTERFACE_DIGEST)
assert len(call.as_bytes())==1048576
assert call.decode_output(call.as_bytes()[4:])==input.value
Path('call.bin').write_bytes(call.as_bytes())
for extra in range(1,5):
    oversized=c.Entry_01020304Input(bytes([0x5a])*(1048566+extra))
    try:
        c.encode_entry_blob_01020304(oversized,c.CODE_HASH,c.INTERFACE_DIGEST)
    except c.BindingRefusal as error:
        assert error.code=='INVALID_VALUE'
    else:
        raise AssertionError('oversized call accepted')
output=bytes([0x5a])*1048570
frame=c._frame(c._Entry_01020304_OUTPUT,output)
assert len(frame)==1048576
assert c.decode_entry_blob_01020304_output(frame)==output
assert c.decode_entry_blob_01020304_failure(7,frame).detail==output
Path('output.bin').write_bytes(frame)
"#;

const SWIFT: &str = r#"
import Foundation
let input = try Lx_blob_01020304.Input([UInt8](repeating: 0x5a, count: 1_048_566))
let call = try Lx_blob_01020304.call(input, deployedCodeHash: layerXCodeHash, publishedDigest: layerXInterfaceDigest)
precondition(call.bytes.count == 1_048_576)
let roundtrip = try Lx_blob_01020304.decodeOutput(Array(call.bytes.dropFirst(4)))
precondition(roundtrip.bytes == input.bytes)
try Data(call.bytes).write(to: URL(fileURLWithPath: "call.bin"))
for extra in 1...4 {
    let oversized = try Lx_blob_01020304.Input([UInt8](repeating: 0x5a, count: 1_048_566 + extra))
    var refused = false
    do { _ = try Lx_blob_01020304.call(oversized, deployedCodeHash: layerXCodeHash, publishedDigest: layerXInterfaceDigest) }
    catch LxBindingRefusal.tooLong { refused = true }
    precondition(refused, "oversized call accepted")
}
let output = try Lx_blob_01020304.Output([UInt8](repeating: 0x5a, count: 1_048_570))
fileprivate var writer = LxWriter(1)
try Lx_blob_01020304.encodeOutput(output, &writer)
precondition(writer.bytes.count == 1_048_576)
let decoded = try Lx_blob_01020304.decodeOutput(writer.bytes)
precondition(decoded == output)
_ = try Lx_blob_01020304.decodeFailure(code: 7, bytes: writer.bytes)
try Data(writer.bytes).write(to: URL(fileURLWithPath: "output.bin"))
"#;

const CSHARP: &str = r#"
using System;
using System.IO;
using System.Linq;
public static class Program {
    static byte[] Value(int size) => Enumerable.Repeat((byte)0x5a,size).ToArray();
    public static void Main() {
        var input=LayerXBindings.EntryBlob.Input.From(Value(1048566));
        var call=LayerXBindings.EntryBlob.Encode(input,LayerXBindings.CodeHash,LayerXBindings.InterfaceDigest);
        if(call.Bytes.Length!=1048576 || !LayerXBindings.EntryBlob.DecodeOutput(call.Bytes.Skip(4).ToArray()).Value.SequenceEqual(input.Value))throw new Exception("maximum call");
        File.WriteAllBytes("call.bin",call.Bytes);
        for(int extra=1;extra<=4;extra++) {
            var oversized=LayerXBindings.EntryBlob.Input.From(Value(1048566+extra));
            bool refused=false;
            try {LayerXBindings.EntryBlob.Encode(oversized,LayerXBindings.CodeHash,LayerXBindings.InterfaceDigest);}
            catch(LayerXBindings.BindingRefusal e) {if(e.Code!=LayerXBindings.Refusal.InvalidValue)throw;refused=true;}
            if(!refused)throw new Exception("oversized call accepted");
        }
        var output=LayerXBindings.EntryBlob.Output.From(Value(1048570));
        var frame=File.ReadAllBytes("../output.bin");
        if(frame.Length!=1048576 || !LayerXBindings.EntryBlob.DecodeOutput(frame).Value.SequenceEqual(output.Value))throw new Exception("output budget");
        LayerXBindings.EntryBlob.DecodeFailure(7,frame);
        File.WriteAllBytes("output.bin",frame);
    }
}
"#;

// This executable imports the actual runtime and drives its unchanged Wasm host linkers.
// A valid call reaches authorization (DENIED without composition authority); an oversized
// transport length is refused by the host's input bound first (BOUNDS), for both ABIs.
const HOST: &str = r#"
use layerx_programs_runtime::test_support::*;
use layerx_programs_runtime::{Abi,AuthorizationContext,CapabilitySet,FeeSchedule,Meter,PrincipalId,ProgramId,ResourceBudget,Storage,UnavailableReceiptOracle,WasmEngine,WasmValue};
fn name(out:&mut Vec<u8>,text:&str){out.extend(unsigned_leb(text.len() as u64));out.extend_from_slice(text.as_bytes());}
fn main(){
    let bytes=std::fs::read(std::env::args().nth(1).unwrap()).unwrap();
    assert_eq!(bytes.len(),1_048_576);
    for version in [1,2] {
        let response=version==2;
        let params=if response{8}else{6};
        let result=if response{TYPE_I64}else{TYPE_I32};
        let mut exports=vec![3];name(&mut exports,"run");exports.extend([0,1]);name(&mut exports,"write");exports.extend([0,2]);name(&mut exports,"memory");exports.extend([2,0]);
        let mut body=vec![0x41,0,0x41,32,0x41,0xc0,0,0x20,0,0x41,32,0x41,2];
        if response {body.extend([0x41,0,0x41,0]);}
        body.extend([0x10,0,0x0b]);
        let mut data=vec![1,0,0x41,0,0x0b,32];data.extend([0x11;32]);
        let mut write_params=vec![TYPE_I32];write_params.extend([TYPE_I64;8]);
        let mut write_body=Vec::new();
        for index in 0..8 {write_body.extend([0x20,0,0x20,index+1,0x37,3,index*8]);}
        write_body.push(0x0b);
        let wasm=module(&[
            type_section(&[(&vec![TYPE_I32;params],&[result]),(&[TYPE_I32],&[result]),(&write_params,&[])]),
            import_section(&[(if response{"layerx_v2"}else{"layerx_v1"},if response{"program_call_response"}else{"program_call"},0)]),
            function_section(&[1,2]),raw_section(5,&[1,1,17,17]),raw_section(7,&exports),code_section(&[func_body(&[],&body),func_body(&[],&write_body)]),raw_section(11,&data)
        ]);
        let engine=WasmEngine::declared().unwrap();
        let validated=if response{engine.validate_v2(&wasm)}else{engine.validate(&wasm)}.unwrap();
        for (length,status) in [(1_048_576,-1),(1_048_577,-3)] {
            let abi=Abi::new(version,ProgramId::new([0x22;32]).unwrap(),AuthorizationContext::new(PrincipalId::new([0x33;32]).unwrap(),CapabilitySet::empty()),Storage::new(),&UnavailableReceiptOracle).unwrap();
            let mut instance=validated.instantiate_sandbox(Meter::new(ResourceBudget::declared(),FeeSchedule::declared()),abi).unwrap();
            // Transport the actual producer bytes through an exported memory writer;
            // embedding 1 MiB of data would exceed the separate module-size bound.
            for (index,chunk) in bytes.chunks_exact(64).enumerate() {
                let mut args=vec![WasmValue::I32((64+index*64) as i32)];
                args.extend(chunk.chunks_exact(8).map(|word|WasmValue::I64(i64::from_le_bytes(word.try_into().unwrap()))));
                assert!(instance.call("write",&args).unwrap().is_empty());
            }
            let values=instance.call("run",&[WasmValue::I32(length)]).unwrap();
            match values.as_slice(){[WasmValue::I32(actual)]=>assert_eq!(*actual,status),[WasmValue::I64(actual)]=>assert_eq!(*actual,i64::from(status)),other=>panic!("unexpected {other:?}")}
        }
    }
}
"#;
