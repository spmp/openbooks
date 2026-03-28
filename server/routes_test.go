package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type libraryDownload struct {
	Name         string `json:"name"`
	DownloadLink string `json:"downloadLink"`
}

func TestGetAllBooksHandlerListsBooksWhenBrowserDownloadsDisabled(t *testing.T) {
	tempDir := t.TempDir()
	bookPath := filepath.Join(tempDir, "example.epub")
	if err := os.WriteFile(bookPath, []byte("ok"), 0644); err != nil {
		t.Fatalf("write book: %v", err)
	}

	server := &server{
		config: &Config{
			Persist:                 false,
			DisableBrowserDownloads: true,
			DownloadDir:             tempDir,
		},
		log: log.New(io.Discard, "", 0),
	}

	request := httptest.NewRequest(http.MethodGet, "/library", nil)
	recorder := httptest.NewRecorder()

	server.getAllBooksHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	results := []libraryDownload{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(results) != 1 || results[0].Name != "example.epub" {
		t.Fatalf("expected one listed book, got %#v", results)
	}
}
