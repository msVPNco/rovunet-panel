from pathlib import Path
import json
MAX_INT64=(1<<63)-1
def nullable_int():
 return {"type":["integer","null"],"minimum":0,"maximum":MAX_INT64}
time={"type":["string","null"],"format":"date-time"}
id={"type":"string","pattern":"^[0-9a-f]{32}$"}
properties={"version":{"const":2},"server_id":id,"account_id":id,"period_id":id,"revision":{"type":"integer","minimum":1,"maximum":MAX_INT64},"account_status":{"enum":["active","expired","limit_reached","hold","archived"]},"limit_kind":{"enum":["finite","unlimited","unknown"]},"limit_bytes":nullable_int(),"expiry_kind":{"enum":["at","none","unknown"]},"expires_at":time,"confirmed_bytes":nullable_int(),"pending_bytes":nullable_int(),"coverage":{"enum":["partial","unavailable","not_metered"]},"measurement":{"enum":["accounted_lower_bound","none","unknown"]},"sampled_at":time,"generated_at":{"type":"string","format":"date-time"}}
schema={"$schema":"https://json-schema.org/draft/2020-12/schema","title":"Rovunet private status version 2","type":"object","additionalProperties":False,"required":list(properties),"properties":properties,"allOf":[{"if":{"properties":{"limit_kind":{"const":"finite"}}},"then":{"properties":{"limit_bytes":{"type":"integer","minimum":1,"maximum":MAX_INT64}}},"else":{"properties":{"limit_bytes":{"type":"null"}}}},{"if":{"properties":{"expiry_kind":{"const":"at"}}},"then":{"properties":{"expires_at":{"type":"string","format":"date-time"}}},"else":{"properties":{"expires_at":{"type":"null"}}}}]}
destination=Path(__file__).resolve().parents[1]/"docs/contracts/status-v2.schema.json"
destination.write_text(json.dumps(schema,indent=2)+"\n")
