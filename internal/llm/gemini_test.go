package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/service"
)

func fakeGemini(t *testing.T, status int, reply string) *Gemini {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/test-model:generateContent" || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("request %s key %q", r.URL.Path, r.Header.Get("x-goog-api-key"))
		}
		var body struct {
			Contents         []struct{ Parts []struct{ Text string } }
			GenerationConfig struct {
				ResponseFormat struct {
					Text struct {
						MimeType string
						Schema   map[string]any
					}
				}
			}
		}
		json.NewDecoder(r.Body).Decode(&body)
		if f := body.GenerationConfig.ResponseFormat.Text; f.MimeType != "APPLICATION_JSON" || f.Schema["type"] != "object" {
			t.Errorf("response format = %+v", f)
		}
		if !strings.Contains(body.Contents[0].Parts[0].Text, "jdm merah") {
			t.Errorf("query missing from prompt")
		}
		w.WriteHeader(status)
		w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	g := NewGemini("test-key", "test-model")
	g.baseURL = srv.URL
	return g
}

func TestParseQueryReturnsStructuredFilter(t *testing.T) {
	filter := `{"brand":"Mini GT","color":"red","tags":["jdm"],"condition":"mint_on_card","max_price":100000}`
	reply, _ := json.Marshal(map[string]any{"candidates": []any{
		map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": filter}}}},
	}})
	got, err := fakeGemini(t, http.StatusOK, string(reply)).ParseQuery(context.Background(), "jdm merah mint di bawah 100k")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	want := service.Filter{Brand: "Mini GT", Color: "red", Tags: []string{"jdm"}, Condition: "mint_on_card", MaxPrice: 100_000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseQueryFailsOnBadReplies(t *testing.T) {
	for name, c := range map[string]struct {
		status int
		reply  string
	}{
		"http error":    {http.StatusTooManyRequests, `{"error":{}}`},
		"no candidates": {http.StatusOK, `{"candidates":[]}`},
		"not json":      {http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"sure! here"}]}}]}`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := fakeGemini(t, c.status, c.reply).ParseQuery(context.Background(), "jdm merah"); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

// The model may only pick text keywords out of the query, never invent them.
func TestParseQueryKeepsOnlyTextWordsFromTheQuery(t *testing.T) {
	filter := `{"text":"skyline R34 or R32/R33 model context assumed"}`
	reply, _ := json.Marshal(map[string]any{"candidates": []any{
		map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": filter}}}},
	}})
	got, err := fakeGemini(t, http.StatusOK, string(reply)).ParseQuery(context.Background(), "jdm merah Skyline r34")
	if err != nil || got.Text != "skyline R34" {
		t.Fatalf("Text = %q, %v", got.Text, err)
	}
}
