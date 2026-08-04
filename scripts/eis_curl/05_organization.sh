#!/usr/bin/env bash
# 05 — организации 44-ФЗ и 223-ФЗ
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

eis_curl "${OUT_DIR}/05_org44.html" \
  "${BASE}/epz/organization/view/info.html?organizationCode=${ORG_44_CODE}&tab=info"

eis_curl "${OUT_DIR}/05_org223.html" \
  "${BASE}/epz/organization/view223/info.html?agencyId=${ORG_223_AGENCY}&tab=info"
