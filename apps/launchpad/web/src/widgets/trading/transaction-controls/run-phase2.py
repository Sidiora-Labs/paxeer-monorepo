#!/usr/bin/env python3
import argparse, hashlib, json, os, pathlib, subprocess, sys


def prepare_phase2_environment(fixture, fixture_sha256, rpc_url, compiler, compiler_sha256, output):
    fixture, compiler, output = map(pathlib.Path, (fixture, compiler, output))
    output.mkdir(parents=True, exist_ok=False)
    for path, digest in [(fixture, fixture_sha256), (compiler, compiler_sha256)]:
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise RuntimeError('Bound input hash mismatch: ' + str(path))
    source = pathlib.Path(__file__).parent / 'fixtures/ZeroResetToken.sol'
    compiler_input = {'language': 'Solidity', 'sources': {'ZeroResetToken.sol': {'content': source.read_text()}}, 'settings': {'outputSelection': {'*': {'*': ['abi', 'evm.bytecode.object']}}}}
    compiled = subprocess.run([str(compiler), '--standard-json'], input=json.dumps(compiler_input), text=True, capture_output=True, timeout=60)
    (output / 'compiler.log').write_text(compiled.stdout + '\n' + compiled.stderr)
    if compiled.returncode:
        raise RuntimeError('Actual fixture compiler failed: ' + str(output / 'compiler.log'))
    result = json.loads(compiled.stdout)
    if any(error.get('severity') == 'error' for error in result.get('errors', [])):
        raise RuntimeError('Actual fixture compilation failed')
    token = result['contracts']['ZeroResetToken.sol']['ZeroResetToken']
    artifact = output / 'ZeroResetToken.json'
    artifact.write_text(json.dumps({'abi': token['abi'], 'bytecode': '0x' + token['evm']['bytecode']['object'], 'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest()}))
    return {'L26_FIXTURE_JSON': str(fixture), 'L26_FIXTURE_SHA256': fixture_sha256, 'L26_RPC_URL': rpc_url, 'L26_ZERO_RESET_ARTIFACT': str(artifact), 'L26_CACHE_DIR': str(output / 'vite-cache')}

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--fixture',required=True)
    parser.add_argument('--fixture-sha256',required=True)
    parser.add_argument('--rpc-url',required=True)
    parser.add_argument('--compiler',required=True)
    parser.add_argument('--compiler-sha256',required=True)
    parser.add_argument('--output-dir',required=True)
    args=parser.parse_args()
    root=pathlib.Path(__file__).resolve().parents[7]
    output=pathlib.Path(args.output_dir).resolve()
    fixture=pathlib.Path(args.fixture).resolve();compiler=pathlib.Path(args.compiler).resolve()
    for path,digest in [(fixture,args.fixture_sha256),(compiler,args.compiler_sha256)]:
        if hashlib.sha256(path.read_bytes()).hexdigest()!=digest:raise RuntimeError('Bound input hash mismatch: '+str(path))
    prepared = prepare_phase2_environment(fixture, args.fixture_sha256, args.rpc_url, compiler, args.compiler_sha256, output)
    artifact = pathlib.Path(prepared['L26_ZERO_RESET_ARTIFACT'])
    env={**os.environ,'L26_FIXTURE_JSON':str(fixture),'L26_FIXTURE_SHA256':args.fixture_sha256,'L26_RPC_URL':args.rpc_url,'L26_ZERO_RESET_ARTIFACT':str(artifact),'L26_CACHE_DIR':str(output/'vite-cache'),'WALLET_TEST_RPC_URL':args.rpc_url,'WALLET_TEST_ALLOW_CHAIN_MUTATION':'1'}
    native=json.loads((pathlib.Path(__file__).parent/'native-runner.json').read_text())
    web=root/native['web']
    command=['node',str(web/native['entry']),*native['arguments'],'--outputFile='+str(output/'vitest.json')]
    completed=subprocess.run(command,cwd=web,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=240)
    (output/'native.log').write_text(completed.stdout)
    sys.stdout.write(completed.stdout);sys.stdout.flush()
    code=completed.returncode
    (output/'result.json').write_text(json.dumps({'command':command,'exit_code':code,'fixture_sha256':args.fixture_sha256,'compiler_sha256':args.compiler_sha256,'native_log':str(output/'native.log')}))
    return code
if __name__=='__main__':sys.exit(main())
