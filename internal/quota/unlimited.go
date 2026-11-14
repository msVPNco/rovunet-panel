package quota

import (
 "context"
 "crypto/sha256"
 "bytes"
 "database/sql"
 "errors"
)

// ImportUnlimited installs the already-recorded lower bound, and its legacy
// aggregate watermark, atomically. It cannot create transfer authorization.
func (s *Store) ImportUnlimited(ctx context.Context, k Key, op, proof string, opening, watermark int64) error {
 if !valid(k.Account)||!valid(k.Period)||!valid(op)||!valid(proof)||opening<0||watermark<opening { return ErrConflict }
 return s.write(ctx,func(ctx context.Context,c *sql.Conn) error {
  var kind string
  e:=c.QueryRowContext(ctx,"SELECT limit_kind FROM periods WHERE account=? AND period=?",k.Account,k.Period).Scan(&kind)
  if e==nil {
   var origin string;var n,baseline,observed int64;var saved []byte
   if kind!="unlimited"{return ErrConflict}
   if e=c.QueryRowContext(ctx,"SELECT origin,bytes,proof_hash FROM opening WHERE account=? AND period=?",k.Account,k.Period).Scan(&origin,&n,&saved);e!=nil{return e}
   if e=c.QueryRowContext(ctx,"SELECT baseline,observed FROM source_epochs WHERE account=? AND period=? AND source='legacy-aggregate' AND epoch=?",k.Account,k.Period,op).Scan(&baseline,&observed);e!=nil{return e}
   if origin!=op||n!=opening||!bytes.Equal(saved,digestUnlimited(proof))||baseline!=watermark||observed<baseline{return ErrConflict}
   return nil
  }
  if !errors.Is(e,sql.ErrNoRows) {return e}
  if _,e=c.ExecContext(ctx,"INSERT INTO periods VALUES(?,?,'active','unlimited',NULL,'none',NULL,1)",k.Account,k.Period);e!=nil{return e}
  if _,e=c.ExecContext(ctx,"INSERT INTO opening VALUES(?,?,?,?,?)",k.Account,k.Period,op,digestUnlimited(proof),opening);e!=nil{return e}
  if _,e=c.ExecContext(ctx,"INSERT INTO meter_bindings VALUES(?,?,'account-total','sample','legacy-aggregate')",k.Account,k.Period);e!=nil{return e}
  // The aggregate is not a direction. Its delta is deliberately stored as an
  // incomplete sampled source; metadata must never fabricate upload/download.
  _,e=c.ExecContext(ctx,"INSERT INTO source_epochs VALUES(?,?,'legacy-aggregate',?,'up',?,?,1,0)",k.Account,k.Period,op,watermark,watermark)
  return e
 })
}

// ObserveUnlimited never resets a watermark, even after source loss/regression.
func(s *Store) ObserveUnlimited(ctx context.Context,k Key,absolute,stamp int64) error {
 if absolute<0||stamp<0{return ErrConflict}
 return s.write(ctx,func(ctx context.Context,c *sql.Conn)error{
  var kind,state,source string
  if e:=c.QueryRowContext(ctx,"SELECT limit_kind,status FROM periods WHERE account=? AND period=?",k.Account,k.Period).Scan(&kind,&state);e!=nil{return e}
  if kind!="unlimited"||state!="active"{return ErrConflict}
  if e:=c.QueryRowContext(ctx,"SELECT source FROM meter_bindings WHERE account=? AND period=? AND traffic_domain='account-total' AND authority='sample'",k.Account,k.Period).Scan(&source);e!=nil{return e}
  if source!="legacy-aggregate"{return ErrConflict}
  var old int64
  if e:=c.QueryRowContext(ctx,"SELECT observed FROM source_epochs WHERE account=? AND period=? AND source='legacy-aggregate'",k.Account,k.Period).Scan(&old);e!=nil{return e}
  if absolute<old{return ErrConflict}
  if absolute==old{return nil}
  if _,e:=c.ExecContext(ctx,"UPDATE source_epochs SET observed=?,sampled_utc=? WHERE account=? AND period=? AND source='legacy-aggregate'",absolute,stamp,k.Account,k.Period);e!=nil{return e}
  if _,e:=amount(ctx,c,k);e!=nil{return e}
  r,e:=c.ExecContext(ctx,"UPDATE periods SET sequence=sequence+1 WHERE account=? AND period=? AND sequence<9223372036854775807",k.Account,k.Period)
  if e!=nil{return e};n,e:=r.RowsAffected();if e!=nil{return e};if n!=1{return ErrConflict};return nil
 })
}


func digestUnlimited(s string)[]byte {h:=sha256.Sum256([]byte(s));return h[:]}
