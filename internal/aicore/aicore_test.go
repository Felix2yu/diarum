package aicore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPolishSystemPrompt(t *testing.T) {
	tests := []struct {
		name         string
		mode         string
		customPrompt string
		want         string
		wantErr      bool
	}{
		{name: "empty defaults to medium", mode: "", want: mediumPrompt},
		{name: "medium", mode: "medium", want: mediumPrompt},
		{name: "medium padded", mode: "  MEDIUM  ", want: mediumPrompt},
		{name: "strong", mode: "strong", want: strongPrompt},
		{name: "voice", mode: "voice", want: voicePrompt},
		{name: "custom", mode: "custom", customPrompt: "  自定义提示  ", want: "自定义提示"},
		{name: "custom empty prompt", mode: "custom", customPrompt: "   ", wantErr: true},
		{name: "unsupported", mode: "extreme", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PolishSystemPrompt(tt.mode, tt.customPrompt)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PolishSystemPrompt(%q) expected error, got nil", tt.mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("PolishSystemPrompt(%q): %v", tt.mode, err)
			}
			if got != tt.want {
				t.Fatalf("PolishSystemPrompt(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestChatComplete(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		if _, err := ChatComplete(context.Background(), AIConfig{}, nil, false); err == nil {
			t.Fatal("expected error for empty config")
		}
	})

	t.Run("success", func(t *testing.T) {
		var gotAuth, gotURL string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotURL = r.URL.Path
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["model"] != "gpt-test" {
				t.Errorf("model = %v", req["model"])
			}
			if req["stream"] != false {
				t.Errorf("stream = %v", req["stream"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  整理后的文本  "}}]}`))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL + "/", APIKey: "sk-test", Model: "gpt-test"}
		got, err := ChatComplete(context.Background(), cfg, []ChatMessage{{Role: "user", Content: "原文"}}, false)
		if err != nil {
			t.Fatalf("ChatComplete: %v", err)
		}
		if got != "整理后的文本" {
			t.Fatalf("content = %q", got)
		}
		if gotURL != "/v1/chat/completions" {
			t.Fatalf("url path = %q", gotURL)
		}
		if gotAuth != "Bearer sk-test" {
			t.Fatalf("auth = %q", gotAuth)
		}
	})

	t.Run("upstream error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad key", http.StatusUnauthorized)
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test", Model: "gpt-test"}
		_, err := ChatComplete(context.Background(), cfg, nil, false)
		if err == nil || !strings.Contains(err.Error(), "401") {
			t.Fatalf("err = %v, want status 401 error", err)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test", Model: "gpt-test"}
		if _, err := ChatComplete(context.Background(), cfg, nil, false); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("no choices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[]}`))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test", Model: "gpt-test"}
		if _, err := ChatComplete(context.Background(), cfg, nil, false); err == nil || !strings.Contains(err.Error(), "no choices") {
			t.Fatalf("err = %v, want no choices error", err)
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		cfg := AIConfig{Enabled: true, BaseURL: "http://127.0.0.1:1", APIKey: "sk", Model: "m"}
		if _, err := ChatComplete(context.Background(), cfg, nil, false); err == nil {
			t.Fatal("expected request failure")
		}
	})
}

func TestTranscribe(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		if _, err := Transcribe(context.Background(), AIConfig{}, nil, "", "", "", "", ""); err == nil {
			t.Fatal("expected error for empty config")
		}
	})

	t.Run("success with defaults and optional fields", func(t *testing.T) {
		var gotAuth string
		var gotFields map[string]string
		var gotFile string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			}
			gotFields = map[string]string{
				"model":           r.FormValue("model"),
				"language":        r.FormValue("language"),
				"prompt":          r.FormValue("prompt"),
				"response_format": r.FormValue("response_format"),
				"temperature":     r.FormValue("temperature"),
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("FormFile: %v", err)
			} else {
				gotFile = header.Filename
				_ = file.Close()
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"转写结果"}`))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL + "/", APIKey: "sk-test"}
		got, err := Transcribe(context.Background(), cfg, []byte("audio-bytes"), "", "audio/webm", "zh", "", "提示词")
		if err != nil {
			t.Fatalf("Transcribe: %v", err)
		}
		if got != "转写结果" {
			t.Fatalf("text = %q", got)
		}
		if gotAuth != "Bearer sk-test" {
			t.Fatalf("auth = %q", gotAuth)
		}
		if gotFields["model"] != "whisper-1" {
			t.Fatalf("model = %q, want whisper-1 default", gotFields["model"])
		}
		if gotFields["language"] != "zh" || gotFields["prompt"] != "提示词" {
			t.Fatalf("fields = %v", gotFields)
		}
		if gotFields["response_format"] != "json" || gotFields["temperature"] != "0" {
			t.Fatalf("fields = %v", gotFields)
		}
		if gotFile != "audio.webm" {
			t.Fatalf("filename = %q, want audio.webm default", gotFile)
		}
	})

	t.Run("custom model and filename", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseMultipartForm(1 << 20)
			if r.FormValue("model") != "custom-whisper" {
				t.Errorf("model = %q", r.FormValue("model"))
			}
			_, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("FormFile: %v", err)
			} else if header.Filename != "memo.m4a" {
				t.Errorf("filename = %q", header.Filename)
			}
			_, _ = w.Write([]byte(`{"text":"ok"}`))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test"}
		got, err := Transcribe(context.Background(), cfg, []byte("x"), "memo.m4a", "", "", "custom-whisper", "")
		if err != nil || got != "ok" {
			t.Fatalf("Transcribe = %q, %v", got, err)
		}
	})

	t.Run("upstream error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "quota exceeded", http.StatusTooManyRequests)
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test"}
		_, err := Transcribe(context.Background(), cfg, []byte("x"), "", "", "", "", "")
		if err == nil || !strings.Contains(err.Error(), "429") {
			t.Fatalf("err = %v, want status 429 error", err)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("oops"))
		}))
		defer srv.Close()

		cfg := AIConfig{Enabled: true, BaseURL: srv.URL, APIKey: "sk-test"}
		if _, err := Transcribe(context.Background(), cfg, []byte("x"), "", "", "", "", ""); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		cfg := AIConfig{Enabled: true, BaseURL: "http://127.0.0.1:1", APIKey: "sk"}
		if _, err := Transcribe(context.Background(), cfg, []byte("x"), "", "", "", "", ""); err == nil {
			t.Fatal("expected request failure")
		}
	})
}
