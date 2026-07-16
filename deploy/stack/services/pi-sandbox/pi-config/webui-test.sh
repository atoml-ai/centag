#!/bin/bash
BASE=http://localhost:20062
SID="webui-$(date +%H%M%S)"
# 模拟 Web UI：只传 id，不传 --provider/--model
curl -sS -X POST "$BASE/api/sessions" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"id\":\"$SID\"}"
echo
curl -sS -X POST "$BASE/api/prompt" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"session_id\":\"$SID\",\"message\":\"你好，请用中文一句话介绍你自己。\"}"
echo
sleep 8
curl -sS "$BASE/api/sessions/$SID/events?since=0&limit=50" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('total events:', len(d['events']))
for e in d['events']:
    t=e['type']; p=e.get('payload',{})
    m=p.get('message')
    if isinstance(m,dict) and m.get('role')=='assistant':
        c=m.get('content')
        txt=''
        if isinstance(c,str): txt=c
        elif isinstance(c,list):
            for x in c:
                if isinstance(x,dict) and 'text' in x: txt+=x['text']
        print('ASSISTANT (api=%s model=%s provider=%s): %s' % (m.get('api','?'), m.get('model','?'), m.get('provider','?'), txt[:300]))
"
