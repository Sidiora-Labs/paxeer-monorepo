import shutil
import tempfile
from pathlib import Path
import unittest
from dispatcher.patches import build_patch, stage_patches, PatchError, _reserved

class OwnedLeafArtifactTests(unittest.TestCase):
 def test_exact_owned_leaf_artifacts_patch_and_stage(self):
  for lane,paths in [('sdk/launchpad/packages/sdk/**',['sdk/launchpad/packages/sdk/package.json','sdk/launchpad/packages/sdk/src/types.gen.ts']),('apps/launchpad/mobile/**',['apps/launchpad/mobile/package.json'])]:
   with self.subTest(lane=lane),tempfile.TemporaryDirectory() as tmp:
    root=Path(tmp);base=root/'base';base.mkdir();candidate=root/'candidate';candidate.mkdir()
    for path in paths:
     target=candidate/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((Path(__file__).resolve().parents[5]/path).read_bytes())
    patch=build_patch(base,candidate,[lane],root/'patch')
    self.assertEqual({e['path'] for e in patch['entries']},set(paths))
    result=stage_patches(base,[root/'patch'],root/'stage',verify=lambda p: all((p/x).read_bytes()==(candidate/x).read_bytes() for x in paths),ownership={patch['patch_digest']:[lane]})
    self.assertTrue(result['qualified'])
    # Exception is not an authorization: foreign lane still cannot submit it.
    with self.assertRaises(PatchError):build_patch(base,candidate,['unrelated/**'],root/'foreign')
    with self.assertRaises(PatchError):stage_patches(base,[root/'patch'],root/'foreign-stage',verify=lambda tree: self.fail('foreign ownership reached verifier'),ownership={patch['patch_digest']:['unrelated/**']})
    self.assertFalse((root/'foreign-stage').exists())
 def test_root_lock_schema_generated_and_path_escapes_stay_denied(self):
  paths=['client/packages/sdk/package.json','client/packages/sdk/src/types.gen.ts','client/apps/mobile/package.json','package.json','sdk/launchpad/package.json','client/package.json','apps/launchpad/web/package.json','sdk/launchpad/packages/sdk/nested/package.json','apps/launchpad/mobile/nested/package.json','sdk/launchpad/packages/sdk/pnpm-lock.yaml','apps/launchpad/mobile/package-lock.json','pnpm-workspace.yaml','sdk/launchpad/packages/sdk/src/other.gen.ts','apps/launchpad/mobile/types.gen.ts','sdk/launchpad/packages/sdk/types.gen.ts','sdk/launchpad/packages/sdk/src/schema.gen.go','sdk/launchpad/packages/production-contracts/src/types.gen.ts','launchpad/spec/production-system/integration/contracts/v1/openapi.json','sdk/launchpad/shared/interfaces/schema.json','sdk/launchpad/packages/sdk/migrations/001.sql']
  for path in paths:
   with self.subTest(path=path),tempfile.TemporaryDirectory() as tmp:
    self.assertTrue(_reserved(path))
    root=Path(tmp);base=root/'base';base.mkdir();candidate=root/'candidate';candidate.mkdir();target=candidate/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_text('forbidden')
    with self.assertRaises(PatchError):build_patch(base,candidate,[str(Path(path).parent)+'/**'],root/'patch')
  with tempfile.TemporaryDirectory() as tmp:
   root=Path(tmp);base=root/'base';base.mkdir();candidate=root/'candidate';candidate.mkdir()
   for pattern in ['sdk/launchpad/packages/sdk/../sdk/package.json','/sdk/launchpad/packages/sdk/package.json','sdk/launchpad/packages/sdk\\package.json']:
    with self.subTest(pattern=pattern),self.assertRaises(PatchError):build_patch(base,candidate,[pattern],root/'patch')
