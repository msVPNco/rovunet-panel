package quota
import("context";"testing";"path/filepath")
func TestUnlimitedImportReplayAndObservation(t *testing.T){
 ctx:=context.Background();d:=filepath.Join(t.TempDir(),"db");s,e:=Create(ctx,d,"server","store");if e!=nil{t.Fatal(e)};defer s.Close()
 k:=Key{"account","period"}
 if e=s.ImportUnlimited(ctx,k,"operation","proof",300,1000);e!=nil{t.Fatal(e)}
 if e=s.FinishBootstrap(d);e!=nil{t.Fatal(e)}
 if e=s.ObserveUnlimited(ctx,k,1050,1);e!=nil{t.Fatal(e)}
 if e=s.ImportUnlimited(ctx,k,"operation","proof",300,1000);e!=nil{t.Fatal(e)}
 m,e:=s.Metadata(ctx,k);if e!=nil||m.Confirmed!=350||m.Pending!=0||m.LimitKind!="unlimited"||m.Upload!=nil||m.Download!=nil{t.Fatal(m,e)}
 if e=s.ObserveUnlimited(ctx,k,1049,2);e==nil{t.Fatal("regression accepted")}
 if e=s.ImportUnlimited(ctx,k,"other","proof",300,1000);e==nil{t.Fatal("new opening accepted")}
 if e=s.ReserveNative(ctx,k,"ticket","up",1);e==nil{t.Fatal("sample-only unlimited granted native authority")}
 if e=s.ObserveUnlimited(ctx,k,1050,2);e!=nil{t.Fatal(e)}
 a,e:=s.Read(ctx,k);if e!=nil||a.Confirmed!=350{t.Fatal(a,e)}
}

