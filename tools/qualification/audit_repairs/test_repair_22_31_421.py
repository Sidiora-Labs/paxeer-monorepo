"""Execute the script's production query/JSON exchange without a paxd toolchain.

The committed in-memory peer exercises the argument and JSON interfaces. Source
checks bind it to the Go CLI, keeper and protobuf contract; they do not claim an
execution of those Go components or a live chain creation.
"""

import ast
import base64
import json
from pathlib import Path
import re
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "tests/integration/contracts/create_tokenfactory_denoms.sh"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_31_421.kvx"
# Existing tokenfactory genesis fixture, not an invented address spelling.
CREATOR = "pax1y3pxq5dp900czh0mkudhjdqjq5m8cpmmsnt288"
BINARY = "/a directory with spaces/paxd"


def verifier_source():
    return SCRIPT.read_text().split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]


@pytest.fixture
def verifier():
    namespace = {"__name__": "tokenfactory_creator_verifier"}
    exec(compile(verifier_source(), str(SCRIPT), "exec"), namespace)
    return namespace


class CreatorDenomStore:
    """In-memory creator index fixture with the declared CLI/JSON boundary."""

    def __init__(self, page_size, extra=()):
        self.index = {}
        self.requests = []
        self.page_size = page_size
        for number in range(1, 31):
            self.create(CREATOR, str(number))
        for subdenom in extra:
            self.create(CREATOR, subdenom)
        self.create("another-creator", "unrelated")

    def create(self, creator, subdenom):
        self.index.setdefault(creator, set()).add(f"factory/{creator}/{subdenom}")

    def exchange(self, command):
        assert command[:4] == [BINARY, "q", "tokenfactory", "denoms-from-creator"]
        assert command[4] == CREATOR
        assert command[5:9] == ["--output", "json", "--limit", "100"]
        assert len(command) in (9, 11)
        keys = sorted(self.index[command[4]])
        start = 0
        if len(command) == 11:
            assert command[9] == "--page-key"
            # The real SDK takes raw []byte(pageKey), not its base64 spelling.
            start = keys.index(command[10])
        self.requests.append(command)
        end = min(start + self.page_size, len(keys))
        next_key = base64.b64encode(keys[end].encode()).decode() if end < len(keys) else ""
        return json.dumps(
            {"denoms": keys[start:end], "pagination": {"next_key": next_key, "total": "0"}}
        )


def test_tokenfactory_unset_creator_guarded_source_contract():
    parsed = subprocess.run(
        ["bash", "-n", str(SCRIPT)], capture_output=True, text=True, check=False
    )
    assert parsed.returncode == 0, parsed.stderr
    source = SCRIPT.read_text()
    assert "CREATOR_ID" not in source
    assert 'local new_token_denom="factory/$keyaddress/$subdenom"' in source
    assert 'keyaddress=$(printf "12345678\\n" | $paxdbin keys list --output json' in source
    assert re.findall(r"for i in \{(\d+)\.\.(\d+)\}", source) == [
        ("1", "10"),
        ("11", "20"),
        ("21", "30"),
    ]
    assert source.count('new_token_denom=$(create_denom_and_wait "$i") || exit 1') == 3
    assert 'num_denoms=$(python3 - "$paxdbin" "$keyaddress" <<\'PY\'' in source
    assert '\nPY\n) || exit 1\necho "$num_denoms"\n\nexit 0\n' in source
    tree = ast.parse(verifier_source())
    calls = [node for node in ast.walk(tree) if isinstance(node, ast.Call)]
    run = [node for node in calls if ast.unparse(node.func) == "subprocess.run"]
    assert len(run) == 1
    assert [ast.unparse(arg) for arg in run[0].args] == ["command"]
    assert {kw.arg: ast.unparse(kw.value) for kw in run[0].keywords} == {
        "check": "True",
        "stdout": "subprocess.PIPE",
        "text": "True",
    }
    assert "except (OSError, ValueError, subprocess.CalledProcessError) as error:" in verifier_source()
    assert "sys.exit(1)" in verifier_source()
    assert "shell=True" not in verifier_source()


def test_tokenfactory_unset_creator_real_interface_declarations():
    cli = (ROOT / "modules/tokenfactory/client/cli/query.go").read_text()
    creator_cli = cli.split("func GetCmdDenomsFromCreator()", 1)[1].split(
        "// GetCmdDenomMetadata", 1
    )[0]
    for fact in (
        'Use:   "denoms-from-creator [creator address] [flags]"',
        "Args:  cobra.ExactArgs(1)",
        "sdk.AccAddressFromBech32(args[0])",
        "client.ReadPageRequest(cmd.Flags())",
        "Creator:    args[0]",
        "Pagination: pageReq",
        "return clientCtx.PrintProto(res)",
        'flags.AddPaginationFlagsToCmd(cmd, "denoms-from-creator")',
    ):
        assert fact in creator_cli
    utils = (ROOT / "sdk/client/utils.go").read_text()
    assert "pageKey, _ := flagSet.GetString(flags.FlagPageKey)" in utils
    assert "Key:        []byte(pageKey)" in utils
    proto = (ROOT / "api/tokenfactory/query.proto").read_text()
    assert "repeated string denoms = 1" in proto
    assert "cosmos.base.query.v1beta1.PageResponse pagination = 2" in proto
    keeper = (ROOT / "modules/tokenfactory/keeper/grpc_query.go").read_text()
    assert "k.getDenomsFromCreator(sdkCtx, req.GetCreator(), req.GetPagination())" in keeper
    assert "types.QueryDenomsFromCreatorResponse{Denoms: denoms, Pagination: pageRes}" in keeper
    index = (ROOT / "modules/tokenfactory/keeper/creators.go").read_text()
    assert "store := k.GetCreatorPrefixStore(ctx, creator)" in index
    assert "query.Paginate(store, pagination," in index
    assert "denoms = append(denoms, string(key))" in index
    genesis = (ROOT / "modules/tokenfactory/keeper/genesis_test.go").read_text()
    assert f'Admin: "{CREATOR}"' in genesis


@pytest.mark.parametrize("page_size", [1, 7, 30, 100])
@pytest.mark.parametrize("extra", [(), ("preexisting", "0")])
def test_tokenfactory_unset_creator_exchange_all_thirty(verifier, page_size, extra):
    peer = CreatorDenomStore(page_size, extra)
    queries = verifier["creator_queries"](BINARY, CREATOR)
    command = next(queries)
    while True:
        response = peer.exchange(command)
        try:
            command = queries.send(response)
        except StopIteration as complete:
            assert complete.value == 30 + len(extra)
            break
    assert len(peer.requests) == (30 + len(extra) + page_size - 1) // page_size


@pytest.mark.parametrize("missing", range(1, 31))
def test_tokenfactory_unset_creator_rejects_each_missing_denom(verifier, missing):
    peer = CreatorDenomStore(100)
    peer.index[CREATOR].remove(f"factory/{CREATOR}/{missing}")
    # A count-only check would still see thirty after this substitution.
    peer.create(CREATOR, "unrelated")
    queries = verifier["creator_queries"](BINARY, CREATOR)
    response = peer.exchange(next(queries))
    with pytest.raises(ValueError, match="missing expected denoms") as error:
        queries.send(response)
    assert f"factory/{CREATOR}/{missing}" in str(error.value)


@pytest.mark.parametrize(
    "response",
    [
        "",
        "not json",
        "{}",
        "null",
        "[]",
        '{"denoms":null}',
        '{"denoms":"30"}',
        '{"denoms":[null]}',
        '{"denoms":[1]}',
        '{"denoms":[""]}',
        '{"denoms":[],"pagination":[]}',
        '{"denoms":[],"pagination":{"next_key":false}}',
        '{"denoms":[],"pagination":{"next_key":"!invalid!"}}',
        '{"denoms":[],"pagination":{"next_key":"AA=="}}',
        '{"denoms":[],"pagination":{"next_key":"/w=="}}',
    ],
)
def test_tokenfactory_unset_creator_rejects_bad_json_pages(verifier, response):
    queries = verifier["creator_queries"](BINARY, CREATOR)
    next(queries)
    with pytest.raises(ValueError):
        queries.send(response)


@pytest.mark.parametrize("pagination", [None, {}, {"next_key": None}, {"next_key": ""}])
def test_tokenfactory_unset_creator_terminal_proto_forms(verifier, pagination):
    queries = verifier["creator_queries"](BINARY, CREATOR)
    next(queries)
    response = {"denoms": [f"factory/{CREATOR}/{n}" for n in range(1, 31)]}
    if pagination is not None:
        response["pagination"] = pagination
    with pytest.raises(StopIteration) as complete:
        queries.send(json.dumps(response))
    assert complete.value.value == 30


def test_tokenfactory_unset_creator_checks_later_page_errors(verifier):
    queries = verifier["creator_queries"](BINARY, CREATOR)
    peer = CreatorDenomStore(7)
    queries.send(peer.exchange(next(queries)))
    with pytest.raises(ValueError):
        queries.send("broken second page")


def test_tokenfactory_unset_creator_rejects_pagination_cycles(verifier):
    queries = verifier["creator_queries"](BINARY, CREATOR)
    next(queries)
    page = json.dumps(
        {"denoms": [], "pagination": {"next_key": base64.b64encode(b"key").decode()}}
    )
    assert queries.send(page)[-2:] == ["--page-key", "key"]
    with pytest.raises(ValueError, match="repeated or invalid page key"):
        queries.send(page)


def test_tokenfactory_unset_creator_rejects_duplicate_denoms(verifier):
    queries = verifier["creator_queries"](BINARY, CREATOR)
    next(queries)
    with pytest.raises(ValueError, match="repeated a denom"):
        queries.send(json.dumps({"denoms": [f"factory/{CREATOR}/1"] * 30}))


@pytest.mark.parametrize("binary,creator", [("", CREATOR), (BINARY, "")])
def test_tokenfactory_unset_creator_rejects_missing_identity(verifier, binary, creator):
    with pytest.raises(ValueError, match="binary and creator address are required"):
        next(verifier["creator_queries"](binary, creator))


def test_tokenfactory_unset_creator_record_names_proof_and_limits():
    record = RECORD.read_text()
    assert 'finding = "tokenfactory-unset-creator"' in record
    assert "test_tokenfactory_unset_creator_exchange_all_thirty" in record
    assert "test_tokenfactory_unset_creator_guarded_source_contract" in record
    assert "test_tokenfactory_unset_creator_rejects_each_missing_denom" in record
    assert "base_revision" in record and "review_only" in record
    assert "hf-done" in record and "baseline" in record
