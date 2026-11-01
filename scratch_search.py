with open('f:/GameAP_WebMonitoring/frontend/admin_bundle.js', 'r', encoding='utf-8') as f:
    text = f.read()

import re
matches = list(re.finditer(r'type:\s*[\'\"]checkbox[\'\"]', text))
print(f"Total checkbox matches: {len(matches)}")
for idx, m in enumerate(matches):
    start = max(0, m.start() - 100)
    end = min(len(text), m.end() + 250)
    print(f"--- MATCH {idx} ---")
    print(text[start:end])
