// Package llm turns a free-text search into a structured service.Filter. The
// model only fills a JSON schema; the caller normalizes the result and the
// search service binds it as SQL parameters, so the model never writes SQL.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ariefusry/diecast-auction/internal/service"
)

type Gemini struct {
	key, model, baseURL string
	http                *http.Client
}

func NewGemini(key, model string) *Gemini {
	return &Gemini{
		key: key, model: model,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

const prompt = `You convert a diecast (1:64 scale) auction search, usually in Indonesian, into a JSON filter.
Rules:
- brand: only when the user names Hot Wheels, Mini GT or Inno64 (e.g. "hw" means Hot Wheels).
- color: one English color word in lowercase, translated (merah=red, putih=white, hitam=black, biru=blue, hijau=green, kuning=yellow, abu-abu=grey, perak=silver, emas=gold, oranye=orange, ungu=purple).
- tags: lowercase car makes (nissan, honda, toyota, ...) and "jdm" for Japanese cars or when the user says jdm.
- condition: mint/moc/mint on card=mint_on_card, segel/sealed/box=sealed_box, akrilik/acrylic=acrylic_case, loose=loose_mint, box rusak/penyok=damaged_packaging.
- min_price/max_price: whole rupiah ("100k" or "100rb" = 100000, "1,5jt" = 1500000). "di bawah X" sets max_price, "di atas X" sets min_price.
- sort: termurah=price_asc, termahal=price_desc, segera berakhir=ending_soon, terbaru=newest.
- text: remaining model keywords only (e.g. "skyline r34"), never words already used above.
Leave a field out when the query does not mention it.

Query: `

// schema constrains the model's output to the Filter shape.
var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"brand":     map[string]any{"type": "string", "enum": service.Brands},
		"series":    map[string]any{"type": "string"},
		"color":     map[string]any{"type": "string"},
		"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"condition": map[string]any{"type": "string", "enum": service.Conditions},
		"min_price": map[string]any{"type": "integer"},
		"max_price": map[string]any{"type": "integer"},
		"text":      map[string]any{"type": "string"},
		"sort":      map[string]any{"type": "string", "enum": []string{"price_asc", "price_desc", "ending_soon", "newest"}},
	},
}

// ParseQuery asks the model for a Filter. The result is not yet trusted:
// callers must Normalize it (service.Search does).
func (g *Gemini) ParseQuery(ctx context.Context, query string) (service.Filter, error) {
	body, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"parts": []any{map[string]any{"text": prompt + query}}}},
		"generationConfig": map[string]any{
			"responseFormat": map[string]any{"text": map[string]any{"mimeType": "APPLICATION_JSON", "schema": schema}},
		},
	})
	if err != nil {
		return service.Filter{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/models/"+g.model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return service.Filter{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.key)
	resp, err := g.http.Do(req)
	if err != nil {
		return service.Filter{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return service.Filter{}, fmt.Errorf("gemini: %s", resp.Status)
	}

	var out struct {
		Candidates []struct {
			Content struct{ Parts []struct{ Text string } }
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return service.Filter{}, err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return service.Filter{}, errors.New("gemini: empty response")
	}
	var f service.Filter
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text)), &f); err != nil {
		return service.Filter{}, fmt.Errorf("gemini: reply is not a filter: %w", err)
	}
	return f, nil
}
