package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
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
	srv := httptest.NewServer(New(service.NewAuth(pool, secret), service.NewWallets(pool), service.NewAuctions(pool), parse, t.TempDir()))
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
	if code := (&client{t: t, base: base}).do("GET", "/auctions/"+id, nil, &a); code != http.StatusOK || a.CurrentHighestBid == nil || a.Leading {
		t.Fatalf("get = %d %+v", code, a)
	}
	var bids []service.BidEntry
	if code := (&client{t: t, base: base}).do("GET", "/auctions/"+id+"/bids", nil, &bids); code != http.StatusOK ||
		len(bids) != 1 || bids[0].Amount != 120_000 || !strings.Contains(bids[0].Bidder, "***") {
		t.Fatalf("bids = %d %+v", code, bids)
	}
	if code := bidder.do("GET", "/auctions/-1/bids", nil, nil); code != http.StatusNotFound {
		t.Fatalf("bids of unknown auction = %d", code)
	}
	var profile service.Profile
	if code := bidder.do("GET", "/me", nil, &profile); code != http.StatusOK || profile.Username == "" || profile.CreatedAt.IsZero() {
		t.Fatalf("me = %d %+v", code, profile)
	}
	var mine service.MyAuctions
	if code := bidder.do("GET", "/me/auctions", nil, &mine); code != http.StatusOK || len(mine.Bidding) != 1 || mine.Won == nil {
		t.Fatalf("me/auctions = %d %+v", code, mine)
	}
	var history []service.LedgerEntry
	if code := bidder.do("GET", "/wallet/history", nil, &history); code != http.StatusOK || len(history) != 2 || history[0].Type != "lock" {
		t.Fatalf("wallet/history = %d %+v", code, history)
	}
	for _, path := range []string{"/me", "/me/auctions", "/wallet/history"} {
		if code := (&client{t: t, base: base}).do("GET", path, nil, nil); code != http.StatusUnauthorized {
			t.Fatalf("anonymous GET %s = %d", path, code)
		}
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

func TestUniformErrorsDocsAndRateLimit(t *testing.T) {
	base := newServer(t)
	anon := &client{t: t, base: base}

	var errBody struct{ Error string }
	if code := anon.do("GET", "/no-such-route", nil, &errBody); code != http.StatusNotFound || errBody.Error == "" {
		t.Fatalf("unknown route = %d %+v", code, errBody)
	}

	var doc struct{ Paths map[string]any }
	if code := anon.do("GET", "/swagger/doc.json", nil, &doc); code != http.StatusOK || doc.Paths["/auctions/{id}/bids"] == nil {
		t.Fatalf("swagger doc = %d, paths %v", code, doc.Paths)
	}

	// bcrypt is slow under -race and the bucket refills over time, so allow a
	// few extra attempts; a burst well past the limit must still hit 429.
	login := map[string]string{"email": "nobody@x.io", "password": "wrong-horse"}
	for i := range authBurst + 5 {
		code := anon.do("POST", "/auth/login", login, nil)
		if code == http.StatusTooManyRequests {
			if i < authBurst {
				t.Fatalf("limited after only %d logins", i)
			}
			return
		}
		if code != http.StatusUnauthorized {
			t.Fatalf("login %d = %d", i, code)
		}
	}
	t.Fatal("never rate limited")
}

func TestUploadImage(t *testing.T) {
	base := newServer(t)
	user := signup(t, base)

	upload := func(name string, data []byte) (int, string) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("image", name)
		fw.Write(data)
		mw.Close()
		req, _ := http.NewRequest("POST", base+"/uploads", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+user.token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out struct{ URL string }
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out.URL
	}

	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...)
	code, url := upload("car.png", png)
	if code != http.StatusCreated || !strings.HasPrefix(url, "/uploads/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("upload = %d %q", code, url)
	}
	resp, err := http.Get(base + url)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %v %v", url, resp.StatusCode, err)
	}
	resp.Body.Close()

	// The sniffed content decides, not the file name.
	if code, _ := upload("evil.png", []byte("<html><script>alert(1)</script></html>")); code != http.StatusBadRequest {
		t.Fatalf("html upload = %d", code)
	}
	if code, _ := upload("big.png", append(png, make([]byte, maxUpload)...)); code != http.StatusBadRequest {
		t.Fatalf("oversized upload = %d", code)
	}
	if resp, _ := http.Get(base + "/uploads/"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("directory listing = %d", resp.StatusCode)
	}
}

// Every AI search may cost an LLM call, so it is rate limited per IP.
func TestSearchAIIsRateLimited(t *testing.T) {
	parse := func(context.Context, string) (service.Filter, error) { return service.Filter{Text: "zz"}, nil }
	anon := &client{t: t, base: newServerWithParser(t, parse)}
	for i := range aiBurst + 5 {
		code := anon.do("POST", "/auctions/search/ai", map[string]string{"query": "skyline"}, nil)
		if code == http.StatusTooManyRequests {
			if i < aiBurst {
				t.Fatalf("limited after only %d searches", i)
			}
			return
		}
		if code != http.StatusOK {
			t.Fatalf("search %d = %d", i, code)
		}
	}
	t.Fatal("never rate limited")
}

func TestChatAndSellerPage(t *testing.T) {
	base := newServer(t)
	seller, buyer, stranger := signup(t, base), signup(t, base), signup(t, base)
	buyer.do("POST", "/wallet/deposit", map[string]int64{"amount": 500_000}, nil)
	body := map[string]any{
		"item":        map[string]any{"brand": "Tomica", "model_name": "No. 1 Nissan GT-R", "condition": "sealed_box"},
		"start_price": 50_000, "min_increment": 5_000, "buy_now_price": 90_000, "duration_minutes": 60,
	}
	var a service.Auction
	seller.do("POST", "/auctions", body, &a)
	id := strconv.FormatInt(a.ID, 10)
	seller.do("POST", "/auctions/"+id+"/publish", nil, nil)

	if code := buyer.do("POST", "/auctions/"+id+"/messages", map[string]string{"body": "halo"}, nil); code != http.StatusForbidden {
		t.Fatalf("chat before winning = %d, want 403", code)
	}
	if code := buyer.do("POST", "/auctions/"+id+"/buy-now", nil, nil); code != http.StatusOK {
		t.Fatalf("buy-now = %d", code)
	}
	var m service.Message
	if code := buyer.do("POST", "/auctions/"+id+"/messages", map[string]string{"body": "Kirim JNE ya"}, &m); code != http.StatusCreated || m.ID == 0 {
		t.Fatalf("send = %d %+v", code, m)
	}
	var msgs []service.Message
	if code := seller.do("GET", "/auctions/"+id+"/messages", nil, &msgs); code != http.StatusOK || len(msgs) != 2 || !msgs[0].System || msgs[0].Mine ||
		msgs[1].Body != "Kirim JNE ya" || msgs[1].Mine {
		t.Fatalf("messages = %d %+v", code, msgs)
	}
	if !m.Mine || strings.Contains(seller.raw("GET", "/auctions/"+id+"/messages"), `"sender_id"`) {
		t.Fatalf("sent message %+v; the seller must see whose it is only as mine, never the buyer's ID", m)
	}
	if code := stranger.do("GET", "/auctions/"+id+"/messages", nil, nil); code != http.StatusForbidden {
		t.Fatalf("stranger reads = %d, want 403", code)
	}
	if code := (&client{t: t, base: base}).do("GET", "/auctions/"+id+"/messages", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous reads = %d, want 401", code)
	}

	var inbox []service.Conversation
	if code := seller.do("GET", "/chats", nil, &inbox); code != http.StatusOK || len(inbox) == 0 || inbox[0].AuctionID != a.ID || inbox[0].Unread != 2 { // sale notice + buyer
		t.Fatalf("seller inbox = %d %+v", code, inbox)
	}
	if raw := seller.raw("GET", "/chats"); strings.Contains(raw, "sender_id") || strings.Contains(raw, "winner_id") || strings.Contains(raw, "seller_id") {
		t.Fatalf("inbox leaks a user id: %s", raw)
	}
	if code := seller.do("POST", "/auctions/"+id+"/read", map[string]int64{"up_to": m.ID}, nil); code != http.StatusOK {
		t.Fatalf("mark read = %d", code)
	}
	if seller.do("GET", "/chats", nil, &inbox); inbox[0].Unread != 0 {
		t.Fatalf("after read: %+v", inbox[0])
	}
	if code := stranger.do("POST", "/auctions/"+id+"/read", map[string]int64{"up_to": m.ID}, nil); code != http.StatusForbidden {
		t.Fatalf("stranger mark read = %d, want 403", code)
	}

	var p service.SellerProfile
	anon := &client{t: t, base: base}
	if code := anon.do("GET", "/sellers/"+a.Seller, nil, &p); code != http.StatusOK || p.Sold != 1 || p.Username != a.Seller {
		t.Fatalf("seller page = %d %+v", code, p)
	}
	if code := anon.do("GET", "/sellers/nobody_here_"+a.Seller, nil, nil); code != http.StatusNotFound {
		t.Fatalf("unknown seller = %d, want 404", code)
	}
}

// raw returns the response body as text, so a test can check which keys it contains.
func (c *client) raw(method, path string) string {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var b strings.Builder
	if _, err := io.Copy(&b, resp.Body); err != nil {
		c.t.Fatal(err)
	}
	return b.String()
}

// Public responses never carry bidder or winner user IDs: paired with a seller's
// public username they would unmask the bid history. Viewers get booleans instead.
func TestBidderIdentityStaysPrivate(t *testing.T) {
	base := newServer(t)
	seller, bidder := signup(t, base), signup(t, base)
	anon := &client{t: t, base: base}
	bidder.do("POST", "/wallet/deposit", map[string]int64{"amount": 500_000}, nil)
	body := map[string]any{
		"item":        map[string]any{"brand": "Hot Wheels", "model_name": "Private " + testutil.Unique(), "condition": "mint_on_card"},
		"start_price": 100_000, "min_increment": 10_000, "duration_minutes": 60,
	}
	var a service.Auction
	seller.do("POST", "/auctions", body, &a)
	id := strconv.FormatInt(a.ID, 10)
	seller.do("POST", "/auctions/"+id+"/publish", nil, nil)
	var after map[string]any
	if code := bidder.do("POST", "/auctions/"+id+"/bids", map[string]int64{"amount": 100_000}, &after); code != http.StatusOK || after["leading"] != true {
		t.Fatalf("bid = %d %v, want leading", code, after)
	}

	leaks := []string{`"highest_bidder_id"`, `"winner_id"`, `"bidder_id"`, `"sender_id"`}
	for _, path := range []string{"/auctions/" + id, "/auctions/" + id + "/bids", "/auctions?q=" + url.QueryEscape(a.Item.ModelName), "/sellers/" + a.Seller} {
		got := anon.raw("GET", path)
		for _, key := range leaks {
			if strings.Contains(got, key) {
				t.Errorf("GET %s leaks %s: %s", path, key, got)
			}
		}
	}

	var view map[string]any
	anon.do("GET", "/auctions/"+id, nil, &view)
	if view["leading"] != false {
		t.Errorf("anonymous leading = %v, want false", view["leading"])
	}
	bidder.do("GET", "/auctions/"+id, nil, &view)
	if view["leading"] != true {
		t.Errorf("leader's leading = %v, want true", view["leading"])
	}
	var bids []map[string]any
	bidder.do("GET", "/auctions/"+id+"/bids", nil, &bids)
	if len(bids) != 1 || bids[0]["mine"] != true {
		t.Errorf("leader's bids = %v, want mine", bids)
	}
	// A bad token on a public route reads as anonymous, not 401.
	if code := (&client{t: t, base: base, token: "nope"}).do("GET", "/auctions/"+id, nil, nil); code != http.StatusOK {
		t.Errorf("GET with bad token = %d, want 200", code)
	}
}
