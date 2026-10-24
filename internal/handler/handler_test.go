package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/internal/testutil"
)

type client struct {
	t     *testing.T
	base  string
	token string
}

// do sends body as JSON (raw string bodies are sent as-is) and decodes the response into out.
func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		c.t.Fatalf("%s %s: Content-Type = %q", method, path, ct)
	}
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestAuthAndWalletFlow(t *testing.T) {
	pool := testutil.Pool(t)
	secret := []byte(strings.Repeat("k", 32))
	srv := httptest.NewServer(New(service.NewAuth(pool, secret), service.NewWallets(pool)))
	t.Cleanup(srv.Close)
	c := &client{t: t, base: srv.URL}

	u := testutil.Unique()
	creds := map[string]string{"email": u + "@x.io", "username": "u_" + u, "password": "correct-horse"}

	var user service.User
	if code := c.do("POST", "/auth/register", creds, &user); code != http.StatusCreated || user.ID == 0 {
		t.Fatalf("register = %d %+v", code, user)
	}
	var errBody struct{ Error string }
	if code := c.do("POST", "/auth/register", creds, &errBody); code != http.StatusConflict || errBody.Error == "" {
		t.Fatalf("duplicate register = %d %+v", code, errBody)
	}
	if code := c.do("POST", "/auth/register", `{"email":`, nil); code != http.StatusBadRequest {
		t.Fatalf("malformed register = %d", code)
	}
	if code := c.do("POST", "/auth/login", map[string]string{"email": creds["email"], "password": "nope-nope"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", code)
	}
	if code := c.do("GET", "/wallet", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("wallet without token = %d", code)
	}

	var login struct{ Token string }
	if code := c.do("POST", "/auth/login", creds, &login); code != http.StatusOK || login.Token == "" {
		t.Fatalf("login = %d", code)
	}
	c.token = login.Token

	var wallet service.Wallet
	if code := c.do("POST", "/wallet/deposit", map[string]int64{"amount": 250_000}, &wallet); code != http.StatusOK || wallet.Available != 250_000 {
		t.Fatalf("deposit = %d %+v", code, wallet)
	}
	if code := c.do("POST", "/wallet/deposit", map[string]int64{"amount": 0}, nil); code != http.StatusBadRequest {
		t.Fatalf("zero deposit = %d", code)
	}
	if code := c.do("GET", "/wallet", nil, &wallet); code != http.StatusOK || wallet != (service.Wallet{Available: 250_000}) {
		t.Fatalf("wallet = %d %+v", code, wallet)
	}
}
