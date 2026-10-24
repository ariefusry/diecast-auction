package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func fakeParse(token string) (int64, error) {
	if token == "good" {
		return 42, nil
	}
	return 0, errors.New("bad token")
}

func TestRequireAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strconv.FormatInt(UserID(r.Context()), 10)))
	})
	h := RequireAuth(fakeParse)(next)

	cases := map[string]struct {
		header   string
		wantCode int
		wantBody string
	}{
		"valid bearer":   {"Bearer good", http.StatusOK, "42"},
		"missing header": {"", http.StatusUnauthorized, ""},
		"wrong scheme":   {"Basic good", http.StatusUnauthorized, ""},
		"invalid token":  {"Bearer bad", http.StatusUnauthorized, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, c.wantCode)
			}
			if c.wantBody != "" && rec.Body.String() != c.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), c.wantBody)
			}
		})
	}
}
