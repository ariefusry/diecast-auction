package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ariefusry/diecast-auction/internal/testutil"
)

var testSecret = []byte(strings.Repeat("k", 32))

func newAuth(t *testing.T) *Auth {
	return NewAuth(testutil.Pool(t), testSecret)
}

func registerUser(t *testing.T, a *Auth) (User, string) {
	t.Helper()
	u := testutil.Unique()
	user, err := a.Register(context.Background(), "User_"+u+"@Example.com", "user_"+u, "correct-horse")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return user, "user_" + u + "@example.com"
}

func TestRegisterCreatesUserWithEmptyWallet(t *testing.T) {
	a := newAuth(t)
	user, email := registerUser(t, a)

	if user.ID == 0 || user.Email != email {
		t.Fatalf("user = %+v, want lowercased email %q", user, email)
	}
	var available, locked int64
	err := a.pool.QueryRow(context.Background(),
		"SELECT available_balance, locked_balance FROM wallets WHERE user_id = $1", user.ID).Scan(&available, &locked)
	if err != nil || available != 0 || locked != 0 {
		t.Fatalf("wallet = (%d, %d), err %v; want (0, 0)", available, locked, err)
	}
}

func TestRegisterRejectsDuplicateEmailAnyCase(t *testing.T) {
	a := newAuth(t)
	_, email := registerUser(t, a)

	_, err := a.Register(context.Background(), strings.ToUpper(email), "other_"+testutil.Unique(), "correct-horse")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

// Budi_64 and budi_64 would be two storefronts that look the same.
func TestRegisterRejectsDuplicateUsernameAnyCase(t *testing.T) {
	a := newAuth(t)
	u := testutil.Unique()
	if _, err := a.Register(context.Background(), "a"+u+"@x.io", "Case_"+u, "correct-horse"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err := a.Register(context.Background(), "b"+u+"@x.io", strings.ToUpper("case_"+u), "correct-horse")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	a := newAuth(t)
	u := testutil.Unique()
	cases := map[string][3]string{
		"bad email":      {"not-an-email", "name_" + u, "correct-horse"},
		"short username": {u + "@x.io", "ab", "correct-horse"},
		"short password": {u + "@x.io", "name_" + u, "short"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := a.Register(context.Background(), c[0], c[1], c[2])
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLoginReturnsTokenForUser(t *testing.T) {
	a := newAuth(t)
	user, email := registerUser(t, a)

	token, err := a.Login(context.Background(), email, "correct-horse")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	id, err := a.ParseToken(token)
	if err != nil || id != user.ID {
		t.Fatalf("ParseToken = %d, %v; want %d", id, err, user.ID)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	a := newAuth(t)
	_, email := registerUser(t, a)

	for name, c := range map[string][2]string{
		"wrong password": {email, "wrong-horse"},
		"unknown email":  {"nobody_" + testutil.Unique() + "@x.io", "correct-horse"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := a.Login(context.Background(), c[0], c[1]); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestParseTokenRejectsForgedOrExpired(t *testing.T) {
	a := NewAuth(nil, testSecret)
	sign := func(method jwt.SigningMethod, key any, exp time.Time) string {
		s, err := jwt.NewWithClaims(method, jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(exp),
		}).SignedString(key)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return s
	}
	future := time.Now().Add(time.Hour)
	cases := map[string]string{
		"expired":      sign(jwt.SigningMethodHS256, testSecret, time.Now().Add(-time.Minute)),
		"wrong secret": sign(jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), future),
		"alg none":     sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, future),
		"garbage":      "not.a.token",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := a.ParseToken(tok); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
