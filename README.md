# zakupki-parser

Go-сервис парсинга закупок с **разных торговых площадок** в одном репозитории/контейнере.

Сейчас:
- **ЕИС** (`zakupki.gov.ru`) — полноценный адаптер (44/223/pricereq)
- остальные ЭТП — stub-адаптеры (tektorg, mos, roseltorg, …), расширяются в `internal/adapter`

## API

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | liveness |
| POST | `/api/v1/fetch` | `{ "reg_number", "source_site" }` → карточка + документы + тексты |
| GET | `/swagger/` | Swagger UI |
| GET | `/openapi.yaml` | OpenAPI 3 спецификация |

Спецификация также лежит в [`docs/openapi.yaml`](docs/openapi.yaml) (копия для embed — `cmd/service/swagger/openapi.yaml`).

Порт по умолчанию: **8091**.

```bash
go run ./cmd/service
# UI: http://127.0.0.1:8091/swagger/
curl -s -X POST http://127.0.0.1:8091/api/v1/fetch \
  -H 'Content-Type: application/json' \
  -d '{"reg_number":"0334500000125000001","source_site":"https://zakupki.gov.ru"}'
```

## Как добавить площадку

1. Реализуйте `adapter.Adapter` (`Hosts`, `Fetch`) в `internal/adapter/<site>/`.
2. Зарегистрируйте в `init()` через `adapter.Register(...)`.
3. Пересоберите сервис — отдельный репозиторий **не нужен**.

## CLI (legacy)

Рядом остаётся исследовательский CLI из бывшего `DataCode` (`cmd/parser`) для файловой выгрузки в `result/`.

## Оркестрация

Полный стек поднимается из [zakupki-platform](https://github.com/rinat1313/zakupki-platform).
