"""Check the mint CLI/YAML contract and execute the production Python verifier.

Shell commands are inspected as source, never replaced with Python commands or
fake paxd/jq executables. Go, Bash and jq execution is outside this Python gate;
the repair record distinguishes those limits from the executed consumer checks.
"""

from pathlib import Path
import re
import runpy
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SUITE = ROOT / "tests/integration/mint_module/mint_test.yaml"
FIELDS = {
    "DENOM": ("denom", "string"),
    "START_DATE": ("start_date", "string"),
    "END_DATE": ("end_date", "string"),
    "TOTAL_MINT_AMOUNT": ("total_mint_amount", "uint64"),
    "REMAINING_MINT_AMOUNT": ("remaining_mint_amount", "uint64"),
    "LAST_MINT_AMOUNT": ("last_mint_amount", "uint64"),
    "LAST_MINT_DATE": ("last_mint_date", "string"),
    "LAST_MINT_HEIGHT": ("last_mint_height", "uint64"),
}
ASSERTIONS = [
    'DENOM == "uhpx"',
    "START_DATE == LAST_MINT_DATE and END_DATE > START_DATE",
    "TOTAL_MINT_AMOUNT == 999999999999",
    "REMAINING_MINT_AMOUNT == 666666666666",
    "LAST_MINT_AMOUNT == 333333333333",
    "LAST_MINT_HEIGHT == 0",
    "TOTAL_SUPPLY == 5000000000333333333333",
]


@pytest.fixture
def runner():
    namespace = runpy.run_path(str(ROOT / "tests/integration/scripts/runner.py"))
    return namespace["TestRunner"]()


@pytest.fixture
def scenario(runner):
    cases = runner.load_yaml_file(SUITE)
    assert len(cases) == 1
    return cases[0]


@pytest.fixture
def consumer_values():
    """Verifier unit inputs, not a purported recording of a paxd response."""
    return {
        "DENOM": "uhpx",
        "START_DATE": "2026-01-01",
        "END_DATE": "2026-01-04",
        "TOTAL_MINT_AMOUNT": "999999999999",
        "REMAINING_MINT_AMOUNT": "666666666666",
        "LAST_MINT_AMOUNT": "333333333333",
        "LAST_MINT_DATE": "2026-01-01",
        "LAST_MINT_HEIGHT": "0",
        "TOTAL_SUPPLY": "5000000000333333333333",
    }


def guarded_pipeline(command):
    # Exact shell form: nonzero query OR jq status prevents any stdout publication,
    # including a CLI that writes valid JSON before reporting failure.
    match = re.fullmatch(
        r'set -o pipefail; value=\$\((.+)\) && printf "%s\\n" "\$value"',
        command,
    )
    assert match is not None, command
    # The Python runner wraps the entire Docker command in single quotes.
    assert "'" not in command
    tokens = shlex.split(match[1])
    separator = tokens.index("|")
    return tokens[:separator], tokens[separator + 1 :]


def test_mint_output_flags_preserves_entire_scenario(scenario):
    assert set(scenario) == {"name", "inputs", "verifiers"}
    assert scenario["name"] == "Mint event triggered"
    assert scenario["inputs"][0] == {"cmd": "sleep 65"}
    assert [step["env"] for step in scenario["inputs"][1:]] == [
        *FIELDS,
        "TOTAL_SUPPLY",
    ]
    assert all(set(step) == {"cmd", "env"} for step in scenario["inputs"][1:])
    assert scenario["verifiers"] == [
        {"type": "eval", "expr": expression} for expression in ASSERTIONS
    ]


@pytest.mark.parametrize("environment", [*FIELDS, "TOTAL_SUPPLY"])
def test_mint_output_flags_guards_query_and_jq(scenario, environment):
    step = next(step for step in scenario["inputs"] if step.get("env") == environment)
    query, consumer = guarded_pipeline(step["cmd"])
    if environment == "TOTAL_SUPPLY":
        assert query == ["paxd", "q", "bank", "total", "--output", "json"]
        assert consumer == [
            "jq",
            "-er",
            "-M",
            '.supply[] | select(.denom=="uhpx").amount',
        ]
    else:
        assert query == ["paxd", "q", "mint", "minter", "--output", "json"]
        assert consumer == ["jq", "-er", "." + FIELDS[environment][0]]
    assert "--json" not in step["cmd"]


def test_mint_output_flags_registered_producer_transport_and_fields(scenario):
    cli = (ROOT / "modules/mint/client/cli/query.go").read_text()
    registration, command = cli.split("func GetCmdQueryEpochProvisions()", 1)
    assert "mintingQueryCmd.AddCommand(" in registration
    assert "GetCmdQueryEpochProvisions()," in registration
    for fact in (
        'Use:   "minter"',
        "Args: cobra.NoArgs",
        "client.GetClientQueryContext(cmd)",
        "types.NewQueryClient(clientCtx)",
        "res, err := queryClient.Minter(context.Background(), params)",
        "if err != nil {\n\t\t\t\treturn err\n\t\t\t}",
        "return clientCtx.PrintProto(res)",
        "flags.AddQueryFlagsToCmd(cmd)",
    ):
        assert fact in command
    flags = (ROOT / "sdk/client/flags/flags.go").read_text()
    query_flags = flags.split("func AddQueryFlagsToCmd(", 1)[1].split(
        "// AddTxFlagsToCmd", 1
    )[0]
    assert (
        'cmd.Flags().StringP(tmcli.OutputFlag, "o", "text", "Output format (text|json)")'
        in query_flags
    )
    command_flags = (ROOT / "sdk/client/cmd.go").read_text()
    assert "output, _ := flagSet.GetString(cli.OutputFlag)" in command_flags
    assert "clientCtx = clientCtx.WithOutputFormat(output)" in command_flags
    keeper = (ROOT / "modules/mint/keeper/grpc_query.go").read_text()
    assert "minter := q.GetMinter(ctx)" in keeper
    assert "response := types.QueryMinterResponse(minter)\n\treturn &response, nil" in keeper
    proto = (ROOT / "api/mint/v1beta1/query.proto").read_text()
    assert "rpc Minter(QueryMinterRequest) returns (QueryMinterResponse)" in proto
    response = proto.split("message QueryMinterResponse {", 1)[1].split("}", 1)[0]
    # Compare by field name: several response fields share the same scalar type.
    declared = {
        name: kind
        for kind, name in re.findall(r"\b(string|uint64)\s+(\w+)\s*=", response)
    }
    assert declared == {name: kind for name, kind in FIELDS.values()}
    generated = (ROOT / "modules/mint/types/query.pb.go").read_text()
    generated_response = generated.split("type QueryMinterResponse struct {", 1)[1].split(
        "}", 1
    )[0]
    for step in scenario["inputs"][1:-1]:
        _, consumer = guarded_pipeline(step["cmd"])
        field = consumer[-1].removeprefix(".")
        assert field in declared
        assert re.search(
            rf'\b{declared[field]}\s+`[^`]*json:"{field},omitempty" yaml:"{field}"',
            generated_response,
        )
    context = (ROOT / "sdk/client/context.go").read_text()
    print_proto = context.split("func (ctx Context) PrintProto(", 1)[1].split(
        "// PrintObjectLegacy", 1
    )[0]
    assert "out, err := ctx.Codec.MarshalAsJSON(toPrint)" in print_proto
    assert "return ctx.printOutput(out)" in print_proto
    assert 'if ctx.OutputFormat == "text" {' in context
    assert "writer.Write(out)" in context
    codec = (ROOT / "sdk/codec/json.go").read_text()
    assert "OrigName: true, EmitDefaults: true" in codec


def test_mint_output_flags_real_consumer_accepts_integer_strings(
    runner, scenario, consumer_values
):
    for verifier in scenario["verifiers"]:
        assert runner.verify_result(consumer_values, verifier)
    assert int(consumer_values["TOTAL_MINT_AMOUNT"]) == (
        int(consumer_values["REMAINING_MINT_AMOUNT"])
        + int(consumer_values["LAST_MINT_AMOUNT"])
    )


@pytest.mark.parametrize("environment", [*FIELDS, "TOTAL_SUPPLY"])
def test_mint_output_flags_real_consumer_rejects_failed_capture(
    runner, scenario, consumer_values, environment
):
    consumer_values[environment] = ""
    relevant = [
        verifier
        for verifier in scenario["verifiers"]
        if environment in verifier["expr"].split()
    ]
    assert relevant
    for verifier in relevant:
        try:
            accepted = runner.verify_result(consumer_values, verifier)
        except (SyntaxError, TypeError, ValueError, NameError):
            accepted = False
        assert accepted is False


@pytest.mark.parametrize(
    "environment,wrong",
    [
        ("DENOM", "other"),
        ("START_DATE", "2026-01-02"),
        ("END_DATE", "2026-01-01"),
        ("LAST_MINT_DATE", "2026-01-02"),
        ("TOTAL_MINT_AMOUNT", "1000000000000"),
        ("REMAINING_MINT_AMOUNT", "666666666667"),
        ("LAST_MINT_AMOUNT", "333333333334"),
        ("LAST_MINT_HEIGHT", "1"),
        ("TOTAL_SUPPLY", "5000000000333333333334"),
    ],
)
def test_mint_output_flags_real_consumer_rejects_wrong_value(
    runner, scenario, consumer_values, environment, wrong
):
    consumer_values[environment] = wrong
    relevant = [
        verifier
        for verifier in scenario["verifiers"]
        if environment in verifier["expr"].split()
    ]
    assert relevant
    assert all(not runner.verify_result(consumer_values, verifier) for verifier in relevant)


def test_mint_output_flags_record_names_proofs_and_limits():
    record = tomllib.loads(Path(__file__).with_name("repair_22_31_422.kvx").read_text())
    task = record["task"]["22"]["31"]["422"]
    assert task["audit"] == "22.30.172"
    assert task["finding"] == "mint-output-flags"
    tests = set(
        re.findall(
            r"(?m)^def (test_mint_output_flags_\w+)\(", Path(__file__).read_text()
        )
    )
    assert set(task["proofs"]) == tests
    assert task["base_revision"] == "84b6cf09552744359c78f419900ba8fcd9bbeaf5"
    assert "not executed" in task["review_only"]
    assert "No pre-change verification" in task["baseline"]
    assert "hf-done" in task["evidence"]
    assert task["wiring_requests"] == []
