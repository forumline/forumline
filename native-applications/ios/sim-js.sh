#!/bin/bash
# Execute JavaScript in the running iOS simulator WKWebView via ios_webkit_debug_proxy
# Usage: ./sim-js.sh 'document.title'
# Requires: ios_webkit_debug_proxy running on localhost:9222, websocat installed

JS="$1"
TARGET_ID="${2:-page-8}"
PORT="${3:-9222}"

# Auto-detect the websocket page endpoint
PAGE_URL=$(curl -s http://localhost:$PORT/json | python3 -c "import sys,json; pages=json.load(sys.stdin); print(pages[0]['webSocketDebuggerUrl'])" 2>/dev/null)
if [ -z "$PAGE_URL" ]; then
  echo "Error: Could not detect debug page. Is ios_webkit_debug_proxy running?"
  exit 1
fi

# JSON-escape the JS expression
ESCAPED_JS=$(python3 -c "import json,sys; print(json.dumps(sys.argv[1]))" "$JS")

# Build the inner message
INNER_MSG="{\"id\":2,\"method\":\"Runtime.evaluate\",\"params\":{\"expression\":$ESCAPED_JS}}"
ESCAPED_INNER=$(python3 -c "import json,sys; print(json.dumps(sys.argv[1]))" "$INNER_MSG")

# Send enable + evaluate, parse response
RESULT=$(
  (echo "{\"id\":1,\"method\":\"Target.sendMessageToTarget\",\"params\":{\"targetId\":\"$TARGET_ID\",\"message\":\"{\\\"id\\\":1,\\\"method\\\":\\\"Runtime.enable\\\"}\"}}"
   sleep 0.3
   echo "{\"id\":2,\"method\":\"Target.sendMessageToTarget\",\"params\":{\"targetId\":\"$TARGET_ID\",\"message\":$ESCAPED_INNER}}"
   sleep 1) | websocat -t $PAGE_URL 2>/dev/null
)

# Extract the result value from the response
echo "$RESULT" | grep "dispatchMessageFromTarget" | tail -1 | python3 -c "
import sys, json
try:
    msg = json.loads(sys.stdin.read())
    inner = json.loads(msg['params']['message'])
    result = inner.get('result', {}).get('result', {})
    if result.get('type') == 'string':
        print(result['value'])
    elif result.get('type') == 'undefined':
        print('[undefined]')
    else:
        print(json.dumps(result, indent=2))
except Exception as e:
    print(f'Error parsing: {e}')
"
