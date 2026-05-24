package external

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGenerateEmbedding_ShouldSplitLargeBatch(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			Input []string `json:"input"`
			Model string   `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		data := make([]struct {
			Object    string    `json:"object"`
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		}, len(req.Input))
		for i := range req.Input {
			data[i].Object = "embedding"
			data[i].Index = i
			data[i].Embedding = []float64{float64(i), float64(i + 1)}
		}

		resp := EmbeddingResponse{}
		resp.Object = "list"
		resp.Data = data
		resp.Model = req.Model
		resp.Usage.TotalTokens = len(req.Input)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	api := &API{
		client:                server.Client(),
		baseURL:               server.URL,
		apiKey:                "test-key",
		embeddingModelName:    "test-model",
		completionModelName:   "test-completion",
		completionTemperature: 0.3,
		completionMaxTokens:   10,
		retryConfig:           DefaultRetryConfig(),
	}

	texts := make([]string, 26)
	for i := range texts {
		texts[i] = "段落"
	}

	resp, err := api.GenerateEmbedding(texts)
	if err != nil {
		t.Fatalf("GenerateEmbedding() unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("GenerateEmbedding() returned nil response")
	}

	if len(resp.Data) != 26 {
		t.Fatalf("GenerateEmbedding() data length = %d, want 26", len(resp.Data))
	}

	if atomic.LoadInt32(&requestCount) != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
}
