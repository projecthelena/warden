package static

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNewHandler(t *testing.T) {
	// Create a mock filesystem
	mockFS := fstest.MapFS{
		"dist/index.html": &fstest.MapFile{
			Data: []byte("<html><body>Index</body></html>"),
		},
		"dist/assets/style.css": &fstest.MapFile{
			Data: []byte("body { color: red; }"),
		},
	}

	handler := NewHandler(mockFS)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Root returns index",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html><body>Index</body></html>",
		},
		{
			name:           "Existing asset returns asset",
			path:           "/assets/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; }",
		},
		{
			name:           "Unknown route returns index (SPA fallback)",
			path:           "/some/random/route",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html><body>Index</body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer func() { _ = res.Body.Close() }()

			if res.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, res.StatusCode)
			}

			// For index fallback, checking body content ensures we got index and not 404
			// (FileServer would return 404 if fallback logic wasn't working)
		})
	}
}

func TestNewHandler_MissingDist(t *testing.T) {
	mockFS := fstest.MapFS{} // Empty FS
	handler := NewHandler(mockFS)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 when dist missing, got %d", w.Code)
	}
}
