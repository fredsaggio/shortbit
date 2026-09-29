//go:build integration

package repositories_test

import (
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestURLRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	userRepository := repositories.NewUserRepository(pool)
	urlRepository := repositories.NewURLRepository(pool)

	user, err := userRepository.CreateWithPassword(t.Context(), "url-repository@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}

	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	t.Run("reserves distinct IDs", func(t *testing.T) {
		first, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("first ReserveID() error = %v", err)
		}

		second, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("second ReserveID() error = %v", err)
		}

		if first <= 0 || second <= first {
			t.Errorf("reserved IDs = (%d, %d), want positive increasing IDs", first, second)
		}
	})

	t.Run("creates a public URL with an explicitly reserved identity ID", func(t *testing.T) {
		id, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}

		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}

		want := models.URL{
			ID:          id,
			ShortCode:   code,
			UserID:      user.ID,
			OriginalURL: "https://example.com/public",
			Visibility:  models.VisibilityPublic,
		}

		created, err := urlRepository.Create(t.Context(), want)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if created.ID != want.ID || created.ShortCode != want.ShortCode || created.UserID != want.UserID ||
			created.OriginalURL != want.OriginalURL || created.Visibility != want.Visibility {
			t.Errorf("Create() = %+v, want URL fields %+v", created, want)
		}
		if created.PasswordHash != nil {
			t.Errorf("public URL password hash = %q, want nil", *created.PasswordHash)
		}
		if created.ClickCount != 0 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
			t.Errorf("Create() defaults = %+v, want zero clicks and nonzero timestamps", created)
		}

		var storedCode string
		if err := pool.QueryRow(t.Context(), "SELECT short_code FROM urls WHERE id = $1", id).Scan(&storedCode); err != nil {
			t.Fatalf("find created URL by reserved ID: %v", err)
		}
		if storedCode != code {
			t.Errorf("stored shortcode = %q, want %q", storedCode, code)
		}
	})

	t.Run("creates a private URL with a password hash", func(t *testing.T) {
		id, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}

		passwordHash := "$argon2id$private-link-test-hash"
		created, err := urlRepository.Create(t.Context(), models.URL{
			ID:           id,
			ShortCode:    code,
			UserID:       user.ID,
			OriginalURL:  "https://example.com/private",
			Visibility:   models.VisibilityPrivate,
			PasswordHash: &passwordHash,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if created.PasswordHash == nil || *created.PasswordHash != passwordHash {
			t.Errorf("private URL password hash = %v, want %q", created.PasswordHash, passwordHash)
		}
		if created.Visibility != models.VisibilityPrivate || created.ID != id || created.ShortCode != code {
			t.Errorf("Create() = %+v, want private URL with reserved ID and code", created)
		}
	})

	t.Run("rejects a duplicate shortcode without creating a second URL", func(t *testing.T) {
		firstID, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("first ReserveID() error = %v", err)
		}
		code, err := generator.Generate(firstID)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", firstID, err)
		}
		first := models.URL{
			ID: firstID, ShortCode: code, UserID: user.ID,
			OriginalURL: "https://example.com/first", Visibility: models.VisibilityPublic,
		}
		if _, err := urlRepository.Create(t.Context(), first); err != nil {
			t.Fatalf("first Create() error = %v", err)
		}

		secondID, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("second ReserveID() error = %v", err)
		}
		second := models.URL{
			ID: secondID, ShortCode: code, UserID: user.ID,
			OriginalURL: "https://example.com/second", Visibility: models.VisibilityPublic,
		}
		_, err = urlRepository.Create(t.Context(), second)

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "uq_urls_short_code" {
			t.Fatalf("duplicate Create() error = %v, want uq_urls_short_code violation", err)
		}

		var count int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM urls WHERE id = $1", secondID).Scan(&count); err != nil {
			t.Fatalf("count rejected URL: %v", err)
		}
		if count != 0 {
			t.Errorf("rejected URL count = %d, want 0", count)
		}
	})

	t.Run("reserves IDs concurrently without duplicates", func(t *testing.T) {
		const workers = 20
		type result struct {
			id  int64
			err error
		}

		results := make(chan result, workers)
		var group sync.WaitGroup
		for range workers {
			group.Add(1)
			go func() {
				defer group.Done()
				id, err := urlRepository.ReserveID(t.Context())
				results <- result{id: id, err: err}
			}()
		}
		group.Wait()
		close(results)

		seen := make(map[int64]bool, workers)
		for got := range results {
			if got.err != nil {
				t.Errorf("ReserveID() error = %v", got.err)
				continue
			}
			if got.id <= 0 || seen[got.id] {
				t.Errorf("ReserveID() returned invalid or duplicate ID %d", got.id)
			}
			seen[got.id] = true
		}
		if len(seen) != workers {
			t.Errorf("unique reserved IDs = %d, want %d", len(seen), workers)
		}
	})
}

func TestURLRepositoryListIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	owner, err := users.CreateWithPassword(t.Context(), "list-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := users.CreateWithPassword(t.Context(), "list-other@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	createURL := func(userID uuid.UUID, visibility models.Visibility) models.URL {
		t.Helper()
		id, err := urls.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		url := models.URL{ID: id, ShortCode: code, UserID: userID, OriginalURL: "https://example.com", Visibility: visibility}
		if visibility == models.VisibilityPrivate {
			hash := "$argon2id$private-link-test-hash"
			url.PasswordHash = &hash
		}
		created, err := urls.Create(t.Context(), url)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}

	first := createURL(owner.ID, models.VisibilityPublic)
	second := createURL(owner.ID, models.VisibilityPrivate)
	third := createURL(owner.ID, models.VisibilityPublic)
	createURL(other.ID, models.VisibilityPublic)

	// Same timestamps make the ID the decisive part of the keyset order.
	if _, err := pool.Exec(t.Context(), "UPDATE urls SET created_at = TIMESTAMPTZ '2026-01-01 00:00:00+00' WHERE user_id = $1", owner.ID); err != nil {
		t.Fatalf("set owner timestamps: %v", err)
	}

	page, err := urls.List(t.Context(), owner.ID, 2, nil)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	if len(page) != 2 || page[0].ID != third.ID || page[1].ID != second.ID {
		t.Fatalf("first page = %+v, want IDs %d, %d", page, third.ID, second.ID)
	}
	for _, url := range page {
		if url.UserID != owner.ID || url.PasswordHash != nil {
			t.Errorf("listed URL = %+v, want owner and no password hash", url)
		}
	}

	cursor := &repositories.URLCursor{CreatedAt: page[1].CreatedAt, ID: page[1].ID}
	page, err = urls.List(t.Context(), owner.ID, 2, cursor)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}
	if len(page) != 1 || page[0].ID != first.ID {
		t.Fatalf("second page = %+v, want ID %d", page, first.ID)
	}
	if page[0].UserID != owner.ID || page[0].PasswordHash != nil {
		t.Errorf("listed URL = %+v, want owner and no password hash", page[0])
	}

	page, err = urls.List(t.Context(), other.ID, 2, nil)
	if err != nil || len(page) != 1 || page[0].UserID != other.ID {
		t.Fatalf("other user page = %+v, error = %v, want one owned URL", page, err)
	}
}

func TestURLRepositoryGetByShortcodeIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	owner, err := users.CreateWithPassword(t.Context(), "get-url-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := users.CreateWithPassword(t.Context(), "get-url-other@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	id, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() error = %v", err)
	}
	code, err := generator.Generate(id)
	if err != nil {
		t.Fatalf("Generate(%d) error = %v", id, err)
	}
	passwordHash := "$argon2id$private-link-test-hash"
	created, err := urls.Create(t.Context(), models.URL{
		ID: id, ShortCode: code, UserID: owner.ID,
		OriginalURL: "https://example.com/private", Visibility: models.VisibilityPrivate,
		PasswordHash: &passwordHash,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE urls SET click_count = 7 WHERE id = $1", created.ID); err != nil {
		t.Fatalf("set click count: %v", err)
	}
	created.ClickCount = 7

	missingID, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() for missing link error = %v", err)
	}
	missingCode, err := generator.Generate(missingID)
	if err != nil {
		t.Fatalf("Generate(%d) for missing link error = %v", missingID, err)
	}

	t.Run("owner finds link metadata without password hash", func(t *testing.T) {
		got, err := urls.GetByShortcode(t.Context(), owner.ID, code)
		if err != nil {
			t.Fatalf("GetByShortcode() error = %v", err)
		}
		if got.ID != created.ID || got.ShortCode != created.ShortCode || got.UserID != owner.ID ||
			got.OriginalURL != created.OriginalURL || got.Visibility != created.Visibility ||
			got.ClickCount != created.ClickCount || !got.CreatedAt.Equal(created.CreatedAt) ||
			!got.UpdatedAt.Equal(created.UpdatedAt) {
			t.Errorf("GetByShortcode() = %+v, want metadata from %+v", got, created)
		}
		if got.PasswordHash != nil {
			t.Errorf("GetByShortcode() password hash = %q, want nil", *got.PasswordHash)
		}
	})

	t.Run("another user cannot find the link", func(t *testing.T) {
		_, err := urls.GetByShortcode(t.Context(), other.ID, code)
		if !errors.Is(err, repositories.ErrURLNotFound) {
			t.Errorf("GetByShortcode() error = %v, want ErrURLNotFound", err)
		}
	})

	t.Run("unknown shortcode is not found", func(t *testing.T) {
		_, err := urls.GetByShortcode(t.Context(), owner.ID, missingCode)
		if !errors.Is(err, repositories.ErrURLNotFound) {
			t.Errorf("GetByShortcode() error = %v, want ErrURLNotFound", err)
		}
	})
}

func TestURLRepositoryFindPrivateByShortCodeIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	owner, err := users.CreateWithPassword(t.Context(), "private-lookup-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	createURL := func(visibility models.Visibility, passwordHash *string) models.URL {
		t.Helper()
		id, err := urls.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		created, err := urls.Create(t.Context(), models.URL{
			ID: id, ShortCode: code, UserID: owner.ID, OriginalURL: "https://example.com/" + string(visibility),
			Visibility: visibility, PasswordHash: passwordHash,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}

	passwordHash := "$argon2id$private-link-test-hash"
	private := createURL(models.VisibilityPrivate, &passwordHash)
	public := createURL(models.VisibilityPublic, nil)

	t.Run("returns private link ID and password hash", func(t *testing.T) {
		gotID, gotHash, err := urls.FindPrivateByShortCode(t.Context(), private.ShortCode)
		if err != nil {
			t.Fatalf("FindPrivateByShortCode() error = %v", err)
		}
		if gotID != private.ID || gotHash != passwordHash {
			t.Errorf("FindPrivateByShortCode() = (%d, %q), want (%d, %q)", gotID, gotHash, private.ID, passwordHash)
		}
	})

	for _, tt := range []struct {
		name string
		code string
	}{
		{name: "public link", code: public.ShortCode},
		{name: "missing code", code: "NoSuchCode123"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotHash, err := urls.FindPrivateByShortCode(t.Context(), tt.code)
			if gotID != 0 || gotHash != "" || !errors.Is(err, repositories.ErrURLNotFound) {
				t.Errorf("FindPrivateByShortCode(%q) = (%d, %q, %v), want (0, empty, ErrURLNotFound)", tt.code, gotID, gotHash, err)
			}
		})
	}

	var clickCount int64
	if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE id = @id", pgx.StrictNamedArgs{"id": private.ID}).Scan(&clickCount); err != nil {
		t.Fatalf("query private link click count: %v", err)
	}
	if clickCount != 0 {
		t.Errorf("private link click count after password hash lookup = %d, want 0", clickCount)
	}
}

func TestURLRepositoryResolvePublicAndCountClickIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	owner, err := users.CreateWithPassword(t.Context(), "redirect-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	createURL := func(originalURL string, visibility models.Visibility) models.URL {
		t.Helper()
		id, err := urls.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		link := models.URL{ID: id, ShortCode: code, UserID: owner.ID, OriginalURL: originalURL, Visibility: visibility}
		if visibility == models.VisibilityPrivate {
			hash := "$argon2id$private-link-test-hash"
			link.PasswordHash = &hash
		}
		created, err := urls.Create(t.Context(), link)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}
	public := createURL("https://example.com/public", models.VisibilityPublic)
	private := createURL("https://example.com/private", models.VisibilityPrivate)
	clickCount := func(code string) int64 {
		t.Helper()
		var count int64
		if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE short_code = $1", code).Scan(&count); err != nil {
			t.Fatalf("query click count for %q: %v", code, err)
		}
		return count
	}

	for wantCount := int64(1); wantCount <= 2; wantCount++ {
		got, err := urls.ResolvePublicAndCountClick(t.Context(), public.ShortCode)
		if err != nil || got != public.OriginalURL || clickCount(public.ShortCode) != wantCount {
			t.Fatalf("public resolution %d = (%q, %v), count = %d; want URL %q and count %d",
				wantCount, got, err, clickCount(public.ShortCode), public.OriginalURL, wantCount)
		}
	}

	if got, err := urls.ResolvePublicAndCountClick(t.Context(), private.ShortCode); got != "" || !errors.Is(err, repositories.ErrURLNotFound) {
		t.Errorf("private resolution = (%q, %v), want empty URL and ErrURLNotFound", got, err)
	}
	if got := clickCount(private.ShortCode); got != 0 {
		t.Errorf("private click count = %d, want 0", got)
	}

	missingID, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() for missing link error = %v", err)
	}
	missingCode, err := generator.Generate(missingID)
	if err != nil {
		t.Fatalf("Generate(%d) for missing link error = %v", missingID, err)
	}
	if got, err := urls.ResolvePublicAndCountClick(t.Context(), missingCode); got != "" || !errors.Is(err, repositories.ErrURLNotFound) {
		t.Errorf("missing resolution = (%q, %v), want empty URL and ErrURLNotFound", got, err)
	}

	const workers = 20
	type result struct {
		originalURL string
		err         error
	}
	results := make(chan result, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			originalURL, err := urls.ResolvePublicAndCountClick(t.Context(), public.ShortCode)
			results <- result{originalURL: originalURL, err: err}
		}()
	}
	group.Wait()
	close(results)
	for got := range results {
		if got.err != nil || got.originalURL != public.OriginalURL {
			t.Errorf("concurrent resolution = (%q, %v), want %q", got.originalURL, got.err, public.OriginalURL)
		}
	}
	if got := clickCount(public.ShortCode); got != 2+workers {
		t.Errorf("click count after concurrent resolutions = %d, want %d", got, 2+workers)
	}
}

func TestURLRepositoryResolvePrivateAndCountClickIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	sessions := repositories.NewLinkAccessSessionRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	owner, err := users.CreateWithPassword(t.Context(), "private-redirect-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	createPrivateURL := func(originalURL string) models.URL {
		t.Helper()
		id, err := urls.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		passwordHash := "$argon2id$private-link-test-hash"
		created, err := urls.Create(t.Context(), models.URL{
			ID: id, ShortCode: code, UserID: owner.ID, OriginalURL: originalURL,
			Visibility: models.VisibilityPrivate, PasswordHash: &passwordHash,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}
	link := createPrivateURL("https://example.com/private-redirect")
	otherLink := createPrivateURL("https://example.com/other-private-redirect")
	validHash := sha256.Sum256([]byte("valid-private-redirect-token"))
	otherHash := sha256.Sum256([]byte("other-private-redirect-token"))
	expiredHash := sha256.Sum256([]byte("expired-private-redirect-token"))
	unknownHash := sha256.Sum256([]byte("unknown-private-redirect-token"))
	if err := sessions.CreateLinkAccessSession(t.Context(), link.ID, validHash[:], time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("create valid session: %v", err)
	}
	if err := sessions.CreateLinkAccessSession(t.Context(), otherLink.ID, otherHash[:], time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("create session for other link: %v", err)
	}
	_, err = pool.Exec(t.Context(), `
		INSERT INTO link_access_sessions (token_hash, link_id, created_at, expires_at)
		VALUES (@tokenHash, @linkID, @createdAt, @expiresAt)
	`, pgx.StrictNamedArgs{
		"tokenHash": expiredHash[:], "linkID": link.ID,
		"createdAt": time.Now().Add(-30 * time.Minute), "expiresAt": time.Now().Add(-15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	missingID, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() for missing link error = %v", err)
	}

	clickCount := func(id int64) int64 {
		t.Helper()
		var count int64
		if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE id = @id", pgx.StrictNamedArgs{"id": id}).Scan(&count); err != nil {
			t.Fatalf("query click count for link %d: %v", id, err)
		}
		return count
	}

	for _, tt := range []struct {
		name string
		id   int64
		hash []byte
	}{
		{name: "unknown token", id: link.ID, hash: unknownHash[:]},
		{name: "token for another link", id: link.ID, hash: otherHash[:]},
		{name: "expired token", id: link.ID, hash: expiredHash[:]},
		{name: "missing link", id: missingID, hash: validHash[:]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := urls.ResolvePrivateAndCountClick(t.Context(), tt.id, tt.hash)
			if got != "" || !errors.Is(err, repositories.ErrURLNotFound) {
				t.Errorf("ResolvePrivateAndCountClick() = (%q, %v), want empty URL and ErrURLNotFound", got, err)
			}
			if count := clickCount(link.ID); count != 0 {
				t.Errorf("private click count = %d, want 0", count)
			}
		})
	}

	got, err := urls.ResolvePrivateAndCountClick(t.Context(), link.ID, validHash[:])
	if err != nil || got != link.OriginalURL || clickCount(link.ID) != 1 {
		t.Fatalf("valid resolution = (%q, %v), count = %d; want (%q, nil), count 1", got, err, clickCount(link.ID), link.OriginalURL)
	}

	const workers = 20
	type result struct {
		originalURL string
		err         error
	}
	results := make(chan result, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			originalURL, err := urls.ResolvePrivateAndCountClick(t.Context(), link.ID, validHash[:])
			results <- result{originalURL: originalURL, err: err}
		}()
	}
	group.Wait()
	close(results)
	for result := range results {
		if result.err != nil || result.originalURL != link.OriginalURL {
			t.Errorf("concurrent resolution = (%q, %v), want %q", result.originalURL, result.err, link.OriginalURL)
		}
	}
	if count := clickCount(link.ID); count != 1+workers {
		t.Errorf("click count after concurrent resolutions = %d, want %d", count, 1+workers)
	}
	if count := clickCount(otherLink.ID); count != 0 {
		t.Errorf("other link click count = %d, want 0", count)
	}
}
