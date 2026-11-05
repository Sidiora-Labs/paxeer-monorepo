const fs=require('fs'), path=require('path'), crypto=require('crypto'),solc=require('solc');
const root=path.resolve(__dirname,'../../../../../..'), base=path.join(root,'contracts/launchpad');
const entries={'EventEmitter':['contracts/data/EventEmitter.sol','EventEmitter'],'PoolBeacon':['contracts/base/UpgradeableBeacon.sol','UpgradeableBeacon'],'ERC1967Proxy':['contracts/meta-ag/proxy/ERC1967Proxy.sol','ERC1967Proxy']};
const sources={}; function load(p){if(sources[p])return;let content=fs.readFileSync(path.join(base,p),'utf8');sources[p]={content};for(const m of content.matchAll(/import\s+(?:[^;]*?from\s+)?["']([^"']+)["']\s*;/g))load(path.posix.normalize(path.posix.join(path.posix.dirname(p),m[1])));}
for(const [p] of Object.values(entries))load(p);
const settings={optimizer:{enabled:true,runs:200},viaIR:true,outputSelection:{'*':{'*':['abi','evm.bytecode.object']}}};
const out=JSON.parse(solc.compile(JSON.stringify({language:'Solidity',sources,settings})));
for(const e of out.errors||[])if(e.severity==='error')throw Error(e.formattedMessage);
const dir=path.join(root,'sdk/launchpad/protocol/bindings/testdata/repair37');
for(const [name,[p,c]] of Object.entries(entries)){const a=out.contracts[p][c];fs.writeFileSync(path.join(dir,name+'.json'),JSON.stringify({compiler:solc.version(),source:p,contract:c,settings,sourceHashes:Object.fromEntries(Object.entries(sources).map(([p,s])=>[p,crypto.createHash('sha256').update(s.content).digest('hex')])),abi:a.abi,bytecode:'0x'+a.evm.bytecode.object},null,2)+'\n');}
