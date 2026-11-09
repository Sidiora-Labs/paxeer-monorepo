"""Source-contract proof for the demo repairs; no Node or live services required."""

import ast
import json
import re
import shlex
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
DEMO = ROOT / "services/wallet/demo"
SDK = ROOT / "sdk/wallet/src"


def read(path):
    return path.read_text(encoding="utf-8")


def manifest():
    return json.loads(read(DEMO / "package.json"))


def test_demo_lint_command_and_dependencies():
    package = manifest()
    repository_wallet = json.loads(read(ROOT / "apps/wallet/package.json"))
    assert package["name"] == "@paxeer/demo"
    assert package["type"] == "module"
    assert shlex.split(package["scripts"]["lint"]) == ["eslint", "."]
    assert package["scripts"]["lint"] == repository_wallet["scripts"]["lint"]
    dependencies = package["devDependencies"]
    assert dependencies["eslint"] == repository_wallet["devDependencies"]["eslint"]
    assert dependencies["eslint"].split(".")[0] == "9"
    assert dependencies["eslint-config-next"] == package["dependencies"]["next"]
    assert dependencies["eslint-config-next"].startswith("^16.")
    assert "typescript" in dependencies


def test_demo_flat_config_covers_source_without_rule_overrides():
    config = read(DEMO / "eslint.config.mjs")
    # Parse the complete supported shape, so a second export, source exclusion,
    # disabled rule or conditional fallback cannot hide behind matching tokens.
    shape = re.fullmatch(
        r"\s*import\s+(\w+)\s+from\s+['\"]"
        r"eslint-config-next/core-web-vitals['\"];\s*"
        r"export\s+default\s*\[\s*\{\s*ignores:\s*(\[[^\]]*\])"
        r"\s*,?\s*\}\s*,\s*\.\.\.(\w+)\s*,?\s*\];\s*",
        config,
    )
    assert shape is not None, "Use the Next flat preset without rule overrides"
    assert shape[1] == shape[3]
    exclusions = ast.literal_eval(shape[2])
    assert set(exclusions) == {
        ".next/**",
        "out/**",
        "dist/**",
        "coverage/**",
        "next-env.d.ts",
    }
    assert len(exclusions) == 5
    # The repository's production wallet uses this same Next flat entry point.
    assert "eslint-config-next/core-web-vitals" in read(
        ROOT / "apps/wallet/eslint.config.mjs"
    )


def test_readme_package_commands_resolve_to_registered_scripts():
    readme = read(DEMO / "README.md")
    commands = []
    for block in re.findall(r"```sh\n(.*?)```", readme, re.S):
        commands.extend(
            shlex.split(line, comments=True)
            for line in block.splitlines()
            if line.strip()
        )
    package = manifest()
    sdk_package = json.loads(read(ROOT / "sdk/wallet/package.json"))
    assert ["pnpm", "install"] in commands
    demo_commands = []
    for command in commands:
        if command == ["pnpm", "install"]:
            continue
        assert len(command) == 4
        assert command[:2] == ["pnpm", "--filter"]
        target = {"@paxeer/demo": package, "@paxeer/wallet": sdk_package}[command[2]]
        assert command[3] in target["scripts"]
        if target is package:
            demo_commands.append(command[3])
    assert set(demo_commands) == {"dev", "build", "start", "typecheck", "lint"}
    assert "`eslint .`" in readme
    assert "`eslint.config.mjs`" in readme
    assert "next lint" not in readme
    assert "not a working lint command" not in readme


def test_readme_provisioning_matches_modal_and_panel():
    readme = read(DEMO / "README.md")
    connect = read(DEMO / "src/components/ConnectButton.tsx")
    modal = read(DEMO / "src/components/WalletModal.tsx")
    assert "<WalletModal" in connect
    assert "import { SupportedWalletPanel } from './FundedAccountPanel'" in modal
    assert "SendTxForm" not in modal
    provision = modal.index("await paxeer.provisionStandardWallet()")
    assert provision < modal.index("await standard.refresh()", provision)
    assert "!standard.data ? <button" in modal
    assert "onClick={provision}" in modal
    assert modal.count("<SupportedWalletPanel") == 2
    assert "injected={injected}" in modal
    for fact in (
        "`ConnectButton` opens `WalletModal`",
        "`SupportedWalletPanel`",
        "`SendTxForm` is not mounted by the modal",
        "provisionStandardWallet()",
        "useStandardAccount()",
        "sign-in alone does not create the wallet",
        "Create embedded threshold-custody wallet",
        "EIP-6963",
    ):
        assert fact in readme


def test_readme_send_matches_wallet_and_provider_path():
    readme = read(DEMO / "README.md")
    panel = read(DEMO / "src/components/FundedAccountPanel.tsx")
    wallet = read(SDK / "wallet.ts")
    provider = read(SDK / "provider.ts")
    assert "if (chain !== 125) throw" in panel
    assert "provider.on('accountsChanged', reset)" in panel
    assert "provider.on('chainChanged', reset)" in panel
    assert "if (!/^0x[0-9a-fA-F]{40}$/.test(to)" in panel
    assert "/^(0|[1-9][0-9]*)$/.test(value)" in panel
    assert "/^0x(?:[0-9a-fA-F]{2})*$/.test(data)" in panel
    assert (
        "return wallet.sendTransaction({ to: to as Hex, value: BigInt(value), "
        "data: data as Hex, chainId: 125 })"
    ) in panel
    assert "method: 'eth_sendTransaction', params: [params]" in wallet
    assert "this.signingGateway<SendTxResponse>(context, '/v1/wallet/send'" in provider
    assert "return response.tx_hash" in provider
    assert "injected?.provider ?? new PaxeerProvider" in panel
    assert "confirm: request => window.confirm" in panel
    for fact in (
        "chain ID `125`",
        "integer amount in **wei**",
        "`0x` for a plain transfer",
        "Review and send transaction",
        "`WalletInterface.sendTransaction()`",
        "`eth_sendTransaction`",
        "`/v1/wallet/send`",
        "`window.confirm`",
        "injected provider handles its own signing and submission",
        "does not wait for a committed receipt",
        "funds for the transaction and gas",
    ):
        assert fact in readme


def test_readme_planning_limit_matches_real_endpoint_contract():
    readme = read(DEMO / "README.md")
    panel = read(DEMO / "src/components/FundedAccountPanel.tsx")
    endpoint = read(SDK / "endpoint.ts")
    human = read(SDK / "human.ts")
    assert panel.index("const availability = await kernel.current()") < panel.index(
        "return human.planIntent("
    )
    assert "if (!availability.available) return availability" in panel
    assert "new EndpointClient({" in panel
    assert "hostedUrl:" not in panel
    assert "EndpointClient: hostedUrl required for hosted px_ reads" in endpoint
    assert "this.rpc.call('px_getNetwork', [])" in endpoint
    assert "this.send('POST', '/v1/intents/plan', request, undefined)" in human
    assert "human.submitPlan" not in panel
    for fact in (
        "**Read availability and plan intent**",
        "separate from sending a transaction",
        "`HumanClient.planIntent()`",
        "`POST /v1/intents/plan`",
        "does not submit or execute a plan",
        "`hostedUrl`",
        "EndpointClient: hostedUrl required for hosted px_ reads",
    ):
        assert fact in readme


def test_readme_source_links_and_configuration_are_current():
    readme = read(DEMO / "README.md")
    links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", readme)
    assert links
    for link in links:
        assert (DEMO / link).resolve().exists(), link
    singleton = read(DEMO / "src/lib/paxeer.ts")
    for name in (
        "NEXT_PUBLIC_PAXEER_WALLET_API",
        "NEXT_PUBLIC_SUPABASE_URL",
        "NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY",
        "NEXT_PUBLIC_AUTH_REDIRECT_URL",
    ):
        assert name in singleton
        assert f"`{name}`" in readme
    assert "`NEXT_PUBLIC_SITE_URL`" in readme
    assert "NEXT_PUBLIC_SITE_URL" in read(DEMO / "src/app/layout.tsx")
    assert "does not register a root JSON-RPC handler" in readme
    assert "not substitutes for `NEXT_PUBLIC_PAXEER_WALLET_API`" in readme
    record = read(ROOT / "tools/qualification/audit_repairs/repair_22_33_154.kvx")
    assert "[finding.demo-readme-flow]" in record
    assert "[finding.demo-lint-command]" in record
    assert "review_only =" in record
    assert "services/wallet/pnpm-lock.yaml" in record
