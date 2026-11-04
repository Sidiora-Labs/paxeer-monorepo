"""Bind the complete timelock ABI to fresh, successful Foundry state tests."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys


CONTRACT = 'contracts/governance/SidioraProxyTimelock.sol:SidioraProxyTimelock'
TEST_PATH = 'tests/solidity/governance/SidioraProxyTimelock.t.sol'
TEST_CONTRACT = 'SidioraProxyTimelockTest'
TEST_KEY = TEST_PATH + ':' + TEST_CONTRACT
BINDINGS = {
    'schedule(address,uint256,bytes,bytes32,uint64)': (
        'test_schedule_recordsOperationIdAndReadyTime',
        ('timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY)',
         'require(timelock.readyAt(id) == scheduledAt + DELAY,',
         'require(timelock.operationNonce() == nonce + 1,'),
    ),
    'execute(address,uint256,bytes,bytes32,uint256)': (
        'test_execute_rejectsBeforeDelayAppliesAtReadyTimeAndRejectsReplay',
        ('timelock.execute(address(treasury), 0, upgradeData, SALT, 0)',
         'vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector)',
         'require(!timelock.completed(id),', 'require(timelock.completed(id),',
         '_assertNotApplied()', '_assertApplied()'),
    ),
    'cancel(bytes32)': (
        'test_cancel_proposerWithGuardianRoleOnlyAndPreventsExecution',
        ('timelock.cancel(id)', 'require(timelock.readyAt(id) == ready,',
         'require(timelock.readyAt(id) == 0,',
         'vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector)',
         'timelock.execute(address(treasury), 0, upgradeData, SALT, 0)',
         'require(!timelock.completed(id),'),
    ),
}


class SurfaceError(ValueError):
    pass


def abi_type(item):
    kind = item['type']
    if kind.startswith('tuple'):
        return '(' + ','.join(abi_type(part) for part in item['components']) + ')' + kind[5:]
    return kind


def state_selectors(abi):
    return {
        item['name'] + '(' + ','.join(abi_type(part) for part in item['inputs']) + ')'
        for item in abi if item['type'] == 'function'
        and item['stateMutability'] not in ('view', 'pure')
    }


def function_body(source, name):
    # Strip comments while retaining strings so a comment cannot supply a call.
    source = re.sub(r'//[^\n]*|/\*[\s\S]*?\*/|"(?:\\.|[^"\\])*"',
                    lambda match: match[0] if match[0].startswith('"') else ' ', source)
    match = re.search(r'\bfunction\s+' + re.escape(name) + r'\s*\([^)]*\)[^{;]*\{', source)
    if not match:
        raise SurfaceError('bound test function is absent')
    depth = 1
    start = index = match.end()
    # Braces inside Solidity strings are irrelevant to the function boundary.
    tokens = re.finditer(r'"(?:\\.|[^"\\])*"|[{}]', source[index:])
    for token in tokens:
        if token[0] == '{':
            depth += 1
        elif token[0] == '}':
            depth -= 1
            if depth == 0:
                return source[start:index + token.start()]
    raise SurfaceError('unterminated test function')


def code_tokens(source):
    return re.findall(r'[A-Za-z_][A-Za-z_0-9]*|[0-9]+|[^\s]', source)


def has_code(body, fragment):
    # String literals cannot masquerade as executed assertions or calls.
    body = re.sub(r'"(?:\\.|[^"\\])*"', '""', body)
    tokens, wanted = code_tokens(body), code_tokens(fragment)
    return any(tokens[index:index + len(wanted)] == wanted
               for index in range(len(tokens) - len(wanted) + 1))


def verify_source_bindings(abi, source):
    selectors = state_selectors(abi)
    if not selectors or selectors != set(BINDINGS):
        raise SurfaceError('state-changing ABI has an unbound full signature or missing selector')
    for selector, (name, fragments) in BINDINGS.items():
        body = function_body(source, name)
        if not all(has_code(body, fragment) for fragment in fragments):
            raise SurfaceError('test lacks the bound operation and state assertion: ' + selector)
    # Execution proves the proxy implementation and migration state, too.
    for name, fragments in {
        '_assertApplied': (
            'vm.load(address(treasury), IMPLEMENTATION_SLOT) == bytes32(uint256(uint160(address(nextImplementation))))',
            'require(treasury.hasRole(treasury.DEPOSITOR_ROLE(), OUTSIDER),',
        ),
        '_assertNotApplied': (
            'vm.load(address(treasury), IMPLEMENTATION_SLOT) == bytes32(uint256(uint160(address(originalImplementation))))',
            'require(!treasury.hasRole(treasury.DEPOSITOR_ROLE(), OUTSIDER),',
        ),
    }.items():
        body = function_body(source, name)
        if not all(has_code(body, fragment) for fragment in fragments):
            raise SurfaceError('proxy state assertion is absent')
    return selectors


def verify_evidence(abi, results, source):
    selectors = verify_source_bindings(abi, source)
    suite = results.get(TEST_KEY, {}).get('test_results', {})
    if not suite or any(test.get('status') != 'Success' for test in suite.values()):
        raise SurfaceError('Foundry suite is empty or contains a failed/skipped test')
    for selector, (name, _) in BINDINGS.items():
        if suite.get(name + '()', {}).get('status') != 'Success':
            raise SurfaceError('full selector has no successful executed Foundry test: ' + selector)
    return selectors


def forge_json(arguments, environment):
    result = subprocess.run(['forge', *arguments], env=environment, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    if result.returncode != 0:
        raise SurfaceError('Foundry command failed')
    return json.loads(result.stdout)


def main():
    environment = os.environ.copy()
    environment['FOUNDRY_PROFILE'] = 'governance'
    environment['FOUNDRY_CONFIG'] = 'foundry.toml'
    try:
        abi = forge_json(['inspect', CONTRACT, 'abi', '--json'], environment)
        results = forge_json(['test', '--json', '--match-path', TEST_PATH,
                              '--match-contract', '^' + TEST_CONTRACT + '$'], environment)
        selectors = verify_evidence(abi, results, Path(TEST_PATH).read_text())
    except (OSError, ValueError, KeyError, TypeError) as error:
        print('solidity state surface refused: ' + str(error), file=sys.stderr)
        return 1
    print(f'solidity state surface: {len(selectors)} full selectors have passing state evidence')
    return 0


if __name__ == '__main__':
    sys.exit(main())
