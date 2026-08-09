package auth

import "testing"

func TestDemoPasswordHashFromMigration(t *testing.T) {
	// Must stay in sync with migrations/search/001_init.sql demo user.
	const hash = "$2a$10$7DVFvBs46wuWHeLQG9WIie5l5bUGD/H2TiUrU5eoK2cMW2cqFzuqW"
	if !CheckPassword(hash, "demo") {
		t.Fatal("demo password hash mismatch")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}
