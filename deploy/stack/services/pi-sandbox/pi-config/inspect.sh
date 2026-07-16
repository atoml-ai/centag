#!/bin/bash
SID="${1:-bigmodel-zh-160705}"
curl -sS "http://localhost:20062/api/sessions/$SID/events?since=0&limit=200" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('total events:', len(d['events']))
for e in d['events']:
    if e['type'] in ('message_start','message_end'):
        m = e.get('payload',{}).get('message',{})
        if m.get('role')=='user':
            print('USER content:', repr(m.get('content')))
            break
asst=[e for e in d['events'] if e['type']=='message_end' and e.get('payload',{}).get('message',{}).get('role')=='assistant']
if asst:
    c=asst[-1]['payload']['message'].get('content')
    if isinstance(c,str): print('ASSISTANT:',c)
    elif isinstance(c,list):
        for p in c:
            if isinstance(p,dict) and 'text' in p: print('ASSISTANT:',p['text'])
"
