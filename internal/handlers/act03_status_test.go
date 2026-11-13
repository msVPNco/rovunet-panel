package handlers

import (
 "os"
 "path/filepath"
 "testing"
 "strings"
 "github.com/msVPNco/rovunet-panel/internal/config"
)

func TestAct03Snapshots(t *testing.T){
 dir:=os.Getenv("ACT03_STATUS");if dir==""{t.Skip("fixture generation only")}
 u:=config.UserConfig{Username:"fixture",AccountID:strings.Repeat("ab",16),PeriodID:strings.Repeat("cd",16),QuotaGB:1,UsageBase:100}
 path:=filepath.Join(t.TempDir(),"account.status.json")
 for _,name:=range []string{"active","partial","expired","renewed","limit-reached"}{
  n:=int64(1234);reason:=""
  if name=="expired"{reason="expired"}
  if name=="renewed"{u.PeriodID=strings.Repeat("ef",16);u.UsageBase=n}
  if name=="limit-reached"{n+=1<<30;reason="traffic limit"}
  input:=statusTestInput(map[string]int64{"fixture":n},true,name!="partial"); input.serverID=strings.Repeat("12",16)
  b,e:=statusBody(buildStatus(input,u,reason),path);if e!=nil{t.Fatal(e)}
  for _,f:=range []string{path,filepath.Join(dir,name+".status.json")}{if e=os.WriteFile(f,b,0600);e!=nil{t.Fatal(e)}}
 }
}
