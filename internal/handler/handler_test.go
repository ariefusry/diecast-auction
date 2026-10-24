package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func newServer(t *testing.T) string {
	return newServerWithParser(t, nil)
}

func newServerWithParser(t *testing.T, parse ParseFunc) string {
	pool := testutil.Pool(t)
	secret := []byte(strings.Repeat("k", 32))
	srv := httptest.NewServer(New(service.NewAuth(pool, secret), service.NewWallets(pool), service.NewAuctions(pool), parse))
	t.Cleanup(srv.Close)
	return srv.URL
}

// signup registers and logs in a fresh user, returning an authenticated client.
func signup(t *testing.T, base string) *client {
	t.Helper()
	c := &client{t: t, base: base}
	u := testutil.Unique()
	creds := map[string]string{"email": u + "@x.io", "username": "u_" + u, "password": "correct-horse"}
	var login struct{ Token string }
	if c.do("POST", "/auth/register", creds, nil) != http.StatusCreated || c.do("POST", "/auth/login", creds, &login) != http.StatusOK {
		t.Fatal("signup failed")
	}
	c.token = login.Token
	return c
}

func TestAuthAndWalletFlow(t *testing.T) {
	c := &client{t: t, base: newServer(t)}

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

func TestAuctionFlow(t *testing.T) {
	base := newServer(t)
	seller, bidder := signup(t, base), signup(t, base)
	bidder.do("POST", "/wallet/deposit", map[string]int64{"amount": 500_000}, nil)

	body := map[string]any{
		"item":             map[string]any{"brand": "Hot Wheels", "model_name": "Datsun 510", "condition": "mint_on_card"},
		"start_price":      100_000,
		"min_increment":    10_000,
		"duration_minutes": 60,
	}
	var a service.Auction
	if code := seller.do("POST", "/auctions", body, &a); code != http.StatusCreated || a.Status != "draft" {
		t.Fatalf("create = %d %+v", code, a)
	}
	id := strconv.FormatInt(a.ID, 10)
	if code := (&client{t: t, base: base}).do("POST", "/auctions", body, nil); code != http.StatusUnauthorized {
		t.Fatalf("create without token = %d", code)
	}
	if code := bidder.do("POST", "/auctions/"+id+"/publish", nil, nil); code != http.StatusForbidden {
		t.Fatalf("publish by bidder = %d", code)
	}
	if code := seller.do("POST", "/auctions/"+id+"/publish", nil, &a); code != http.StatusOK || a.Status != "active" {
		t.Fatalf("publish = %d %+v", code, a)
	}
	if code := bidder.do("POST", "/auctions/"+id+"/bids", map[string]int64{"amount": 50_000}, nil); code != http.StatusConflict {
		t.Fatalf("low bid = %d", code)
	}
	if code := bidder.do("POST", "/auctions/"+id+"/bids", map[string]int64{"amount": 120_000}, &a); code != http.StatusOK || *a.CurrentHighestBid != 120_000 {
		t.Fatalf("bid = %d %+v", code, a)
	}
	if code := (&client{t: t, base: base}).do("GET", "/auctions/"+id, nil, &a); code != http.StatusOK || *a.HighestBidderID == 0 {
		t.Fatalf("get = %d %+v", code, a)
	}
	if code := seller.do("POST", "/auctions/"+id+"/cancel", nil, nil); code != http.StatusConflict {
		t.Fatalf("cancel with bid = %d", code)
	}
	if code := bidder.do("POST", "/auctions/"+id+"/buy-now", nil, nil); code != http.StatusConflict {
		t.Fatalf("buy-now with bid = %d", code)
	}
	for _, path := range []string{"/auctions/-1", "/auctions/abc"} {
		if code := bidder.do("GET", path, nil, nil); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d", path, code)
		}
	}
}

func TestSearchEndpoints(t *testing.T) {
	token := "Zq" + testutil.Unique()
	var parsed string
	parse := func(_ context.Context, q string) (service.Filter, error) {
		parsed = q
		if q == "llm down" {
			return service.Filter{}, errors.New("timeout")
		}
		return service.Filter{Text: token, Brand: "hot wheels", Condition: "made-up"}, nil
	}
	base := newServerWithParser(t, parse)
	seller := signup(t, base)
	for _, brand := range []string{"Hot Wheels", "Mini GT"} {
		body := map[string]any{
			"item":        map[string]any{"brand": brand, "model_name": token + " Civic", "condition": "loose_mint"},
			"start_price": 50_000, "min_increment": 5_000, "duration_minutes": 60,
		}
		var a service.Auction
		seller.do("POST", "/auctions", body, &a)
		seller.do("POST", "/auctions/"+strconv.FormatInt(a.ID, 10)+"/publish", nil, nil)
	}
	anon := &client{t: t, base: base}

	var res struct {
		AI bool
		service.SearchResult
	}
	if code := anon.do("GET", "/auctions?q="+token+"&brand=Mini+GT", nil, &res); code != http.StatusOK ||
		len(res.Auctions) != 1 || res.Auctions[0].Item.Brand != "Mini GT" {
		t.Fatalf("GET /auctions = %d %+v", code, res)
	}

	// The LLM filter is normalized: brand casing fixed, unknown condition dropped.
	if code := anon.do("POST", "/auctions/search/ai", map[string]string{"query": "hw civic loose"}, &res); code != http.StatusOK ||
		!res.AI || parsed != "hw civic loose" || len(res.Auctions) != 1 || res.Filter.Brand != "Hot Wheels" || res.Filter.Condition != "" {
		t.Fatalf("AI search = %d %+v", code, res)
	}
	// An LLM failure degrades to keyword search instead of an error.
	if code := anon.do("POST", "/auctions/search/ai", map[string]string{"query": "llm down"}, &res); code != http.StatusOK || res.AI {
		t.Fatalf("AI search with LLM down = %d %+v", code, res)
	}
	for _, q := range []string{"", strings.Repeat("x", 201)} {
		if code := anon.do("POST", "/auctions/search/ai", map[string]string{"query": q}, nil); code != http.StatusBadRequest {
			t.Fatalf("query len %d = %d", len(q), code)
		}
	}
}
