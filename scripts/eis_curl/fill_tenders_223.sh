#!/usr/bin/env bash
# fill_tenders_223.sh — добрать свежие номера 223-ФЗ из расширенного поиска ЕИС
# в data/tenders_223.csv (без дублей).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="${1:-$ROOT/data/tenders_223.csv}"
TMP="$(mktemp)"
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36'
BASE='https://zakupki.gov.ru'

curl -ksS --connect-timeout 30 --max-time 120 -A "$UA" -o "$TMP" \
  --get "$BASE/epz/order/extendedsearch/results.html" \
  --data-urlencode "morphology=on" \
  --data-urlencode "search-filter=Дате размещения" \
  --data-urlencode "pageNumber=1" \
  --data-urlencode "sortDirection=false" \
  --data-urlencode "recordsPerPage=_50" \
  --data-urlencode "sortBy=UPDATE_DATE" \
  --data-urlencode "fz223=on" \
  --data-urlencode "af=on" \
  --data-urlencode "ca=on" \
  --data-urlencode "pc=on" \
  --data-urlencode "pa=on" \
  --data-urlencode "currencyIdGeneral=-1"

python3 - "$TMP" "$OUT" <<'PY'
import re, sys, csv, os
html_path, out_path = sys.argv[1], sys.argv[2]
html = open(html_path, encoding='utf-8', errors='ignore').read()
found = []
seen = set()
for m in re.finditer(r'notice223/common-info\.html\?([^"\']+)', html):
    q = m.group(1).replace('&amp;', '&')
    params = dict(x.split('=', 1) for x in q.split('&') if '=' in x)
    n = params.get('regNumber') or params.get('purchaseNoticeNumber')
    g = params.get('noticeGuid') or params.get('purchaseNoticeGuid') or ''
    if not n or n in seen:
        continue
    seen.add(n)
    found.append((n, g))

existing = {}
if os.path.exists(out_path):
    with open(out_path, newline='', encoding='utf-8') as f:
        r = csv.DictReader(f)
        for row in r:
            n = (row.get('reg_number') or '').strip()
            g = (row.get('notice_guid') or '').strip()
            if n:
                existing[n] = g

for n, g in found:
    if n not in existing:
        existing[n] = g
    elif not existing[n] and g:
        existing[n] = g

os.makedirs(os.path.dirname(out_path) or '.', exist_ok=True)
with open(out_path, 'w', newline='', encoding='utf-8') as f:
    w = csv.writer(f)
    w.writerow(['reg_number', 'notice_guid'])
    for n, g in existing.items():
        w.writerow([n, g])

print(f'wrote {len(existing)} rows → {out_path} (new from search: {len(found)})')
PY

rm -f "$TMP"
