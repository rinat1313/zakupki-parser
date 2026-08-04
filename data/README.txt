DataCode/data/tenders.csv — общий список (44 + 223 + pricereq)
=============================================================
Один файл: номера в любом порядке. Закон можно не указывать —
парсер сам определит 44/223/pricereq на сайте ЕИС (-law=auto по умолчанию).

Формат:
  reg_number,notice_guid,law
  0432200000826003978,,
  32616257756,ddfc7c5e-…,
  32312323655,,
  0303200000126000028,,pricereq

Колонки:
  reg_number   — обязательно (для запроса цен = reestrNumber)
  notice_guid  — опционально (223); если пусто — из HTML
  law          — опционально 44|223|pricereq; если пусто — автодетект

tenders_223.csv оставлен как запасной список только 223.

Суффиксы вроде _1: сначала ищется полный номер, при ошибке — без `_цифры`
  (0103200007726000019_1 → 0103200007726000019).

Параллельность: по умолчанию -workers 1 (ЕИС часто режет массовые запросы).
При ошибке поиска — до -retries 5 с паузой. Probe знает eap20/zkp20/okp20
и отдельно реестр /epz/pricereq (запросы цен).

После прогона:
  result/statuses.csv   — csv_id, resolved_id, status (ok|bad), law, error
  result/statuses.json

Запуск (cwd = DataCode):

  go run ./cmd/parser
  go run ./cmd/parser -csv data/tenders.csv -workers 1 -retries 5
  go run ./cmd/parser -law auto -limit 3
  go run ./cmd/parser -law pricereq -limit 1

  # осторожно: параллель повышает риск пустых ответов ЕИС
  go run ./cmd/parser -workers 3 -retries 5

PDF-сканы: pdftotext → OCR (tesseract rus+eng).
Пустой/крошечный txt → over_info/failed_texts.json и result/failed_texts.json.

