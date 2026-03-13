package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := ping(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

type Voucher struct {
	Code      string
	Minutes   int
	CreatedAt time.Time
	UsedAt    sql.NullTime
	UsedByMAC sql.NullString
	UsedByIP  sql.NullString
}

type Session struct {
	ID      int64
	MAC     string
	IP      string
	StartAt time.Time
	EndAt   time.Time
}

var ErrVoucherNotFound = errors.New("voucher not found")
var ErrVoucherUsed = errors.New("voucher already used")

func (s *Store) CreateVoucher(ctx context.Context, code string, minutes int) (Voucher, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		insert into vouchers (code, minutes, created_at_unix)
		values (?, ?, ?)
	`, code, minutes, now.Unix())
	if err != nil {
		return Voucher{}, err
	}
	return Voucher{
		Code:      code,
		Minutes:   minutes,
		CreatedAt: now,
	}, nil
}

type ConsumeVoucherParams struct {
	Code string
	MAC  string
	IP   string
	Now  time.Time
}

type ConsumeVoucherResult struct {
	Voucher Voucher
	Session Session
}

func (s *Store) ConsumeVoucher(ctx context.Context, p ConsumeVoucherParams) (ConsumeVoucherResult, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ConsumeVoucherResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		minutes       int
		createdAtUnix int64
		usedAtUnix    sql.NullInt64
		usedByMAC     sql.NullString
		usedByIP      sql.NullString
	)
	row := tx.QueryRowContext(ctx, `
		select minutes, created_at_unix, used_at_unix, used_by_mac, used_by_ip
		from vouchers
		where code = ?
	`, p.Code)
	if err := row.Scan(&minutes, &createdAtUnix, &usedAtUnix, &usedByMAC, &usedByIP); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ConsumeVoucherResult{}, ErrVoucherNotFound
		}
		return ConsumeVoucherResult{}, err
	}
	if usedAtUnix.Valid {
		return ConsumeVoucherResult{}, ErrVoucherUsed
	}

	now := p.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	endAt := now.Add(time.Duration(minutes) * time.Minute)

	res, err := tx.ExecContext(ctx, `
		update vouchers
		set used_at_unix = ?, used_by_mac = ?, used_by_ip = ?
		where code = ? and used_at_unix is null
	`, now.Unix(), nullIfEmpty(p.MAC), nullIfEmpty(p.IP), p.Code)
	if err != nil {
		return ConsumeVoucherResult{}, err
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return ConsumeVoucherResult{}, ErrVoucherUsed
	}

	insertRes, err := tx.ExecContext(ctx, `
		insert into sessions (mac, ip, start_at_unix, end_at_unix)
		values (?, ?, ?, ?)
	`, nullIfEmpty(p.MAC), nullIfEmpty(p.IP), now.Unix(), endAt.Unix())
	if err != nil {
		return ConsumeVoucherResult{}, err
	}
	sessionID, err := insertRes.LastInsertId()
	if err != nil {
		return ConsumeVoucherResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return ConsumeVoucherResult{}, err
	}

	return ConsumeVoucherResult{
		Voucher: Voucher{
			Code:      p.Code,
			Minutes:   minutes,
			CreatedAt: time.Unix(createdAtUnix, 0).UTC(),
			UsedAt:    sql.NullTime{Time: now, Valid: true},
			UsedByMAC: nullIfEmpty(p.MAC),
			UsedByIP:  nullIfEmpty(p.IP),
		},
		Session: Session{
			ID:      sessionID,
			MAC:     p.MAC,
			IP:      p.IP,
			StartAt: now,
			EndAt:   endAt,
		},
	}, nil
}

func (s *Store) GetActiveSessionByIP(ctx context.Context, ip string, now time.Time) (Session, bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	row := s.db.QueryRowContext(ctx, `
		select id, mac, ip, start_at_unix, end_at_unix
		from sessions
		where ip = ? and end_at_unix > ?
		order by end_at_unix desc
		limit 1
	`, ip, now.Unix())

	var (
		id          int64
		mac         sql.NullString
		ipOut       sql.NullString
		startAtUnix int64
		endAtUnix   int64
	)
	if err := row.Scan(&id, &mac, &ipOut, &startAtUnix, &endAtUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, false, nil
		}
		return Session{}, false, err
	}
	return Session{
		ID:      id,
		MAC:     mac.String,
		IP:      ipOut.String,
		StartAt: time.Unix(startAtUnix, 0).UTC(),
		EndAt:   time.Unix(endAtUnix, 0).UTC(),
	}, true, nil
}

func (s *Store) ListVouchers(ctx context.Context, limit int) ([]Voucher, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		select code, minutes, created_at_unix, used_at_unix, used_by_mac, used_by_ip
		from vouchers
		order by created_at_unix desc
		limit ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Voucher, 0, min(limit, 200))
	for rows.Next() {
		var (
			code         string
			minutes      int
			createdAtUnix int64
			usedAtUnix   sql.NullInt64
			usedByMAC    sql.NullString
			usedByIP     sql.NullString
		)
		if err := rows.Scan(&code, &minutes, &createdAtUnix, &usedAtUnix, &usedByMAC, &usedByIP); err != nil {
			return nil, err
		}
		v := Voucher{
			Code:      code,
			Minutes:   minutes,
			CreatedAt: time.Unix(createdAtUnix, 0).UTC(),
			UsedByMAC: usedByMAC,
			UsedByIP:  usedByIP,
		}
		if usedAtUnix.Valid {
			v.UsedAt = sql.NullTime{Time: time.Unix(usedAtUnix.Int64, 0).UTC(), Valid: true}
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		select id, mac, ip, start_at_unix, end_at_unix
		from sessions
		order by start_at_unix desc
		limit ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Session, 0, min(limit, 200))
	for rows.Next() {
		var (
			id          int64
			mac         sql.NullString
			ipOut       sql.NullString
			startAtUnix int64
			endAtUnix   int64
		)
		if err := rows.Scan(&id, &mac, &ipOut, &startAtUnix, &endAtUnix); err != nil {
			return nil, err
		}
		out = append(out, Session{
			ID:      id,
			MAC:     mac.String,
			IP:      ipOut.String,
			StartAt: time.Unix(startAtUnix, 0).UTC(),
			EndAt:   time.Unix(endAtUnix, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ping(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`create table if not exists vouchers (
			code text primary key not null,
			minutes integer not null,
			created_at_unix integer not null,
			used_at_unix integer null,
			used_by_mac text null,
			used_by_ip text null
		);`,
		`create index if not exists idx_vouchers_used_at_unix on vouchers (used_at_unix);`,
		`create table if not exists sessions (
			id integer primary key autoincrement,
			mac text null,
			ip text null,
			start_at_unix integer not null,
			end_at_unix integer not null
		);`,
		`create index if not exists idx_sessions_ip_end on sessions (ip, end_at_unix);`,
	}
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func (s *Store) CountVouchers(ctx context.Context) (int64, error) {
	row := s.db.QueryRowContext(ctx, `select count(*) from vouchers`)
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) CountActiveSessions(ctx context.Context, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	row := s.db.QueryRowContext(ctx, `select count(*) from sessions where end_at_unix > ?`, now.Unix())
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
