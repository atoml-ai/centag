#!/bin/bash
curl -sS "http://localhost:20062/api/sessions/diag-161928/events?since=0&limit=50" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for e in d['events']:
    t=e['type']; p=e.get('payload',{})
    m=p.get('message')
    if isinstance(m,dict) and m.get('role')=='assistant':
        print(t, 'api='+str(m.get('api','?')), 'model='+str(m.get('model','?')), 'provider='+str(m.get('provider','?')))
    err=p.get('errorMessage')
    if err: print(t, 'ERROR:', err[:300])
"
