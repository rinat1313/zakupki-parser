package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

var ErrNotFound = errors.New("not found")

type UserRow struct {
	ID           string
	Login        string
	PasswordHash string
	DisplayName  string
}

func (u UserRow) Public() models.User {
	name := u.DisplayName
	if name == "" {
		name = u.Login
	}
	return models.User{
		ID:          u.ID,
		Login:       u.Login,
		Name:        name,
		DisplayName: u.DisplayName,
	}
}

func (s *Store) GetUserByLogin(ctx context.Context, login string) (UserRow, error) {
	var u UserRow
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, login, password_hash, display_name
		FROM users WHERE login = $1`, login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (models.User, error) {
	var u models.User
	var display string
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, login, display_name, created_at, updated_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Login, &display, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	if err != nil {
		return u, err
	}
	u.DisplayName = display
	u.Name = display
	if u.Name == "" {
		u.Name = u.Login
	}
	return u, nil
}

func (s *Store) CreateUser(ctx context.Context, login, passwordHash, displayName string) (models.User, error) {
	var u models.User
	var display string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO users (login, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text, login, display_name, created_at, updated_at`,
		login, passwordHash, displayName,
	).Scan(&u.ID, &u.Login, &display, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return u, fmt.Errorf("create user: %w", err)
	}
	u.DisplayName = display
	u.Name = display
	if u.Name == "" {
		u.Name = u.Login
	}
	return u, nil
}
