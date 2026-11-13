import { loadPyodide } from 'pyodide';
import { fileURLToPath } from 'node:url';

let runtime;

export async function runPythonProof(source) {
  if (!runtime) {
    runtime = await loadPyodide({
      indexURL: fileURLToPath(new URL('./', import.meta.resolve('pyodide'))),
    });
    runtime.FS.mkdir('/repo');
    runtime.FS.mount(runtime.FS.filesystems.NODEFS, {
      root: fileURLToPath(new URL('../../../../../', import.meta.url)),
    }, '/repo');
    runtime.runPython(`
import sys
sys.dont_write_bytecode = True
sys.path.insert(0, '/repo/sdk/launchpad/packages/sdk/test')
import pathlib, runpy, tempfile
from authoritative_inputs import load_module
suite = pathlib.Path('/repo/sdk/launchpad/packages/sdk/test')
`);
  }
  return runtime.runPython(source);
}
