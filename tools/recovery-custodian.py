"""Independent operator custody with an independently retained durable witness.
Never distribute either private directory; release signing identity is separate.
"""
from pathlib import Path
import argparse,base64,contextlib,fcntl,hashlib,json,os,re,signal
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization
MAX=(1<<63)-1
FIELDS={'version','sequence','status','instance','snapshot','operation','seal_operation','target'}
def raw(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
def digest(data):return hashlib.sha256(data).hexdigest()
def unique(pairs):
 out={}
 for k,v in pairs:
  if k in out:raise ValueError('duplicate JSON member')
  out[k]=v
 return out
def parse(data):return json.loads(data,object_pairs_hook=unique)
def sync(directory):
 fd=os.open(directory,os.O_RDONLY|os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def atomic(path,data):
 path=Path(path);temp=path.with_name(path.name+'.new')
 fd=os.open(temp,os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'wb') as f:
  os.fchmod(f.fileno(),0o600);f.write(data);f.flush();os.fsync(f.fileno())
 os.replace(temp,path);sync(path.parent)
def remove(path):path.unlink();sync(path.parent)
def barrier(name):
 if os.getenv('ROVUNET_CUSTODY_KILL_AT')==name:os.kill(os.getpid(),signal.SIGKILL)
def encode(value,key):
 payload=raw(value)
 return raw({'payload':base64.b64encode(payload).decode(),'signature':base64.b64encode(key.sign(payload)).decode()})
def valid_id(value,n):return isinstance(value,str) and re.fullmatch('[0-9a-f]{'+str(n)+'}',value) is not None
def validate(state):
 if set(state)!=FIELDS or type(state['version']) is not int or state['version']!=1:raise ValueError('unsupported custody state')
 seq=state['sequence']
 if type(seq) is not int or not 0<=seq<=MAX:raise ValueError('invalid custody sequence')
 if state['status']=='initialized':
  if seq!=0 or any(state[k]!='' for k in ['instance','snapshot','operation','seal_operation','target']):raise ValueError('invalid genesis')
 elif state['status'] in ['sealed','claimed','released']:
  if seq<=0 or not all(valid_id(state[k],32) for k in ['instance','operation','seal_operation']) or not valid_id(state['snapshot'],64):raise ValueError('invalid custody binding')
  if state['status']=='claimed':
   if not valid_id(state['target'],32):raise ValueError('invalid target')
  elif state['target']!='':raise ValueError('unexpected target')
 else:raise ValueError('unknown state')
def decode(envelope,key):
 data=parse(envelope)
 if set(data)!={'payload','signature'}:raise ValueError('invalid envelope')
 payload=base64.b64decode(data['payload'],validate=True)
 key.public_key().verify(base64.b64decode(data['signature'],validate=True),payload)
 value=parse(payload);validate(value);return value
def public(key):return key.public_key().public_bytes(serialization.Encoding.Raw,serialization.PublicFormat.Raw).hex()
def load_key(root):return Ed25519PrivateKey.from_private_bytes((root/'key').read_bytes())
def document(envelope,key):return raw({'format':2,'identity':public(key),'envelope':parse(envelope)})
def read_document(data,key):
 value=parse(data)
 if set(value)!={'format','identity','envelope'} or value['format']!=2 or value['identity']!=public(key):raise ValueError('custody identity mismatch')
 env=raw(value['envelope']);state=decode(env,key);return data,state,env
def read_head(path,key):return read_document(path.read_bytes(),key)
@contextlib.contextmanager
def locked(root,create=False):
 fd=os.open(root/'lock',os.O_RDWR|os.O_NOFOLLOW|(os.O_CREAT if create else 0),0o600)
 with os.fdopen(fd,'r+b') as f:
  fcntl.flock(f,fcntl.LOCK_EX);yield
def marker(root,key,witness=None):
 value=parse((root/'initialized.json').read_bytes())
 if set(value)!={'format','identity','witness'} or value['format']!=2 or value['identity']!=public(key):raise ValueError('invalid completion marker')
 w=Path(value['witness'])
 if not w.is_absolute() or (witness is not None and w!=Path(witness).resolve()):raise ValueError('witness binding mismatch')
 if w==root or root in w.parents:raise ValueError('witness must be independent')
 if (root/'public-key').read_text().strip()!=public(key):raise ValueError('public key mismatch')
 return w
def initialize(root,witness):
 if witness is None:raise ValueError('explicit independent witness directory required')
 w=Path(witness).resolve()
 if w==root or root in w.parents:raise ValueError('witness must be independent')
 if not root.exists():root.mkdir(mode=0o700);sync(root.parent)
 with locked(root,True):
  if (root/'initialized.json').exists():
   key=load_key(root);marker(root,key,w);load_current(root,key,w);return public(key).encode()
  pending=root/'initializing.json'
  if not pending.exists():
   if any(p.name!='lock' for p in root.iterdir()):raise ValueError('legacy/incomplete authority requires explicit migration')
   if w.exists():raise ValueError('witness must be fresh')
   atomic(pending,raw({'format':2,'witness':str(w),'operation':os.urandom(16).hex()}))
  plan=parse(pending.read_bytes())
  if plan.get('format')!=2 or plan.get('witness')!=str(w):raise ValueError('initialization binding mismatch')
  if not (root/'key').exists():
   if any((root/n).exists() for n in ['public-key','state.json','initialized.json']) or (w/'head.json').exists():raise ValueError('key missing; never rotate automatically')
   key=Ed25519PrivateKey.generate()
   atomic(root/'key',key.private_bytes(serialization.Encoding.Raw,serialization.PrivateFormat.Raw,serialization.NoEncryption()))
  key=load_key(root);barrier('init-key')
  if (root/'public-key').exists() and (root/'public-key').read_text()!=public(key):raise ValueError('public key mismatch')
  atomic(root/'public-key',public(key).encode())
  if not w.exists():w.mkdir(mode=0o700);sync(w.parent)
  initial={'version':1,'sequence':0,'status':'initialized','instance':'','snapshot':'','operation':'','seal_operation':'','target':''}
  data=document(encode(initial,key),key)
  for path in [root/'state.json',w/'head.json']:
   if path.exists():
    before,state,_=read_head(path,key)
    if before!=data or state!=initial:raise ValueError('non-genesis state during initialization')
   else:atomic(path,data)
  barrier('init-state')
  atomic(root/'initialized.json',raw({'format':2,'identity':public(key),'witness':str(w)}));barrier('init-complete')
  if pending.exists():remove(pending)
  return public(key).encode()
def load_current(root,key,w):
 # A lost state is never reconstructed on a normal operation.
 data,state,env=read_head(root/'state.json',key)
 head,external,_=read_head(w/'head.json',key)
 intent=root/'transaction.json'
 if intent.exists():
  tx=parse(intent.read_bytes())
  if set(tx)!={'format','identity','before','after'} or tx['format']!=2 or tx['identity']!=public(key):raise ValueError('invalid pending transaction')
  after=base64.b64decode(tx['after'],validate=True);_,next_state,_=read_document(after,key)
  for old,old_state in [(data,state),(head,external)]:
   if old!=after and (digest(old)!=tx['before'] or next_state['sequence']!=old_state['sequence']+1):raise ValueError('pending commit conflicts with high-water')
  if data!=after:atomic(root/'state.json',after)
  if head!=after:atomic(w/'head.json',after)
  remove(intent);data,state,env=read_head(root/'state.json',key)
 elif data!=head:raise ValueError('custody stale/diverged from independent high-water; explicit repair required')
 return data,state,env
def migrate(root,witness,trusted_envelope,trusted_hash):
 if witness is None or trusted_envelope is None or not valid_id(trusted_hash,64):raise ValueError('independently trusted latest envelope/hash required')
 key=load_key(root);reference=Path(trusted_envelope).resolve()
 if reference==root or root in reference.parents:raise ValueError('migration reference must be outside custody')
 env=reference.read_bytes()
 if digest(env)!=trusted_hash:raise ValueError('migration reference hash mismatch')
 state=decode(env,key)
 if state['status']=='initialized':raise ValueError('legacy history required')
 w=Path(witness).resolve()
 if w==root or root in w.parents:raise ValueError('witness must be independent')
 with locked(root,True):
  if (root/'initialized.json').exists():
   marker(root,key,w);_,current,_=load_current(root,key,w)
   if current!=state:raise ValueError('migration cannot replace later authority')
   return public(key).encode()
  if (root/'state.json').exists():
   old=parse((root/'state.json').read_bytes())
   if old.get('format')==2:_,old,_=read_document((root/'state.json').read_bytes(),key)
   else:validate(old)
   if old!=state:raise ValueError('legacy state conflicts with trusted last claim')
  data=document(env,key);pending=root/'migrating.json'
  expected={'format':2,'identity':public(key),'witness':str(w),'head':digest(data)}
  if pending.exists():
   if parse(pending.read_bytes())!=expected:raise ValueError('another migration is pending')
  else:
   if w.exists() and (w/'head.json').read_bytes()!=data:raise ValueError('witness conflict')
   atomic(pending,raw(expected))
  barrier('migrate-prepared')
  if not w.exists():w.mkdir(mode=0o700);sync(w.parent)
  if (w/'head.json').exists() and (w/'head.json').read_bytes()!=data:raise ValueError('witness conflict')
  atomic(root/'public-key',public(key).encode());atomic(root/'state.json',data);barrier('migrate-state')
  atomic(w/'head.json',data)
  atomic(root/'initialized.json',raw({'format':2,'identity':public(key),'witness':str(w)}));barrier('migrate-complete')
  remove(pending);return public(key).encode()
def repair(root,trusted_hash):
 key=load_key(root)
 with locked(root):
  w=marker(root,key);head,state,env=read_head(w/'head.json',key)
  if not valid_id(trusted_hash,64) or digest(head)!=trusted_hash:raise ValueError('independently trusted latest head hash required')
  if (root/'transaction.json').exists():raise ValueError('pending commit needs forensic recovery; never discard it')
  if (root/'state.json').exists():
   try:_,old,_=read_head(root/'state.json',key)
   except Exception:old=None
   if old is not None and old['sequence']>state['sequence']:raise ValueError('witness older than surviving state')
  atomic(root/'state.json',head);return env
def operate(root,action,instance=None,snapshot=None,operation=None,target=None,witness=None,trusted_envelope=None,trusted_hash=None):
 root=Path(root).resolve()
 if action=='init':return initialize(root,witness)
 if action=='migrate-legacy':return migrate(root,witness,trusted_envelope,trusted_hash)
 if action=='repair':return repair(root,trusted_hash)
 key=load_key(root)
 with locked(root):
  w=marker(root,key,witness);before,state,env=load_current(root,key,w)
  if action=='replay':atomic(root/'latest.envelope',env);return env
  if not valid_id(operation,32) or not valid_id(snapshot,64):raise ValueError('invalid operation/checkpoint')
  if action=='seal':
   if not valid_id(instance,32):raise ValueError('invalid instance')
   if state['status']=='sealed' and state['snapshot']==snapshot and state['operation']==operation:return env
   if state['status']=='sealed':raise ValueError('another sealed checkpoint exists')
   if state['status']=='claimed' and state['target']!=instance:raise ValueError('only last recovery target may seal')
   state={'version':1,'sequence':state['sequence']+1,'status':'sealed','instance':instance,'snapshot':snapshot,'operation':operation,'seal_operation':operation,'target':''}
  elif action=='claim':
   if not valid_id(target,32):raise ValueError('invalid target')
   if state['status']=='claimed':
    if state['snapshot']==snapshot and state['operation']==operation and state['target']==target:return env
    raise ValueError('checkpoint already claimed')
   if state['status']!='sealed' or state['snapshot']!=snapshot:raise ValueError('not current sealed checkpoint')
   state.update(sequence=state['sequence']+1,status='claimed',target=target,operation=operation)
  elif action=='release':
   if state['status']=='released' and state['instance']==instance and state['snapshot']==snapshot and state['operation']==operation:return env
   if state['status']!='sealed' or state['instance']!=instance or state['snapshot']!=snapshot:raise ValueError('checkpoint cannot resume at source')
   state.update(sequence=state['sequence']+1,status='released',operation=operation)
  else:raise ValueError('unknown action')
  validate(state);env=encode(state,key);after=document(env,key)
  atomic(root/'transaction.json',raw({'format':2,'identity':public(key),'before':digest(before),'after':base64.b64encode(after).decode()}));barrier('prepared')
  atomic(root/'state.json',after);barrier('state-committed')
  atomic(w/'head.json',after);barrier('witness-committed')
  remove(root/'transaction.json');barrier('committed')
  atomic(root/'latest.envelope',env);return env
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('action',choices=['init','seal','claim','release','replay','repair','migrate-legacy']);parser.add_argument('directory')
 for name in ['instance','snapshot','operation','target','output','witness','trusted-envelope','trusted-hash']:parser.add_argument('--'+name)
 a=parser.parse_args()
 if a.action not in ['init','migrate-legacy'] and not a.output:parser.error('provide --output before mutation')
 result=operate(a.directory,a.action,a.instance,a.snapshot,a.operation,a.target,a.witness,a.trusted_envelope,a.trusted_hash)
 if a.output:atomic(Path(a.output),result)
 elif a.action in ['init','migrate-legacy']:print(result.decode())

