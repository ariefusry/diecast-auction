package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const tokenTTL = 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,30}$`)

// dummyHash keeps Login timing equal for unknown emails, so response time
// does not reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type User struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type Auth struct {
	pool   *pgxpool.Pool
	secret []byte
}

func NewAuth(pool *pgxpool.Pool, secret []byte) *Auth {
	return &Auth{pool: pool, secret: secret}
}

// Register creates the user and an empty wallet in one transaction.
func (a *Auth) Register(ctx context.Context, email, username, password string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return User{}, fmt.Errorf("%w: email is not valid", ErrInvalidInput)
	}
	if !usernamePattern.MatchString(username) {
		return User{}, fmt.Errorf("%w: username must be 3-30 letters, digits or underscores", ErrInvalidInput)
	}
	// bcrypt only uses the first 72 bytes, so longer passwords are refused rather than silently truncated.
	if len(password) < 8 || len(password) > 72 {
		return User{}, fmt.Errorf("%w: password must be 8-72 bytes", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	user := User{Email: email, Username: username}
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			"INSERT INTO users (email, username, password_hash) VALUES ($1, $2, $3) RETURNING id",
			email, username, hash).Scan(&user.ID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO wallets (user_id) VALUES ($1)", user.ID)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, fmt.Errorf("%w: email or username is taken", ErrConflict)
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// Login returns a signed JWT whose subject is the user ID.
func (a *Auth) Login(ctx context.Context, email, password string) (string, error) {
	var id int64
	hash := dummyHash
	err := a.pool.QueryRow(ctx,
		"SELECT id, password_hash FROM users WHERE lower(email) = lower($1)",
		strings.TrimSpace(email)).Scan(&id, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil || id == 0 {
		return "", ErrInvalidCredentials
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(id, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
	}).SignedString(a.secret)
}

// ParseToken validates signature (HS256 only), expiry, and returns the user ID.
func (a *Auth) ParseToken(token string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}
