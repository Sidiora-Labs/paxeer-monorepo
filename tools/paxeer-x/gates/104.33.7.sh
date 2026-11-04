#!/usr/bin/env bash
# paxeer-x-services: platform-gateway human-explorer-index platform-program-registry sequencer platform-receipt-authority
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
exec python3 - <<'GATE'
import os,pathlib,stat,subprocess,sys
root=pathlib.Path.cwd()
sys.dont_write_bytecode = True
sys.path.insert(0, str(root / "tools/paxeer-x/gates"))
from hosted_profile import load_native as load_profile
import program_operations as owner

def require(value,reason):
    if not value:raise RuntimeError(reason)
def private(path):
    p=pathlib.Path(path);info=p.lstat()
    require(stat.S_ISREG(info.st_mode) and not p.is_symlink() and info.st_mode&0o077==0,'private regular artifact required')
    require(not any(part=='.env' or part.startswith('.env.') for part in p.parts),'environment files forbidden')
    return p
try:
    raw=os.environ.get('PAXEER_X_HOSTED_PROGRAM_ARTIFACTS')
    require(raw,'actual current prebuilt gateway/explorer artifact manifest required')
    private(raw)
    revision=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
    profile_path=os.environ.get('PAXEER_X_HOSTED_PROGRAM_PROFILE')
    require(profile_path,'protected connected native Programs profile required')
    inputs=load_profile(profile_path,revision,root)
    owner.qualify(root,inputs)
except (OSError,ValueError,KeyError,RuntimeError,subprocess.SubprocessError) as error:
    print('hosted Programs qualification refused: '+str(error),file=sys.stderr);sys.exit(78)
GATE
