#!/bin/bash
BASE=http://localhost:20062
SID="diag-$(date +%H%M%S)"
curl -sS -X POST "$BASE/api/sessions" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"id\":\"$SID\",\"args\":[\"--provider\",\"bigmodel\",\"--model\",\"glm-4-flash\"]}"
echo
curl -sS -X POST "$BASE/api/prompt" \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary "{\"session_id\":\"$SID\",\"message\":\"你好\"}"
echo
sleep 8
curl -sS "$BASE/api/sessions/$SID/events?since=0&limit=50" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for e in d['events']:
    t=e['type']; p=e.get('payload',{})
    err=p.get('errorMessage'); api=p.get('api'); model=p.get('model'); provider=p.get('provider')
    role=p.get('message',{}).get('role') if isinstance(p.get('message'),dict) else None
    text=''
    if role=='assistant':
        c=p.get('message',{}).get('content')
        if isinstance(c,str): text=c[:200]
        elif isinstance(c,list):
            for x in c:
                if isinstance(x,dict) and 'text' in x: text=x['text'][:200]
    info=''
    if err: info=' ERR:'+err[:200]
    if api: info+=' api='+api
    if model: info+=' model='+model
    if provider: info+=' provider='+provider
    if role: info+=' role='+role
    if text: info+=' '+repr(text[:200])
    if info: print(t, info)
"
