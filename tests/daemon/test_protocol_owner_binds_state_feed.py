import re
import subprocess
from pathlib import Path


def test_protocol_owner_binds_the_state_feed():
    root = Path(__file__).resolve().parents[2]
    build = subprocess.run(
        ['make', '-C', '.', 'layerxd'],
        cwd=root,
        capture_output=True,
        text=True,
    )
    assert build.returncode == 0, build.stdout + build.stderr

    protocol = (root / 'cmd/layerxd/lxp_daemon_protocol.c').read_text()
    protocol = re.sub(r'/\*.*?\*/|//[^\n]*', '', protocol, flags=re.DOTALL)
    attach = re.search(
        r'\blxp_result\s+lxp_daemon_protocol_owner_attach\s*\([^)]*\)'
        r'\s*\{(.*?)^\}',
        protocol,
        re.DOTALL | re.MULTILINE,
    )
    assert attach is not None
    body = attach.group(1)
    runtime_binding = re.search(
        r'\bstatus\s*=\s*lxp_kernel_bind_module_runtime\s*\('
        r'\s*kernel\s*,\s*LXP_MODULE_PROGRAMS\s*,\s*programs_runtime\s*\);',
        body,
    )
    assert runtime_binding is not None
    feed_binding = re.search(
        r'\bif\s*\(\s*status\s*==\s*LXP_OK\s*\)\s*'
        r'status\s*=\s*lxp_programs_bind_state_feed\s*\('
        r'\s*kernel\s*,\s*&owner->feed_store\.feed\s*\);',
        body,
    )
    assert feed_binding is not None
    maintenance_binding = re.search(
        r'\blxp_programs_state_feed_store_bind_maintenance\s*\('
        r'\s*&owner->feed_store\s*,\s*kernel\s*\);',
        body,
    )
    assert maintenance_binding is not None
    assert runtime_binding.end() < feed_binding.start()
    assert feed_binding.end() < maintenance_binding.start()

    kernel = (root / 'src/protocol/lxp_kernel.c').read_text()
    kernel = re.sub(r'/\*.*?\*/|//[^\n]*', '', kernel, flags=re.DOTALL)
    assert re.search(r'\blxp_programs_bind_state_feed\s*\(', kernel) is None
