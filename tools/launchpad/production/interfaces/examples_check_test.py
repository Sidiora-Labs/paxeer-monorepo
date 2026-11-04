"""Real JSONSchema instance and example-manifest regression checks."""
import argparse
import contextlib
import copy
import io
import importlib.util
import json
import pathlib
import sys
import tempfile
import tomllib
import unittest
from jsonschema import ValidationError

HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('examples_check',HERE/'examples_check.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)
ROOT=HERE.parents[3]
CONTRACTS=None
SPEC=None
CATALOG=None
MATERIALIZER=HERE/'materialize.py'

def bind_inputs(contracts, spec=None, catalog=None, materializer=MATERIALIZER):
    global CONTRACTS, SPEC, CATALOG, MATERIALIZER
    if (spec is None)==(catalog is None):
        raise ValueError('exactly one authoritative spec or catalog required')
    CONTRACTS=pathlib.Path(contracts).resolve()
    SPEC=pathlib.Path(spec).resolve() if spec is not None else None
    CATALOG=pathlib.Path(catalog).resolve() if catalog is not None else None
    MATERIALIZER=pathlib.Path(materializer).resolve()


class ExamplesTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if CONTRACTS is None or (SPEC is None and CATALOG is None):
            raise ValueError('explicit spec/catalog and contracts required for example suite')
        cls.catalog=json.loads(CATALOG.read_text()) if CATALOG else tomllib.loads(SPEC.read_text())
        if 'sections' in cls.catalog:cls.catalog=cls.catalog['sections']
        cls.generated=e.generate(cls.catalog,CONTRACTS,e.load_materializer(MATERIALIZER))
        cls.samples=e.Samples(json.loads((CONTRACTS/'schema-definitions.json').read_bytes()))
        cls.openapi=json.loads((CONTRACTS/'openapi.json').read_bytes())
    def test_exact_108_coverage_and_proofs(self):
        self.assertEqual(len(self.generated['endpoints']),108)
        self.assertEqual(set(self.generated['endpoints']),set(self.catalog['endpoint']))
        for name in ('IF_LOGIN','IF_WALLET_LINK_VERIFY'):
            variants=self.generated['endpoints'][name]['requestVariants']
            self.assertEqual({v['request']['body']['proof']['kind'] for v in variants},{'eoa_eip191','erc1271'})
    def test_missing_and_unknown_ids_fail(self):
        for unknown in (False,True):
            bad=copy.deepcopy(self.generated)
            if unknown:bad['endpoints']['IF_UNKNOWN']=bad['endpoints']['IF_LOGIN']
            else:del bad['endpoints']['IF_LOGIN']
            with self.assertRaises(e.ExampleError):e.check(bad,self.generated)
    def test_stale_hash_and_missing_instance_fail(self):
        for field in ('inputs','endpoints'):
            bad=copy.deepcopy(self.generated)
            if field=='inputs':bad[field]['semanticCatalogSha256']='0'*64
            else:bad[field]['IF_LOGIN']['requestVariants']=[]
            with self.assertRaises(e.ExampleError):e.check(bad,self.generated)
    def test_missing_required_rejected_by_actual_schema(self):
        value=self.samples.sample({'$ref':'#/$defs/LoginBody'})
        del value['challengeId']
        with self.assertRaises(ValidationError):self.samples.validate({'$ref':'#/$defs/LoginBody'},value)
    def test_amount_number_rejected_by_actual_schema(self):
        value=self.samples.sample({'$ref':'#/$defs/QuoteRequest'})
        value['inputRaw']=9007199254740993
        with self.assertRaises(ValidationError):self.samples.validate({'$ref':'#/$defs/QuoteRequest'},value)
    def test_unknown_mutation_and_nested_proof_fields_rejected(self):
        for nested in (False,True):
            value=self.samples.sample({'$ref':'#/$defs/LoginBody'})
            (value['proof'] if nested else value)['unknownMutationField']=True
            with self.assertRaises(ValidationError):self.samples.validate({'$ref':'#/$defs/LoginBody'},value)
    def test_additive_response_accepted_by_actual_schema(self):
        value=self.samples.sample({'$ref':'#/$defs/PoolListResponse'})
        value['futureEnvelopeField']='compatible'
        value['data']['futureDomainField']='compatible'
        self.samples.validate({'$ref':'#/$defs/PoolListResponse'},value)
        e.reject_pagination_duplicate(value)
    def test_pagination_duplicate_schema_vs_semantic_distinction(self):
        value=self.samples.sample({'$ref':'#/$defs/PoolListResponse'})
        value['data']['nextCursor']='legacy.cursor'
        self.samples.validate({'$ref':'#/$defs/PoolListResponse'},value)
        with self.assertRaises(e.ExampleError):e.reject_pagination_duplicate(value)
    def test_misplaced_request_parameter_fails(self):
        endpoint=self.generated['endpoints']['IF_CANDLES']
        request=copy.deepcopy(endpoint['requestVariants'][0]['request'])
        value=request['path'].pop('poolAddress');request['query']['poolAddress']=value
        operation=self.openapi['paths'][endpoint['path']][endpoint['method'].lower()]
        with self.assertRaises(e.ExampleError):e.validate_placement(self.samples,operation,request)
    def test_sse_frames_and_empty204(self):
        seen_empty=0
        for endpoint in self.generated['endpoints'].values():
            for success in endpoint['successes']:
                if success['status']==204:
                    seen_empty+=1;self.assertIsNone(success['body']);self.assertEqual(success['bodyBytes'],0)
                if success.get('contentType')=='text/event-stream':
                    frame=success['frame'];self.assertTrue(frame.endswith('\n\n'))
                    line=next(line for line in frame.splitlines() if line.startswith('data: '))
                    self.assertEqual(json.loads(line[6:]),success['jsonData'])
        self.assertGreater(seen_empty,0)
        self.assertEqual(len(self.generated['endpoints']['IF_EVENTS_SSE']['successes']),13)
    def test_unknown_schema_or_pattern_has_no_fallback(self):
        for shape in ({},{'type':'object'},{'type':'string','pattern':'^never-generic-fallback$'}):
            with self.assertRaises(e.ExampleError):self.samples.sample(shape)
    def test_task_lifecycle_excluded_normative_change_included(self):
        materializer=e.load_materializer(MATERIALIZER)
        digest=materializer.semantic_digest(self.catalog)
        other=copy.deepcopy(self.catalog);other['task']['lifecycle_test']={'status':'done'}
        self.assertEqual(digest,materializer.semantic_digest(other))
        other['endpoint']['IF_LOGIN']['success_status']=299
        self.assertNotEqual(digest,materializer.semantic_digest(other))
    def test_check_read_only_and_rejects_mutated_bytes(self):
        with tempfile.TemporaryDirectory() as temp:
            path=pathlib.Path(temp)/'examples.json';path.write_bytes(e.encode(self.generated));before=path.read_bytes()
            cmd=['--check','--contracts',str(CONTRACTS),'--materializer',str(MATERIALIZER),'--examples',str(path)]
            cmd+=['--catalog',str(CATALOG)] if CATALOG else ['--spec',str(SPEC)]
            with contextlib.redirect_stdout(io.StringIO()):e.main(cmd)
            self.assertEqual(path.read_bytes(),before)
            path.write_bytes(before.replace(b'"IF_LOGIN":',b'"IF_LOGIN_CHANGED":',1));bad=path.read_bytes()
            with self.assertRaises(e.ExampleError):e.main(cmd)
            self.assertEqual(path.read_bytes(),bad)

def main(argv=None):
    parser=argparse.ArgumentParser()
    source=parser.add_mutually_exclusive_group(required=True)
    source.add_argument('--spec',type=pathlib.Path)
    source.add_argument('--catalog',type=pathlib.Path)
    parser.add_argument('--contracts',type=pathlib.Path,required=True)
    parser.add_argument('--materializer',type=pathlib.Path,default=MATERIALIZER)
    args,remaining=parser.parse_known_args(argv)
    bind_inputs(args.contracts,args.spec,args.catalog,args.materializer)
    unittest.main(argv=[sys.argv[0],*remaining])

if __name__=='__main__':main()
