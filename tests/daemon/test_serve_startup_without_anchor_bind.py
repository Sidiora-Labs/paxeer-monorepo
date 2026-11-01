import re
import subprocess
from pathlib import Path


def test_serve_startup_no_longer_binds_the_finality_authority():
    root = Path(__file__).resolve().parents[2]
    build = subprocess.run(
        ['make', '-C', '.', 'layerxd'],
        cwd=root,
        capture_output=True,
        text=True,
    )
    assert build.returncode == 0, build.stdout + build.stderr

    process = (root / 'cmd/layerxd/lxp_daemon_process.c').read_text()
    assert re.search(r'\blxp_finality_authority_bind\s*\(', process) is None
    assert re.search(r'\bstage\s*=\s*"finality authority"', process) is None
    assert re.search(
        r'\bevidence_store\s*\.\s*verify_finality_authority\s*=(?!=)',
        process,
    ) is None

    history_open = re.search(
        r'\blxp_daemon_evidence_open_history\s*\((.*?)\);',
        process,
        re.DOTALL,
    )
    assert history_open is not None
    arguments = [argument.strip() for argument in history_open.group(1).split(',')]
    assert len(arguments) == 10
    assert arguments[6:8] == ['NULL', 'NULL']
    history_stage = re.search(r'\bstage\s*=\s*"history"', process)
    assert history_stage is not None
    assert history_open.end() < history_stage.start()
    assert 'LXP_ERR_MODULE_DISABLED' not in process[
        history_open.end():history_stage.start()
    ]

    authority = (root / 'cmd/layerxd/lxp_daemon_finality_authority.c').read_text()
    assert re.search(
        r'\blxp_result\s+lxp_finality_authority_bind\s*\([^)]*\)\s*\{',
        authority,
    ) is not None
