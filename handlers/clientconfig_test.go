package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestClientConfigHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/config", ClientConfig(50<<20, "https://models.example.com/bg"))

	resp, err := app.Test(httptest.NewRequest("GET", "/config", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		MaxFileBytes int64 `json:"maxFileBytes"`
		BgRemoval    struct {
			BaseURL string `json:"baseUrl"`
		} `json:"bgRemoval"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	if got.MaxFileBytes != 50<<20 {
		t.Errorf("maxFileBytes = %d, want %d", got.MaxFileBytes, 50<<20)
	}
	if got.BgRemoval.BaseURL != "https://models.example.com/bg" {
		t.Errorf("bgRemoval.baseUrl = %q", got.BgRemoval.BaseURL)
	}
}
