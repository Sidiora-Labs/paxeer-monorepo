import json
from pathlib import Path
import subprocess
import hashlib
import sys


def generate_account_fixtures():
    root = Path(__file__).resolve().parents[3]
    raw = subprocess.check_output([root / 'build/tests/programs_call_activity', '--dump-asset-account-v4'], cwd=root)
    vector = json.loads(raw)
    out = root / 'programs/fixtures/pay5'
    (out / 'receipt-account-bound-v4.json').write_bytes(raw)
    terminal = bytes.fromhex(vector['terminal_payload_hex'])
    domain = b'LayerX/programs/402LXP/account-bound-set/v1\0'
    offset = terminal.index(domain)
    length = int.from_bytes(terminal[offset-4:offset], 'big')
    auth = terminal[offset:offset+length]
    original_length = int.from_bytes(auth[len(domain):len(domain)+4], 'big')
    name_offset = len(domain)+4+original_length
    name_length = int.from_bytes(auth[name_offset:name_offset+2], 'big')
    assert name_offset + 2 + name_length == len(auth)
    # The native terminal's applied leg attachment supplies the signed kernel root.
    legs_length = int.from_bytes(terminal[-119:-115], 'big')
    assert legs_length == 115
    leg = terminal[-115:]
    root_hash = hashlib.sha256(b'LXP/v1/merkle-leaf\0' + leg).hexdigest()
    cases = [{'name': 'native-per-asset', 'encoded': auth.hex(), 'root': root_hash, 'accept': True}]

    def reject(name, data):
        cases.append({'name': name, 'encoded': data.hex(), 'root': root_hash, 'accept': False})

    reject('trailing', auth+b'\0')
    reject('nested', domain+len(auth).to_bytes(4,'big')+auth)
    for index in range(len(auth)):
        reject(f'truncated-{index}', auth[:index])
    for label, replacement in [('owner', b'agent:did:lxp:other:main'), ('empty', b''), ('uppercase', b'agent:DID:lxp:artifact:main'), ('double-colon', b'agent:did::artifact:main'), ('leading-colon', b'agent::artifact:main'), ('oversized', b'agent:'+b'a'*512+b':main')]:
        reject(label, auth[:name_offset]+len(replacement).to_bytes(2,'big')+replacement)
    changed = bytearray(auth); changed[-1] = ord('0') if changed[-1] != ord('0') else ord('1')
    reject('wrong-asset', bytes(changed))
    (out / 'account-authorization-vectors.json').write_text(json.dumps(cases, indent=2)+'\n')
    for path in sorted(out.glob('native-*/*.terminal')):
        terminal = path.read_bytes()
        if domain not in terminal:
            continue
        offset = terminal.index(domain)
        length = int.from_bytes(terminal[offset-4:offset], 'big')
        auth = terminal[offset:offset+length]
        prefix = b'LXP/programs/terminal-applied-legs/v1\0'
        assert terminal.startswith(prefix)
        detail_length = int.from_bytes(terminal[len(prefix):len(prefix)+4], 'big')
        leg_offset = len(prefix)+4+detail_length
        legs_length = int.from_bytes(terminal[leg_offset:leg_offset+4], 'big')
        legs = terminal[leg_offset+4:]
        assert len(legs) == legs_length and legs_length % 115 == 0 and legs_length > 0
        leaves = [hashlib.sha256(b'LXP/v1/merkle-leaf\0'+legs[i:i+115]).digest() for i in range(0,len(legs),115)]
        while len(leaves)>1:
            leaves = [hashlib.sha256(b'LXP/v1/merkle-internal\0'+leaves[i]+leaves[min(i+1,len(leaves)-1)]).digest() for i in range(0,len(leaves),2)]
        root_hash = leaves[0].hex()
        label = str(path.relative_to(out))
        cases.append({'name': label, 'encoded': auth.hex(), 'root': root_hash, 'accept': True})
        original_length = int.from_bytes(auth[len(domain):len(domain)+4], 'big')
        cursor = len(domain)+4+original_length
        while cursor < len(auth):
            length = int.from_bytes(auth[cursor:cursor+2], 'big')
            reject(label+f'-forged-name-{cursor}', auth[:cursor]+b'\0\1x'+auth[cursor+2+length:])
            cursor += 2+length
    (out / 'account-authorization-vectors.json').write_text(json.dumps(cases, indent=2)+'\n')


def destination_vectors(accepted):
    domain = b'LayerX/programs/402LXP/account-bound-set/v2\0'
    v1_domain = b'LayerX/programs/402LXP/account-bound-set/v1\0'
    cases = list(accepted)
    for vector in accepted:
        auth = bytes.fromhex(vector['encoded'])
        assert vector['accept'] and auth.startswith(domain)
        original_length = int.from_bytes(auth[len(domain):len(domain)+4], 'big')
        names_offset = len(domain)+4+original_length
        cursor = names_offset
        names = []
        while cursor < len(auth):
            pair = []
            for _ in range(2):
                length = int.from_bytes(auth[cursor:cursor+2], 'big')
                cursor += 2
                pair.append(auth[cursor:cursor+length])
                cursor += length
            names.append(pair)
        assert cursor == len(auth) and names[0][1]

        def reject(label, data):
            cases.append(dict(name=vector['name']+'-'+label,
                              encoded=data.hex(), root=vector['root'], accept=False))

        def wrapped(pairs):
            return auth[:names_offset]+b''.join(
                len(name).to_bytes(2, 'big')+name for pair in pairs for name in pair)

        reject('trailing', auth+b'\0')
        reject('nested-v2', domain+len(auth).to_bytes(4, 'big')+auth)
        v1 = v1_domain+auth[len(domain):names_offset]+b''.join(
            len(pair[0]).to_bytes(2, 'big')+pair[0] for pair in names)
        reject('nested-v1', domain+len(v1).to_bytes(4, 'big')+v1)
        reject('v1-nested-v2', v1_domain+len(auth).to_bytes(4, 'big')+auth)
        for index in range(len(auth)):
            reject(f'truncated-{index}', auth[:index])
        for label, replacement in [
            ('destination-other-identity', b'agent:did:lxp:other:main'),
            ('destination-wrong-asset', b'agent:did:lxp:counter-recipient:asset:'+b'08'*32),
        ]:
            changed = [pair.copy() for pair in names]
            changed[0][1] = replacement
            reject(label, wrapped(changed))
        reject('empty-destinations', wrapped([[pair[0], b''] for pair in names]))
    return cases


def generate_destination_fixtures():
    root = Path(__file__).resolve().parents[3]
    subprocess.run(['make', '-C', str(root), 'build/tests/programs_call_activity'],
                   cwd=root, stdout=sys.stderr, check=True)
    accepted = [json.loads(subprocess.check_output(
        [root / 'build/tests/programs_call_activity', option], cwd=root))
        for option in ['--dump-identity-destination', '--dump-mixed-destinations']]
    target = root / 'programs/fixtures/pay5/destination-authorization-vectors.json'
    target.write_text(json.dumps(destination_vectors(accepted), indent=2)+'\n')


if __name__ == '__main__':
    if sys.argv[1:] != ['--destinations-only']:
        generate_account_fixtures()
    generate_destination_fixtures()
