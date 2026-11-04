import argparse,copy,hashlib,importlib.util,json,pathlib,re,tempfile,unittest
from jsonschema import Draft202012Validator,FormatChecker
HERE=pathlib.Path(__file__).resolve().parent
loader=importlib.util.spec_from_file_location('materialize',HERE/'materialize.py');m=importlib.util.module_from_spec(loader);loader.loader.exec_module(m)
ROOT=HERE.parents[3]
SPEC=None
OUT=ROOT/'sdk/launchpad/protocol/contracts/v1'
CATALOG=None
RECORD=HERE/'testdata/repair_22_33_963/sdk-placement.json'
SUPPLEMENTARY=None
def bind_inputs(supplementary, spec=None, catalog=None):
 global SPEC,CATALOG,SUPPLEMENTARY
 if (spec is None)==(catalog is None):raise ValueError('exactly one authoritative spec or catalog required')
 SPEC=pathlib.Path(spec).resolve() if spec is not None else None
 CATALOG=pathlib.Path(catalog).resolve() if catalog is not None else None
 SUPPLEMENTARY=pathlib.Path(supplementary).resolve()
def derive_authority(directory, record_path=RECORD):
 """Project the admitted SDK operation record into the materializer's input DSL."""
 record=json.loads(record_path.read_bytes())
 for relative,digest in record['sources'].items():
  if hashlib.sha256((ROOT/relative).read_bytes()).hexdigest()!=digest:
   raise m.ContractError('Recorded SDK source digest mismatch: '+relative)
 definitions=json.loads((ROOT/record['schemaSource']).read_bytes())['$defs']
 operations=json.loads((ROOT/'sdk/launchpad/packages/sdk/src/operations.gen.ts').read_text().split('export const operationDefinitions = ',1)[1].rsplit(' as const;',1)[0])
 catalog={key:{} for key in m.SEMANTIC_SECTIONS}
 supplementary={'definitions':{},'fieldOverrides':{},'endpointOverrides':{}}
 compiler=m.Compiler(catalog,supplementary)
 candidates=['UTC','UUID','UUIDv7','hex20','hex32','hex65','hex64charsWithoutPrefix',
             'lowercaseHex64charsWithoutPrefix','finiteDecimalString','allowedHttpsOrigin',
             'httpsURL','hostname','BCP47','IANA','relativeAllowlistedPath','uppercaseSnake<=64',
             'boolean','null','TopicSequenceVector<=20']
 scalar_shapes={json.dumps(compiler.parse_type(token),sort_keys=True):token for token in candidates}
 def token(shape):
  shape=copy.deepcopy(shape)
  default=shape.pop('default',None)
  if default is not None:return token(shape)+' default'+str(default)
  if '$ref' in shape:
   name=shape['$ref'].removeprefix('#/$defs/')
   return name.removeprefix('Input') if name.startswith('Input') else name
  key=json.dumps(shape,sort_keys=True)
  if key in scalar_shapes:return scalar_shapes[key]
  if 'anyOf' in shape:
   variants=shape['anyOf']
   if len(variants)==2 and variants[-1]=={'type':'null'}:return token(variants[0])+'|null'
   if variants==definitions['TopicSnapshotEntry']['properties']['payload']['anyOf']:
    return 'declaredTopicPayload'
   raise m.ContractError('Unrepresentable recorded union: '+str(shape))
  if shape.get('type')==['string','null']:
   shape['type']='string';return token(shape)+'|null'
  if 'const' in shape:
   value=shape['const']
   if type(value) is bool:return 'boolean='+str(value).lower()
   return ('integer=' if type(value) is int else 'constant=')+str(value)
  if 'enum' in shape:return '|'.join(shape['enum'])
  if shape.get('type')=='integer':return str(shape['minimum'])+'..'+str(shape['maximum'])
  if shape.get('type')=='array':
   item=token(shape['items'])
   if not re.fullmatch('[A-Za-z0-9]+',item):
    name='RecordedItem'+hashlib.sha256(json.dumps(shape['items'],sort_keys=True).encode()).hexdigest()[:12]
    supplementary['definitions'][name]=item;item=name
   if 'minItems' not in shape and shape['maxItems']==100 and not shape.get('uniqueItems'):
    return item+'[]'
   return ('distinct' if shape.get('uniqueItems') else '')+item+'Array'+str(shape.get('minItems',0))+'..'+str(shape['maxItems'])
  if shape.get('type')=='string' and set(shape)<= {'type','minLength','maxLength'}:
   return 'string'+str(shape.get('minLength',0))+'..'+str(shape['maxLength'])
  if shape.get('pattern')=='^0x(?:[0-9a-fA-F]{2})*$':return 'hexBytes<='+str((shape['maxLength']-2)//2)
  raise m.ContractError('Unrepresentable recorded field: '+str(shape))
 def fields(shape):
  return '; '.join(name+':'+token(field)+('' if name in shape['required'] else '?')
                   for name,field in shape['properties'].items()) or 'none'
 for identifier in record['catalog']['schema']:
  names=[name for name in definitions if name.lower()==identifier.removeprefix('IF_').replace('_','').lower()]
  if len(names)!=1:raise m.ContractError('Ambiguous SDK schema: '+identifier)
  name=names[0]
  if name in ('Conventions','ResponseEnvelope'):
   shape={'properties':{},'required':[]}
  elif name=='StreamEvent':
   shape=copy.deepcopy(definitions['MarketTradesEvent'])
   shape['properties']['payload']={'anyOf':definitions['TopicSnapshotEntry']['properties']['payload']['anyOf']}
   shape['properties']['topic']={'type':'string','maxLength':200}
  else:shape=definitions[name]
  catalog['schema'][identifier]={'wire_name':name,'fields':fields(shape) if 'properties' in shape else 'none',
                                'semantics':shape.get('description','Proposed '+name)}
 catalog['schema']['IF_CONVENTIONS']['time']='UTC millisecond calendar timestamp'
 domain_names={entry['wire_name'] for entry in catalog['schema'].values()}
 generated_names={name for entry in record['mapping']['endpoints'].values() for name in (entry['request'],entry['data'],entry['response'],entry['request'].removesuffix('Request')+'Body')}
 builtins=set(compiler.defs)|{'OpaqueResumeCursor','WalletProof','EoaWalletProof','Erc1271WalletProof'}
 # The compiler owns scalar and wallet unions; supplementary named objects come
 # directly from the frozen producer and never supply an unconstrained shape.
 for name,shape in definitions.items():
  if name in generated_names or name in domain_names or name in builtins or name in record['mapping']['streams'] or name.startswith('Input'):continue
  if 'properties' in shape:supplementary['definitions'][name]=fields(shape)
  elif name=='ResponseEnvelopeData':supplementary['definitions'][name]='declaredDomainPayload'
  else:supplementary['definitions'][name]=token(shape)
 for identifier,entry in record['mapping']['endpoints'].items():
  catalog['endpoint'][identifier]={
   'path':re.sub(r'^/v2/','/v1/',entry['path']), 'method':entry['method'],
   'request_fields':fields(definitions[entry['request']]),
   'response_fields':'empty204' if entry['wireMode']=='empty' else (
       'none' if entry['wireMode']=='sse' else fields(definitions[entry['data']])),
   'success_status':record['catalog']['endpoint'][identifier]['success_status'],
   'auth':'session' if operations[identifier]['authenticated'] else 'public',
  }
 for envelope in record['mapping']['streams']:
  shape=definitions[envelope]
  pattern=shape['properties']['topic']['pattern'][1:-1]
  pattern=pattern.replace('0x[0-9a-fA-F]{40}','{poolAddress}' if envelope.startswith(('Market','Candle','Stats')) else '{wallet}')
  pattern=pattern.replace('(?:1m|5m|15m|1h|4h|1d|1w)','{interval}')
  pattern=pattern.replace('(?:trending|breakout|volume|gainers|losers|movers|unusual|new)','{category}')
  pattern=pattern.replace('[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}','{id}')
  catalog['stream']['IF_'+re.sub(r'(?<!^)(?=[A-Z])','_',envelope.removesuffix('Event')).upper()]={
   'topic':pattern.replace(r'\.','.'),'payload_schema':shape['properties']['payload']['$ref'].split('/')[-1],
   'semantics':shape['description'],
  }
 error_source=ROOT/'services/launchpad/api/internal/rest/primitives.go'
 if hashlib.sha256(error_source.read_bytes()).hexdigest()!='87bb6e3de16019446a2a527dd98bb38bef9d977b5c73aebaa4782e5e91633bf1':
  raise m.ContractError('Recorded error source digest mismatch')
 rules=error_source.read_text().split('var v2ErrorRules = map[string]v2ErrorRule{',1)[1].split('\n}',1)[0]
 # Select the sixteen contract errors, excluding the parser/internal wrapper rows.
 for code,status in re.findall(r'"([A-Z_]+)":\s*\{(\d+),',rules):
  if code not in ('QUERY_INVALID','INTERNAL_ERROR'):
   catalog['error']['IF_'+code]={'code':code,'http_status':int(status)}
 catalog_path=directory/'admitted-catalog.json';supplementary_path=directory/'supplementary.json'
 catalog_path.write_text(json.dumps(m.semantic_input(catalog),indent=2,sort_keys=True)+'\n')
 supplementary_path.write_text(json.dumps(supplementary,indent=2,sort_keys=True)+'\n')
 return catalog_path,supplementary_path
class MaterializeTests(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  output=tempfile.TemporaryDirectory(prefix='materializer-suite-');cls.addClassCleanup(output.cleanup)
  authority=pathlib.Path(output.name)
  if SPEC is None and CATALOG is None:
   catalog,supplementary=derive_authority(authority)
   args=['--catalog',str(catalog)]
  else:
   supplementary=SUPPLEMENTARY
   args=['--catalog',str(CATALOG)] if CATALOG is not None else ['--spec',str(SPEC)]
  out=authority/'v1'
  m.main(args+['--supplementary',str(supplementary),'--out',str(out),'--frozen-v1-out',str(out)])
  cls.files={str(p.relative_to(out)):p.read_bytes() for p in out.rglob('*') if p.is_file()}
  # The catalog is produced from the selected authority, never assumed to be
  # committed alongside the reviewed supplementary input.
  cls.spec=json.loads(cls.files['inputs/catalog.json'])['sections']
  cls.supp=json.loads(supplementary.read_bytes())
  cls.doc=json.loads(cls.files['schema-definitions.json']);cls.api=json.loads(cls.files['openapi.json']);cls.asyncapi=json.loads(cls.files['asyncapi.json']);cls.map=json.loads(cls.files['catalog-map.json'])
 def valid(self,name,value):
  doc={'$schema':m.DIALECT,'$ref':'#/$defs/'+name,'$defs':self.doc['$defs']};return Draft202012Validator(doc,format_checker=FormatChecker()).is_valid(value)
 def test_all_catalog_entries_mapped(self):
  self.assertEqual((len(self.map['schemas']),len(self.map['endpoints']),len(self.map['streams']),len(self.map['errors'])),(59,108,12,16))
  for id,e in self.map['endpoints'].items():
   for field in ['request','data','response']:self.assertIn(e[field],self.doc['$defs'],id)
 def test_regeneration_deterministic(self):self.assertEqual(self.files,m.generate(self.spec,self.supp))
 def test_all_local_refs_resolve(self):
  for name in ['schema-definitions.json','openapi.json','asyncapi.json']:m.validate_refs(json.loads(self.files[name]))
 def test_uint256_exact_boundary(self):
  for v in ['0',str(2**256-1)]:self.assertTrue(self.valid('UInt256',v))
  for v in [str(2**256),'01','-0','+1','1e18',' 1','1.0',1]:self.assertFalse(self.valid('UInt256',v),str(v))
 def test_aggregate_and_price_have_distinct_caps(self):
  self.assertTrue(self.valid('AggregateUInt','9'*156));self.assertFalse(self.valid('AggregateUInt','9'*157));self.assertTrue(self.valid('PriceComponent','9'*512));self.assertFalse(self.valid('PriceComponent','9'*513));self.assertFalse(self.valid('PositivePriceComponent','0'))
 def test_unknown_type_fails(self):
  spec=copy.deepcopy(self.spec);spec['schema']['IF_POOL']['fields']+='; invented:InventedAnyThing'
  with self.assertRaisesRegex(m.ContractError,'Unknown field type'):m.generate(spec,self.supp)
 def test_duplicate_fields_fail(self):
  spec=copy.deepcopy(self.spec);spec['schema']['IF_POOL']['fields']+='; poolAddress:hex20'
  with self.assertRaisesRegex(m.ContractError,'Duplicate field'):m.generate(spec,self.supp)
 def test_generic_type_fails(self):
  spec=copy.deepcopy(self.spec);spec['schema']['IF_POOL']['fields']+='; invented:object'
  with self.assertRaisesRegex(m.ContractError,'Unresolved generic'):m.generate(spec,self.supp)
 def test_recursive_strict_wallet_proof_and_real_union(self):
  a='0x'+'11'*20
  eoa={'kind':'eoa_eip191','wallet':a,'signature':'0x'+'22'*65};erc={'kind':'erc1271','wallet':a,'signature':'0x1234'}
  self.assertTrue(self.valid('WalletProof',eoa));self.assertTrue(self.valid('WalletProof',erc))
  eoa['signature']='0x12';self.assertFalse(self.valid('WalletProof',eoa))
  request={'challengeId':'00000000-0000-4000-8000-000000000001','proof':erc,'deviceLabel':'test'}
  self.assertTrue(self.valid('LoginBody',request));request['proof']['unexpected']='bad';self.assertFalse(self.valid('LoginBody',request))
 def test_required_nullable_envelope_and_additive_output(self):
  env={'requestId':'00000000-0000-4000-8000-000000000001','data':{'items':[]},'watermark':None,'revision':None,'quality':'complete','nextCursor':None,'generatedAt':'2026-10-09T00:00:00.000Z'}
  self.assertTrue(self.valid('PoolListResponse',env));env['newOptional']='future';self.assertTrue(self.valid('PoolListResponse',env))
  del env['watermark'];self.assertFalse(self.valid('PoolListResponse',env))
 def test_cookie_header_and_sse_contracts(self):
  refresh=self.api['paths']['/v1/auth/sessions:refresh']['post'];self.assertNotIn('requestBody',refresh);self.assertIn(('sidiora_refresh','cookie'),[(x['name'],x['in']) for x in refresh['parameters']])
  deletion=self.api['paths']['/v1/me/deletion-requests/{jobId}']['get'];self.assertIn(('X-Recovery-Receipt','header'),[(x['name'],x['in']) for x in deletion['parameters']]);self.assertNotIn('requestBody',deletion)
  sse=next(v for k,v in self.map['endpoints'].items() if k=='IF_EVENTS_SSE');op=self.api['paths'][sse['path']]['get'];self.assertIn('text/event-stream',op['responses']['200']['content']);self.assertEqual(sse['wireMode'],'sse')
 def test_subscribe_rejects_unknown_and_openapi_parameters_match_strict_request(self):
  frame={'op':'subscribe','subscriptionId':'s1','topics':['market.price.0x'+'11'*20],'afterCursor':None}
  self.assertTrue(self.valid('Subscribe',frame));frame['unknown']=True;self.assertFalse(self.valid('Subscribe',frame))
  for id,entry in self.map['endpoints'].items():
   operation=self.api['paths'][entry['path']][entry['method'].lower()]
   request=self.doc['$defs'][entry['request']]
   for parameter in operation['parameters']:
    if parameter['name'] in request.get('properties',{}):
     self.assertEqual(parameter['schema'],m.replace_refs(request['properties'][parameter['name']],'#/components/schemas/'),id)
 def test_semantic_identity_ignores_task_lifecycle_but_detects_norms(self):
  changed=copy.deepcopy(self.spec);changed.setdefault('task',{}).setdefault('0.2',{})['status']='done';changed.setdefault('meta',{})['status']='different'
  self.assertEqual(m.semantic_digest(self.spec),m.semantic_digest(changed));self.assertEqual(self.files,m.generate(changed,self.supp))
  changed['schema']['IF_POOL']['status']='implemented';self.assertEqual(m.semantic_digest(self.spec),m.semantic_digest(changed));self.assertEqual(self.files,m.generate(changed,self.supp))
  changed['schema']['IF_CONVENTIONS']['time']+=' changed normative';self.assertNotEqual(m.semantic_digest(self.spec),m.semantic_digest(changed))
 def test_utc_clock_range_and_calendar(self):
  field=self.doc['$defs']['Session']['properties']['accessExpiresAt']
  checker=Draft202012Validator(field,format_checker=FormatChecker())
  for value in ['2026-10-09T23:59:59.000Z','0001-01-01T00:00:00.000Z','9999-12-31T23:59:59.999Z','2000-02-29T00:00:00.000Z','2024-02-29T00:00:00.000Z']:self.assertTrue(checker.is_valid(value),value)
  for value in ['2026-12-31T23:59:60.000Z','2026-02-30T00:00:00.000Z','0000-01-01T00:00:00.000Z','1900-02-29T00:00:00.000Z','2100-02-29T00:00:00.000Z','2026-04-31T00:00:00.000Z','2026-13-01T00:00:00.000Z','2026-10-09T24:00:00.000Z','2026-10-09T00:00:00Z']:
   self.assertFalse(checker.is_valid(value),value)
 def test_https_origin_has_authority_and_no_credentials_or_path(self):
  origin=self.doc['$defs']['ChallengeBody']['properties']['origin'];validator=Draft202012Validator(origin,format_checker=FormatChecker())
  self.assertTrue(validator.is_valid('https://example.invalid:443'))
  for value in ['https://','https:///path','https://user:password@example.invalid','https://example.invalid/path','https://example.invalid?token=x']:
   self.assertFalse(validator.is_valid(value),value)
 def test_version_isolation_rejects_v1_paths_for_v2(self):
  with self.assertRaisesRegex(m.ContractError,'endpoint path outside configured API base'):
   m.generate(self.spec,self.supp,major=2)
  with self.assertRaisesRegex(m.ContractError,'version/API base mismatch'):
   m.generate(self.spec,self.supp,major=1,version='2.0.0')
 def test_selected_catalog_provenance_matches_generated_bundle(self):
  self.assertEqual(self.map['inputs']['catalogSha256'],m.semantic_digest(self.spec))
 def test_no_unconstrained_json_schema_or_unknown_array(self):
  def walk(v):
   if isinstance(v,dict):
    if 'properties' in v:
     for shape in v['properties'].values():self.assertNotEqual(shape,{})
    if v.get('type')=='array':self.assertIn('items',v);self.assertNotEqual(v['items'],{})
    for value in v.values():walk(value)
   elif isinstance(v,list):
    for value in v:walk(value)
  walk(self.doc)
def main(argv=None):
 parser=argparse.ArgumentParser(description=__doc__)
 source=parser.add_mutually_exclusive_group(required=True)
 source.add_argument('--spec',type=pathlib.Path);source.add_argument('--catalog',type=pathlib.Path)
 parser.add_argument('--supplementary',type=pathlib.Path,required=True)
 args=parser.parse_args(argv);bind_inputs(args.supplementary,spec=args.spec,catalog=args.catalog)
 result=unittest.TextTestRunner().run(unittest.defaultTestLoader.loadTestsFromTestCase(MaterializeTests))
 return int(not result.wasSuccessful())
if __name__=='__main__':raise SystemExit(main())
