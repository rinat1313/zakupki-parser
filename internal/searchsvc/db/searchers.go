package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

const searcherSelect = `
	s.id::text, s.user_id::text, s.name, s.config, s.auto_ai,
	COALESCE((SELECT COUNT(*)::int FROM searcher_tenders t WHERE t.searcher_id = s.id), 0) AS tenders_count,
	s.created_at, s.updated_at, s.last_run_at`

func (s *Store) ListSearchers(ctx context.Context, userID string) ([]models.Searcher, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+searcherSelect+`
		FROM searchers s
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Searcher, 0)
	for rows.Next() {
		item, err := scanSearcher(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetSearcher(ctx context.Context, userID, id string) (models.Searcher, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+searcherSelect+`
		FROM searchers s
		WHERE s.id = $1 AND s.user_id = $2`, id, userID)
	item, err := scanSearcher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateSearcher(ctx context.Context, userID string, name string, cfg models.SearcherConfig, autoAI bool) (models.Searcher, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return models.Searcher{}, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO searchers (user_id, name, config, auto_ai)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id::text, user_id::text, name, config, auto_ai,
			0::int, created_at, updated_at, last_run_at`,
		userID, name, string(raw), autoAI,
	)
	return scanSearcher(row)
}

func (s *Store) UpdateSearcher(ctx context.Context, userID, id string, name string, cfg models.SearcherConfig, autoAI bool) (models.Searcher, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return models.Searcher{}, err
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE searchers s
		SET name = $3,
		    config = $4::jsonb,
		    auto_ai = $5,
		    updated_at = now()
		WHERE s.id = $1 AND s.user_id = $2
		RETURNING `+searcherSelect,
		id, userID, name, string(raw), autoAI,
	)
	item, err := scanSearcher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s *Store) SetSearcherAutoAI(ctx context.Context, userID, id string, enabled bool) (models.Searcher, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE searchers s
		SET auto_ai = $3, updated_at = now()
		WHERE s.id = $1 AND s.user_id = $2
		RETURNING `+searcherSelect,
		id, userID, enabled,
	)
	item, err := scanSearcher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s *Store) DeleteSearcher(ctx context.Context, userID, id string) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM searchers WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TouchSearcherRun(ctx context.Context, userID, id string) (models.Searcher, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE searchers s
		SET last_run_at = now(), updated_at = now()
		WHERE s.id = $1 AND s.user_id = $2
		RETURNING `+searcherSelect,
		id, userID,
	)
	item, err := scanSearcher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

// UpsertSearcherTender inserts a hit; returns true if the row was newly inserted.
func (s *Store) UpsertSearcherTender(ctx context.Context, searcherID string, hit models.SearchHit) (bool, error) {
	var inserted bool
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO searcher_tenders (
			searcher_id, reg_number, object_name, notice_url, nmck, application_end, law
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (searcher_id, reg_number) DO UPDATE SET
			object_name = CASE
				WHEN EXCLUDED.object_name <> '' THEN EXCLUDED.object_name
				ELSE searcher_tenders.object_name
			END,
			notice_url = CASE
				WHEN EXCLUDED.notice_url <> '' THEN EXCLUDED.notice_url
				ELSE searcher_tenders.notice_url
			END,
			nmck = COALESCE(EXCLUDED.nmck, searcher_tenders.nmck),
			application_end = CASE
				WHEN EXCLUDED.application_end <> '' THEN EXCLUDED.application_end
				ELSE searcher_tenders.application_end
			END,
			law = CASE
				WHEN EXCLUDED.law <> '' THEN EXCLUDED.law
				ELSE searcher_tenders.law
			END
		RETURNING (xmax = 0)`,
		searcherID, hit.RegNumber, hit.ObjectName, hit.NoticeURL, hit.NMCK, hit.ApplicationEnd, hit.Law,
	).Scan(&inserted)
	return inserted, err
}

func (s *Store) ListSearcherTenders(ctx context.Context, searcherID, q string) ([]models.TenderCard, error) {
	q = strings.TrimSpace(q)
	var (
		rows pgx.Rows
		err  error
	)
	if q == "" {
		rows, err = s.Pool.Query(ctx, `
			SELECT reg_number, object_name, COALESCE(application_end, ''), nmck,
			       notice_url, law
			FROM searcher_tenders
			WHERE searcher_id = $1
			ORDER BY found_at DESC`, searcherID)
	} else {
		like := "%" + q + "%"
		rows, err = s.Pool.Query(ctx, `
			SELECT reg_number, object_name, COALESCE(application_end, ''), nmck,
			       notice_url, law
			FROM searcher_tenders
			WHERE searcher_id = $1
			  AND (reg_number ILIKE $2 OR object_name ILIKE $2)
			ORDER BY found_at DESC`, searcherID, like)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.TenderCard, 0)
	for rows.Next() {
		var t models.TenderCard
		var notice, law string
		if err := rows.Scan(&t.RegNumber, &t.ObjectName, &t.ApplicationEnd, &t.NMCK, &notice, &law); err != nil {
			return nil, err
		}
		t.NoticeURL = notice
		t.Law = law
		t.AnalysisStatus = "none"
		t.CardTone = "neutral"
		t.CollectPct = 0
		t.AIPct = 0
		out = append(out, t)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanSearcher(row scannable) (models.Searcher, error) {
	var item models.Searcher
	var raw []byte
	var lastRun *time.Time
	err := row.Scan(
		&item.ID, &item.UserID, &item.Name, &raw, &item.AutoAI,
		&item.TendersCount, &item.CreatedAt, &item.UpdatedAt, &lastRun,
	)
	if err != nil {
		return item, err
	}
	item.LastRunAt = lastRun
	item.Config = models.DefaultSearcherConfig()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &item.Config); err != nil {
			return item, fmt.Errorf("decode config: %w", err)
		}
	}
	return item, nil
}
