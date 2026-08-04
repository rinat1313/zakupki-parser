Тестовые curl к zakupki.gov.ru
==============================
Проверено вручную 2026-08-02 (нужен флаг -k из‑за SSL на части Mac).

КАК ЗАПУСТИТЬ ИЗ IDE / ТЕРМИНАЛА
--------------------------------
Из корня проекта «анализ госзакупки»:

  bash DataCode/scripts/eis_curl/01_home.sh
  bash DataCode/scripts/eis_curl/02_notice44.sh
  bash DataCode/scripts/eis_curl/03_notice223.sh
  bash DataCode/scripts/eis_curl/04_search.sh
  bash DataCode/scripts/eis_curl/04_search.sh "Разработка ПО"
  bash DataCode/scripts/eis_curl/05_organization.sh
  bash DataCode/scripts/eis_curl/06_documents_list.sh
  bash DataCode/scripts/eis_curl/07_download_file.sh

Все сразу (с паузой 1с между шагами):

  bash DataCode/scripts/eis_curl/run_all.sh

Результаты пишутся в:
  DataCode/scripts/eis_curl/out/


СКРИПТЫ
-------
01_home.sh            — главная
02_notice44.sh        — извещение 44 HTML + printForm XML
03_notice223.sh       — извещение 223
04_search.sh [query]  — поиск (по умолчанию «Разработка ПО»)
05_organization.sh    — org 44 + org 223
06_documents_list.sh  — страницы документов + список filestore URL
07_download_file.sh   — скачать по одному файлу 44 и 223
_common.sh            — BASE URL, номера, CURL_OPTS (-k), eis_curl()
run_all.sh            — прогон всего


ПАРАМЕТРЫ (править в _common.sh)
--------------------------------
NOTICE_44, NOTICE_223, NOTICE_223_GUID
ORG_44_CODE, ORG_223_AGENCY
FILE_UID_44, FILE_UID_223   ← не называть UID (zsh)


SSL
---
curl error 60 без -k — нормально на части окружений.
В скриптах уже есть -k. В Go-клиенте позже настроить CA отдельно.


СМ. ТАКЖЕ
---------
result/14_curl_tests.txt — краткая шпаргалка и ссылка сюда
