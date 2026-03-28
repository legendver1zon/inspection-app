package cloudstorage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestYandex создаёт YandexDisk, направленный на тестовый HTTP-сервер.
func newTestYandex(serverURL string) *YandexDisk {
	orig := yadiskAPI
	yadiskAPI = serverURL
	y := NewYandexDisk("test-token", "disk:/test-root")
	yadiskAPI = orig // восстанавливаем, сервер URL уже захвачен в замыкании клиентом
	// Хак: подменяем API-переменную для самого теста.
	// YandexDisk использует yadiskAPI напрямую при каждом запросе, поэтому
	// переменную нужно держать подменённой на время работы теста.
	return y
}

// withFakeAPI устанавливает yadiskAPI = serverURL на время теста и восстанавливает после.
func withFakeAPI(t *testing.T, serverURL string) {
	t.Helper()
	orig := yadiskAPI
	yadiskAPI = serverURL
	t.Cleanup(func() { yadiskAPI = orig })
}

// --- FolderExists ---

func TestFolderExists_Returns200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/resources") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	exists, err := y.FolderExists("inspections/18-280326")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true for HTTP 200")
	}
}

func TestFolderExists_Returns404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	exists, err := y.FolderExists("inspections/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists=false for HTTP 404")
	}
}

func TestFolderExists_ReturnsErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	_, err := y.FolderExists("inspections/something")
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

// --- MoveFolder ---

func TestMoveFolder_Success201(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	err := y.MoveFolder("inspections/27", "inspections/18-280326")
	if err != nil {
		t.Fatalf("MoveFolder: %v", err)
	}

	// Проверяем что запрос содержит from и path
	if !strings.Contains(capturedURL, "from=") {
		t.Errorf("URL не содержит 'from=': %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "path=") {
		t.Errorf("URL не содержит 'path=': %s", capturedURL)
	}
}

func TestMoveFolder_Success200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	if err := y.MoveFolder("old", "new"); err != nil {
		t.Fatalf("MoveFolder: %v", err)
	}
}

func TestMoveFolder_ErrorOn409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	err := y.MoveFolder("old", "new")
	if err == nil {
		t.Error("ожидалась ошибка при HTTP 409, got nil")
	}
}

func TestMoveFolder_SendsPostMethod(t *testing.T) {
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	withFakeAPI(t, srv.URL)

	y := NewYandexDisk("tok", "disk:/root")
	y.MoveFolder("a", "b") //nolint:errcheck

	if capturedMethod != http.MethodPost {
		t.Errorf("метод = %q, ожидался POST", capturedMethod)
	}
}
