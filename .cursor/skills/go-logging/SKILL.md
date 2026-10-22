---
name: go-logging
description: Настраивает логирование функций в Go через log/slog. Use when adding logging, timers around functions, LOG_LEVEL, log files, slog, or debug vs production logs.
---

# Go logging

Стандарт проекта: пакет **`log/slog`** (stdlib). Не подключай сторонние логгеры без явной просьбы.

## Цели

1. Логировать время срабатывания рабочих функций
2. Писать логи в файл
3. Менять уровень логирования через конфигурацию

## Уровни

| Уровень | Когда | Что писать |
| --- | --- | --- |
| `debug` | локальная отладка | вход/выход функций, длительность, промежуточные шаги |
| `info` | прод по умолчанию | старт/стоп сервиса, важные бизнес-события, длительность ключевых функций |
| `warn` | прод | восстановленные сбои, деградация, ретраи |
| `error` | всегда | ошибки, из-за которых операция не удалась |

Уровень задаётся конфигом/env: `LOG_LEVEL=debug|info|warn|error` (по умолчанию `info`).

## Время выполнения функций

```go
func DoWork(ctx context.Context, id string) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("DoWork done",
			"id", id,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()

	slog.Debug("DoWork start", "id", id)
	return doWork(ctx, id)
}
```

Правила:

- Имя события = имя функции + суффикс (`start` / `done`)
- Длительность — поле `duration_ms` (миллисекунды, int64)
- На `debug`: логируй `start` и `done`
- На `info` (прод): логируй `done` для ключевых операций
- Ошибку в defer бери через named return `err`

## Что не логировать

- пароли, токены, секреты
- огромные тела запросов/ответов на уровне info
- шум от горячих циклов на info
