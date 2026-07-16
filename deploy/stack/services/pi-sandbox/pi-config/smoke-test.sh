#!/bin/bash
set -e
BASE=http://localhost:20062
SID="bigmodel-zh-$(date +%H%M%S)"
echo "=== Create session: $SID ==="
curl -sS -X POST "$BASE/api/sessions" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"id\":\"$SID\",\"args\":[\"--provider\",\"bigmodel\",\"--model\",\"glm-4-flash\"]}"
echo
echo "=== Send Chinese prompt ==="
curl -sS -X POST "$BASE/api/prompt" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"session_id\":\"$SID\",\"message\":\"用一句话介绍你自己。\"}"
echo
sleep 12
echo "=== Assistant final text ==="
curl -sS "$BASE/api/sessions/$SID/events?since=0&limit=200" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('total events:', len(d['events']))
asst=[e for e in d['events'] if e['type']=='message_end' and e.get('payload',{}).get('message',{}).get('role')=='assistant']
if asst:
    c=asst[-1]['payload']['message'].get('content')
    if isinstance(c,str): print('ASSISTANT:',c)
    elif isinstance(c,list):
        for p in c:
            if isinstance(p,dict) and 'text' in p: print('ASSISTANT:',p['text'])
"
