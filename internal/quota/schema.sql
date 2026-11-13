-- Proposed schema for the local design review. Apply only to a fresh lab database.
-- The connection/bootstrap contract and Go validation are specified in 01-ACCOUNTING-FA.md.
PRAGMA foreign_keys=ON;
BEGIN IMMEDIATE;
CREATE TABLE store_meta (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 schema_version INTEGER NOT NULL CHECK(schema_version=1),
 store_id TEXT NOT NULL UNIQUE CHECK(length(store_id)>0),
 server_id TEXT NOT NULL CHECK(length(server_id)>0),
 fence INTEGER NOT NULL CHECK(fence>0)
) STRICT;
CREATE TABLE periods (
 account TEXT NOT NULL, period TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('active','frozen','closed')),
 limit_kind TEXT NOT NULL CHECK(limit_kind IN ('finite','unlimited','unknown')),
 limit_bytes INTEGER, expires_kind TEXT NOT NULL CHECK(expires_kind IN ('at','none','unknown')), expires_utc INTEGER,
 sequence INTEGER NOT NULL CHECK(sequence>=0),
 PRIMARY KEY(account,period),
 CHECK((limit_kind='finite' AND limit_bytes IS NOT NULL AND limit_bytes>=0) OR (limit_kind<>'finite' AND limit_bytes IS NULL)),
 CHECK((expires_kind='at' AND expires_utc IS NOT NULL AND expires_utc>=0) OR (expires_kind<>'at' AND expires_utc IS NULL))
) STRICT;
CREATE UNIQUE INDEX one_current ON periods(account) WHERE status IN ('active','frozen');
CREATE TABLE opening (
 account TEXT NOT NULL, period TEXT NOT NULL, origin TEXT NOT NULL, proof_hash BLOB NOT NULL CHECK(length(proof_hash)=32),
 bytes INTEGER NOT NULL CHECK(bytes>=0), PRIMARY KEY(account,period,origin),
 FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
CREATE TABLE tickets (
 account TEXT NOT NULL, period TEXT NOT NULL, op TEXT NOT NULL, meter TEXT NOT NULL,
 direction TEXT NOT NULL CHECK(direction IN ('up','down')), owner_fence INTEGER NOT NULL CHECK(owner_fence>0),
 requested INTEGER NOT NULL CHECK(requested>0), granted INTEGER NOT NULL CHECK(granted>0 AND granted<=requested),
 actual INTEGER CHECK(actual IS NULL OR (actual>=0 AND actual<=granted)),
 PRIMARY KEY(account,period,op), FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
CREATE TABLE source_epochs (
 account TEXT NOT NULL, period TEXT NOT NULL, source TEXT NOT NULL, epoch TEXT NOT NULL,
 direction TEXT NOT NULL CHECK(direction IN ('up','down')), baseline INTEGER NOT NULL CHECK(baseline>=0),
 observed INTEGER NOT NULL CHECK(observed>=baseline),
 incomplete INTEGER NOT NULL CHECK(incomplete IN (0,1)), sampled_utc INTEGER NOT NULL CHECK(sampled_utc>=0),
 PRIMARY KEY(account,period,source,epoch,direction), FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
CREATE TABLE meter_bindings (
 account TEXT NOT NULL, period TEXT NOT NULL, traffic_domain TEXT NOT NULL,
 authority TEXT NOT NULL CHECK(authority IN ('ticket','sample')), source TEXT NOT NULL,
 PRIMARY KEY(account,period,traffic_domain), FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
CREATE TABLE receipts (
 account TEXT NOT NULL, period TEXT NOT NULL, kind TEXT NOT NULL, id TEXT NOT NULL,
 request_hash BLOB NOT NULL CHECK(length(request_hash)=32), result BLOB NOT NULL,
 seq INTEGER NOT NULL CHECK(seq>0), PRIMARY KEY(account,period,kind,id),
 UNIQUE(account,period,seq), FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
CREATE TABLE checkpoints (
 account TEXT NOT NULL, period TEXT NOT NULL, checkpoint_id TEXT NOT NULL,
 coverage_hash BLOB NOT NULL CHECK(length(coverage_hash)=32), manifest_hash BLOB NOT NULL CHECK(length(manifest_hash)=32),
 PRIMARY KEY(account,period,checkpoint_id), FOREIGN KEY(account,period) REFERENCES periods(account,period)
) STRICT;
-- All quantities in the view are audited in Go with checked int64 addition
-- before a state-changing commit. SQLite integer SUM overflow aborts the read.
CREATE VIEW quantities AS
 SELECT p.account,p.period,
 COALESCE((SELECT SUM(b.bytes) FROM opening b WHERE (b.account,b.period)=(p.account,p.period)),0) AS opening_bytes,
 COALESCE((SELECT SUM(t.actual) FROM tickets t WHERE (t.account,t.period)=(p.account,p.period)),0) AS settled_bytes,
 COALESCE((SELECT SUM(s.observed-s.baseline) FROM source_epochs s WHERE (s.account,s.period)=(p.account,p.period)),0) AS sampled_bytes,
 COALESCE((SELECT SUM(t.granted) FROM tickets t WHERE (t.account,t.period)=(p.account,p.period) AND t.actual IS NULL),0) AS pending_bytes
 FROM periods p;
CREATE TABLE limit_observations(account TEXT NOT NULL,period TEXT NOT NULL,monotonic_ns INTEGER NOT NULL CHECK(monotonic_ns>=0),confirmed INTEGER NOT NULL CHECK(confirmed>=0),PRIMARY KEY(account,period),FOREIGN KEY(account,period) REFERENCES periods(account,period)) STRICT;
PRAGMA user_version=1;
COMMIT;
