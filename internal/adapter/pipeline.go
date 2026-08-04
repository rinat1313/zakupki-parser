// Package sources — конвейер источников закупок.
// Сначала ЕИС (zakupki.gov.ru) по номеру, затем адаптер по host из URL.
// Новые сайты регистрируются через Register.
package adapter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/rinat1313/zakupki-parser/pkg/collect"
)

// Adapter — модуль конкретного сайта (tektorg SOAP, mos.ru и т.д.).
type Adapter interface {
	// Hosts — домены, которые обслуживает адаптер (без схемы).
	Hosts() []string
	// Fetch — загрузка по URL/номеру. Пока многие адаптеры возвращают ErrNotImplemented.
	Fetch(ctx context.Context, regNumber, sourceURL string) (*collect.Result, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Adapter{}
)

// Register добавляет адаптер (можно вызывать из init пакетов сайтов).
func Register(a Adapter) {
	mu.Lock()
	defer mu.Unlock()
	for _, h := range a.Hosts() {
		registry[strings.ToLower(strings.TrimSpace(h))] = a
	}
}

// HostFromURL извлекает hostname из ссылки или «голого» домена.
func HostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		if strings.Contains(raw, "/") {
			raw = "https://" + raw
		} else {
			return strings.ToLower(raw)
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Lookup возвращает адаптер для URL/хоста.
func Lookup(sourceURL string) Adapter {
	host := HostFromURL(sourceURL)
	if host == "" {
		return nil
	}
	mu.RLock()
	defer mu.RUnlock()
	if a, ok := registry[host]; ok {
		return a
	}
	// www.
	if strings.HasPrefix(host, "www.") {
		if a, ok := registry[strings.TrimPrefix(host, "www.")]; ok {
			return a
		}
	}
	return nil
}

// Pipeline собирает закупку: ЕИС → адаптер по ссылке.
type Pipeline struct {
	EIS *collect.Collector
}

type Outcome struct {
	Result       *collect.Result
	SourceUsed   string // eis | adapter:<host> | none
	CSVSource    string
	FailedAnalyze bool
	Message      string
}

// Resolve выполняет порядок: 1) ЕИС по номеру, 2) адаптер по host ссылки.
func (p *Pipeline) Resolve(ctx context.Context, regNumber, csvSource string) Outcome {
	out := Outcome{CSVSource: csvSource}
	if p.EIS != nil {
		res, err := p.EIS.Collect(ctx, regNumber, "https://zakupki.gov.ru")
		if err == nil && res != nil {
			out.Result = res
			out.SourceUsed = "eis"
			return out
		}
		if err != nil {
			out.Message = "eis: " + err.Error()
		}
	}

	host := HostFromURL(csvSource)
	if host == "" || collect.IsZakupki(csvSource) {
		out.FailedAnalyze = true
		if out.Message == "" {
			out.Message = "не удалось проанализировать на ЕИС"
		}
		out.SourceUsed = "none"
		return out
	}

	ad := Lookup(csvSource)
	if ad == nil {
		out.FailedAnalyze = true
		out.SourceUsed = "none"
		out.Message = fmt.Sprintf("%s; адаптер для %s ещё не подключён", out.Message, host)
		return out
	}
	res, err := ad.Fetch(ctx, regNumber, csvSource)
	if err != nil {
		out.FailedAnalyze = true
		out.SourceUsed = "adapter:" + host
		out.Message = fmt.Sprintf("%s; %s: %v", out.Message, host, err)
		return out
	}
	out.Result = res
	out.SourceUsed = "adapter:" + host
	return out
}

// StubAdapter — заготовка сайта без реализации (mos, tektorg…).
type StubAdapter struct {
	Name  string
	Host  []string
	Hint  string
}

func (s StubAdapter) Hosts() []string { return s.Host }

func (s StubAdapter) Fetch(_ context.Context, _, _ string) (*collect.Result, error) {
	hint := s.Hint
	if hint == "" {
		hint = "модуль будет добавлен позже"
	}
	return nil, fmt.Errorf("%s: %s", s.Name, hint)
}

func init() {
	Register(StubAdapter{Name: "tektorg", Host: []string{"tektorg.ru", "www.tektorg.ru"}, Hint: "планируется SOAP API"})
	Register(StubAdapter{Name: "zakupki.mos", Host: []string{"zakupki.mos.ru"}, Hint: "модуль mos.ru будет добавлен отдельно"})
	Register(StubAdapter{Name: "roseltorg", Host: []string{"roseltorg.ru", "www.roseltorg.ru"}, Hint: "модуль будет добавлен отдельно"})
	Register(StubAdapter{Name: "sberbank-ast", Host: []string{"sberbank-ast.ru", "www.sberbank-ast.ru"}, Hint: "модуль будет добавлен отдельно"})
	Register(StubAdapter{Name: "rts-tender", Host: []string{"rts-tender.ru", "www.rts-tender.ru"}, Hint: "модуль будет добавлен отдельно"})
	Register(StubAdapter{Name: "etp-ets", Host: []string{"etp.ets.ru", "ets.ru"}, Hint: "модуль будет добавлен отдельно"})
	Register(StubAdapter{Name: "fabrikant", Host: []string{"fabrikant.ru", "www.fabrikant.ru"}, Hint: "модуль будет добавлен отдельно"})
	Register(StubAdapter{Name: "gpb", Host: []string{"etp.gpb.ru", "gpb.ru"}, Hint: "модуль будет добавлен отдельно"})
}
