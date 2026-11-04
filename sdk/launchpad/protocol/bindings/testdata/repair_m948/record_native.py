"""Record the real executor emitter output with the repository's Go toolchain.

The source overlay adds an observation entrypoint, leaving production files
untouched. All serialization and origin/refusal decisions run in events.go.
"""
from pathlib import Path
import json
import subprocess
import tempfile

here = Path(__file__).resolve().parent
root = here.parents[5]
with tempfile.TemporaryDirectory(prefix='repair-m948-native-') as scratch:
    scratch = Path(scratch)
    bridge = scratch / 'bridge.go'
    bridge.write_text((here / 'native_recorder.go.txt').read_text())
    overlay = scratch / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': {str(root / 'precompiles/launchpad/record_m948.go'): str(bridge)}}))
    main = scratch / 'main.go'
    main.write_text('''package main
import (
 "os"
 launchpad "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
)
func main(){if err:=launchpad.RecordM948Native(os.Args[1],os.Args[2],os.Stdout);err!=nil{panic(err)}}
''')
    revision = (here / 'producer-revision.txt').read_text().strip()
    with (here / 'native-execution.json').open('wb') as output:
        subprocess.run(['go', 'run', '-overlay', str(overlay), str(main), str(root), revision], cwd=root, stdout=output, check=True)
