package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"inspection-app/internal/thumbs"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setupThumbRouter — роутер с маршрутами фото и подставной авторизацией.
func setupThumbRouter(t *testing.T, userID uint, role string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
		c.Next()
	})
	r.GET("/photos/:id/thumb", GetPhotoThumb)
	r.GET("/photos/:id/download", GetPhotoDownload)
	r.POST("/photos/:id/delete", DeletePhoto)
	r.POST("/defects/:id/photos", PostUploadPhoto)
	t.Cleanup(func() { os.RemoveAll("web") })
	return r
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 255 / w), 80, uint8(y * 255 / h), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func getThumb(r http.Handler, photoID uint) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/photos/"+itoa(photoID)+"/thumb", nil)
	r.ServeHTTP(w, req)
	return w
}

func assertThumbResponse(t *testing.T, w *httptest.ResponseRecorder) image.Image {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; тело: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	img, err := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("тело не JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() > thumbs.MaxSide || b.Dy() > thumbs.MaxSide {
		t.Errorf("миниатюра больше %d px: %v", thumbs.MaxSide, b)
	}
	return img
}

func TestGetPhotoThumb_Local_OK(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-local")
	r := setupThumbRouter(t, insp.UserID, "inspector")
	photo := createLocalPhoto(t, defect.ID, "big.jpg", string(testJPEG(t, 1600, 1200)))

	first := getThumb(r, photo.ID)
	img := assertThumbResponse(t, first)
	if b := img.Bounds(); b.Dx() != 480 || b.Dy() != 360 {
		t.Errorf("size = %dx%d, want 480x360", b.Dx(), b.Dy())
	}
	if !fileExists(thumbs.Path(photo.ID)) {
		t.Fatal("миниатюра не сохранена на диске")
	}

	second := getThumb(r, photo.ID)
	assertThumbResponse(t, second)
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Error("повторный запрос отдал другое содержимое")
	}
}

func TestGetPhotoThumb_Stranger_Forbidden(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-stranger")
	stranger := newUser(t, "thumb_stranger@test.com", "pass1234", "Чужой Чужой", models.RoleInspector)
	if stranger.ID == insp.UserID {
		t.Fatal("тестовые пользователи совпали")
	}
	r := setupThumbRouter(t, stranger.ID, "inspector")
	photo := createLocalPhoto(t, defect.ID, "s.jpg", string(testJPEG(t, 300, 200)))

	w := getThumb(r, photo.ID)
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
	if fileExists(thumbs.Path(photo.ID)) {
		t.Error("миниатюра не должна строиться для чужого запроса")
	}
}

func TestGetPhotoThumb_Admin_OK(t *testing.T) {
	setupTestDB(t)
	_, defect := newInspectionWithDefect(t, "thumb-admin")
	r := setupThumbRouter(t, 9999, "admin")
	photo := createLocalPhoto(t, defect.ID, "a.jpg", string(testJPEG(t, 300, 200)))

	assertThumbResponse(t, getThumb(r, photo.ID))
}

func TestGetPhotoThumb_NoSource_Fallback(t *testing.T) {
	setupTestDB(t)
	cloudStore = nil
	insp, defect := newInspectionWithDefect(t, "thumb-nosrc")
	r := setupThumbRouter(t, insp.UserID, "inspector")

	photo := models.Photo{DefectID: defect.ID, FileName: "gone.jpg"}
	if err := storage.DB.Create(&photo).Error; err != nil {
		t.Fatal(err)
	}

	w := getThumb(r, photo.ID)
	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("got %d, want 307; тело: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/photos/"+itoa(photo.ID)+"/download" {
		t.Errorf("Location = %q", loc)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("редирект-фолбэк не должен кэшироваться: %q", cc)
	}
}

func TestGetPhotoThumb_CorruptSource_Fallback(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-corrupt")
	r := setupThumbRouter(t, insp.UserID, "inspector")
	photo := createLocalPhoto(t, defect.ID, "bad.jpg", "not-a-jpeg-at-all")

	w := getThumb(r, photo.ID)
	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("got %d, want 307", w.Code)
	}
	if fileExists(thumbs.Path(photo.ID)) {
		t.Error("битая миниатюра не должна сохраняться")
	}

	// Запасной маршрут действительно отдаёт оригинал
	dl := httptest.NewRecorder()
	r.ServeHTTP(dl, httptest.NewRequest("GET", w.Header().Get("Location"), nil))
	if dl.Code != http.StatusOK || dl.Body.String() != "not-a-jpeg-at-all" {
		t.Errorf("download fallback: %d %q", dl.Code, dl.Body.String())
	}
}

func TestGetPhotoThumb_Cloud_SingleDownload(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-cloud")
	r := setupThumbRouter(t, insp.UserID, "inspector")

	var hits atomic.Int32
	data := testJPEG(t, 1200, 900)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(data)
	}))
	defer srv.Close()

	cloudStore = &mockCloudStore{downloadURL: srv.URL + "/inspections/x/photo.jpg"}
	defer func() { cloudStore = nil }()

	photo := models.Photo{DefectID: defect.ID, FileName: "cloud.jpg", FileURL: "inspections/x/photo.jpg", UploadStatus: "done"}
	if err := storage.DB.Create(&photo).Error; err != nil {
		t.Fatal(err)
	}

	const n = 8
	results := make([]*httptest.ResponseRecorder, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = getThumb(r, photo.ID)
		}(i)
	}
	wg.Wait()

	for i, w := range results {
		if w.Code != http.StatusOK {
			t.Errorf("запрос %d: got %d; тело: %s", i, w.Code, w.Body.String())
		}
	}
	assertThumbResponse(t, results[0])
	if got := hits.Load(); got != 1 {
		t.Errorf("оригинал скачан %d раз, want 1", got)
	}
}

func TestGetPhotoThumb_CloudHTTPError_Fallback(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-cloud-err")
	r := setupThumbRouter(t, insp.UserID, "inspector")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	cloudStore = &mockCloudStore{downloadURL: srv.URL + "/missing.jpg"}
	defer func() { cloudStore = nil }()

	photo := models.Photo{DefectID: defect.ID, FileName: "c.jpg", FileURL: "inspections/x/missing.jpg", UploadStatus: "done"}
	if err := storage.DB.Create(&photo).Error; err != nil {
		t.Fatal(err)
	}

	if w := getThumb(r, photo.ID); w.Code != http.StatusTemporaryRedirect {
		t.Errorf("got %d, want 307", w.Code)
	}
}

func TestDeletePhoto_RemovesThumb(t *testing.T) {
	setupTestDB(t)
	insp, defect := newInspectionWithDefect(t, "thumb-delete")
	r := setupThumbRouter(t, insp.UserID, "inspector")
	photo := createLocalPhoto(t, defect.ID, "d.jpg", string(testJPEG(t, 300, 200)))

	assertThumbResponse(t, getThumb(r, photo.ID))
	if !fileExists(thumbs.Path(photo.ID)) {
		t.Fatal("миниатюра не создана")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/photos/"+itoa(photo.ID)+"/delete", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete: got %d; тело: %s", w.Code, w.Body.String())
	}
	if fileExists(thumbs.Path(photo.ID)) {
		t.Error("миниатюра не удалена вместе с фото")
	}
}

func TestPostUploadPhoto_GeneratesThumb(t *testing.T) {
	setupTestDB(t)
	cloudStore = nil
	insp, defect := newInspectionWithDefect(t, "thumb-upload")
	r := setupThumbRouter(t, insp.UserID, "inspector")

	body, ct := multipartPhoto(t, "photo.jpg", string(testJPEG(t, 1000, 750)))
	req := httptest.NewRequest("POST", "/defects/"+itoa(defect.ID)+"/photos", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: got %d; тело: %s", w.Code, w.Body.String())
	}

	var photo models.Photo
	if err := storage.DB.Where("defect_id = ?", defect.ID).First(&photo).Error; err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !fileExists(thumbs.Path(photo.ID)) {
		if time.Now().After(deadline) {
			t.Fatal("миниатюра не появилась после загрузки")
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertThumbResponse(t, getThumb(r, photo.ID))
}
