use layerx_ai_market_test_values::RequiredValue as _;
#[path = "../tests/support/repair_478_fixture.rs"]
mod fixture;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::new(), |mut text, b| {
        use core::fmt::Write as _;
        write!(text, "{b:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
        text
    })
}
fn fixed(text: &str) -> [u8; 32] {
    assert_eq!(text.len(), 64);
    core::array::from_fn(|i| u8::from_str_radix(&text[2 * i..2 * i + 2], 16).required())
}
fn main() {
    let args: Vec<String> = std::env::args().collect();
    if args[1] == "--terminal" {
        use layerx_programs_runtime::terminal::{
            CandidateTerminalOutcome, ExecutionTerminal, FailureTerminal, TerminalDetail,
        };
        let bytes = std::fs::read(&args[3]).required_with("read terminal");
        let terminal = fixture::decode_kernel_terminal(args[2].parse().required(), &bytes)
            .required_with("canonical kernel terminal");
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
    if args[1] == "--measure" {
        let allowance = args[3].parse().required_with("diagnostic allowance");
        let fuel = fixture::measure_kernel_fund(std::path::Path::new(&args[2]), allowance)
            .required_with("Fund must produce the exact state, result, event and transfer");
        println!("{{\"cpu_fuel\":{fuel}}}");
        return;
    }
    if args[1] == "--probe" {
        println!(
            "Fund runtime: {:?}",
            fixture::execute_kernel_fund(std::path::Path::new(&args[2]))
        );
        return;
    }
    let wasm = std::fs::read(&args[3]).required_with("read guest artifact");
    layerx_programs_runtime::WasmEngine::declared()
        .required_with("deterministic engine")
        .validate_deployment_versioned_metered(
            5,
            &wasm,
            layerx_programs_runtime::FuelSchedule::WASMI_0_31_2,
        )
        .required_with("ABI5 deployment admission under the production metering schedule");
    let value = fixture::fixture(fixed(&args[1]), fixed(&args[2])).required();
    println!("{{\"create\":\"{}\",\"fund\":\"{}\",\"current\":\"{}\",\"next\":\"{}\",\"result\":\"{}\",\"event\":\"{}\",\"rewards\":\"{}\"}}", hex(&value.create), hex(&value.fund), hex(&value.current), hex(&value.next), hex(&value.result), hex(&value.event), hex(&value.rewards));
}
