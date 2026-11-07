#[path = "../tests/support/repair_478_fixture.rs"]
mod fixture;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}
fn fixed(text: &str) -> [u8; 32] {
    assert_eq!(text.len(), 64);
    core::array::from_fn(|i| u8::from_str_radix(&text[2 * i..2 * i + 2], 16).unwrap())
}
fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args[1] == "--terminal" {
        use layerx_programs_runtime::terminal::*;
        let bytes = std::fs::read(&args[3]).expect("read terminal");
        let terminal = fixture::decode_kernel_terminal(args[2].parse().unwrap(), &bytes)
            .expect("canonical kernel terminal");
        match terminal.detail {
            TerminalDetail::Execution(ExecutionTerminal::CandidateV4 {
                outcome: CandidateTerminalOutcome::Failure(failure),
                ..
            })
            | TerminalDetail::Failure(FailureTerminal::Program(failure)) => {
                println!(
                    "Fund application refusal: {}",
                    fixture::application_error(failure.reason().bytes())
                );
            }
            TerminalDetail::Failure(FailureTerminal::PreRuntime(failure)) => {
                println!(
                    "Fund kernel refusal: code={} callback_stage=absent",
                    failure.result_code
                );
            }
            TerminalDetail::Failure(FailureTerminal::Callback { stage, status }) => {
                println!("Fund callback refusal: stage={stage} code={status}");
            }
            other => println!("Fund terminal: {other:?}"),
        }
        return;
    }
    if args[1] == "--probe" {
        println!(
            "Fund runtime: {:?}",
            fixture::execute_kernel_fund(std::path::Path::new(&args[2]))
        );
        return;
    }
    let wasm = std::fs::read(&args[3]).expect("read guest artifact");
    layerx_programs_runtime::WasmEngine::declared()
        .expect("deterministic engine")
        .validate_deployment_versioned_metered(
            5,
            &wasm,
            layerx_programs_runtime::FuelSchedule::WASMI_0_31_2,
        )
        .expect("ABI5 deployment admission under the production metering schedule");
    let value = fixture::fixture(fixed(&args[1]), fixed(&args[2])).unwrap();
    println!("{{\"create\":\"{}\",\"fund\":\"{}\",\"current\":\"{}\",\"next\":\"{}\",\"result\":\"{}\",\"event\":\"{}\",\"rewards\":\"{}\"}}", hex(&value.create), hex(&value.fund), hex(&value.current), hex(&value.next), hex(&value.result), hex(&value.event), hex(&value.rewards));
}
