package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestVersionHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/version", Version("v1.2.3"))

	resp, err := app.Test(httptest.NewRequest("GET", "/version", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	if got.Version != "v1.2.3" {
		t.Errorf("version = %q, want %q", got.Version, "v1.2.3")
	}
}
