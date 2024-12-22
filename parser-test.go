package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDynamicHost(t *testing.T) {
	html := `
<html>
  <body>
    <script>var someUrl = "https://line22w.bk6bba-resources.com/...";</script>
  </body>
</html>
`
	host, err := GetDynamicHost(html)
	if err != nil {
		t.Fatalf("Неожиданная ошибка: %v", err)
	}
	if host != "line22w.bk6bba-resources.com" {
		t.Errorf("Ожидали line22w.bk6bba-resources.com, получили %s", host)
	}
}

func TestGetDynamicHost_NotFound(t *testing.T) {
	html := "<html><body>No host here</body></html>"
	_, err := GetDynamicHost(html)
	if err == nil {
		t.Fatal("Ожидали ошибку, но её нет")
	}
}

func TestFetchData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"events":[{"id":123},{"id":456}]}`))
	}))
	defer ts.Close()

	result, err := FetchData(ts.URL)
	if err != nil {
		t.Fatalf("Ошибка FetchData: %v", err)
	}
	if len(result.Events) != 2 {
		t.Errorf("Ожидали 2 события, получили %d", len(result.Events))
	}
}
