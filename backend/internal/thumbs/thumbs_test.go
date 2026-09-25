package thumbs

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	red  = color.RGBA{220, 30, 30, 255}
	blue = color.RGBA{30, 30, 220, 255}
)

// halves рисует левую половину красной, правую синей.
func halves(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.SetRGBA(x, y, red)
			} else {
				img.SetRGBA(x, y, blue)
			}
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// exifAPP1 собирает сегмент APP1 с одним тегом Orientation.
func exifAPP1(orientation uint16, bo binary.AppendByteOrder) []byte {
	tiff := make([]byte, 0, 26)
	if bo == binary.AppendByteOrder(binary.LittleEndian) {
		tiff = append(tiff, 'I', 'I')
	} else {
		tiff = append(tiff, 'M', 'M')
	}
	tiff = bo.AppendUint16(tiff, 42)
	tiff = bo.AppendUint32(tiff, 8)
	tiff = bo.AppendUint16(tiff, 1)
	tiff = bo.AppendUint16(tiff, 0x0112)
	tiff = bo.AppendUint16(tiff, 3)
	tiff = bo.AppendUint32(tiff, 1)
	tiff = bo.AppendUint16(tiff, orientation)
	tiff = bo.AppendUint16(tiff, 0)
	tiff = bo.AppendUint32(tiff, 0)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	return append(seg, payload...)
}

// withOrientation вставляет APP1 сразу после SOI.
func withOrientation(jpg []byte, orientation uint16) []byte {
	out := append([]byte{}, jpg[:2]...)
	out = append(out, exifAPP1(orientation, binary.BigEndian)...)
	return append(out, jpg[2:]...)
}

func makeTo(t *testing.T, data []byte) (string, image.Image) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "t.jpg")
	if err := Make(bytes.NewReader(data), dst); err != nil {
		t.Fatalf("Make: %v", err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("format = %s, want jpeg", format)
	}
	return dst, img
}

func isRed(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return r>>8 > 150 && b>>8 < 100
}

func isBlue(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return b>>8 > 150 && r>>8 < 100
}

func TestMake_JPEGDownscale(t *testing.T) {
	_, img := makeTo(t, encodeJPEG(t, halves(3000, 2000)))
	b := img.Bounds()
	if b.Dx() != 480 || b.Dy() != 320 {
		t.Fatalf("size = %dx%d, want 480x320", b.Dx(), b.Dy())
	}
	if !isRed(img.At(48, 160)) || !isBlue(img.At(432, 160)) {
		t.Errorf("colors: left %v, right %v", img.At(48, 160), img.At(432, 160))
	}
}

func TestMake_PNGDownscale(t *testing.T) {
	_, img := makeTo(t, encodePNG(t, halves(600, 900)))
	b := img.Bounds()
	if b.Dx() != 320 || b.Dy() != 480 {
		t.Fatalf("size = %dx%d, want 320x480", b.Dx(), b.Dy())
	}
	if !isRed(img.At(32, 240)) || !isBlue(img.At(288, 240)) {
		t.Errorf("colors: left %v, right %v", img.At(32, 240), img.At(288, 240))
	}
}

func TestMake_SmallKeepsSize(t *testing.T) {
	_, img := makeTo(t, encodeJPEG(t, halves(200, 100)))
	b := img.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("size = %dx%d, want 200x100", b.Dx(), b.Dy())
	}
}

func TestMake_ExifOrientation(t *testing.T) {
	src := encodeJPEG(t, halves(3000, 2000))
	cases := []struct {
		orientation uint16
		w, h        int
		red, blue   image.Point
	}{
		{1, 480, 320, image.Pt(48, 160), image.Pt(432, 160)},
		{2, 480, 320, image.Pt(432, 160), image.Pt(48, 160)},
		{3, 480, 320, image.Pt(432, 160), image.Pt(48, 160)},
		{6, 320, 480, image.Pt(160, 48), image.Pt(160, 432)},
		{8, 320, 480, image.Pt(160, 432), image.Pt(160, 48)},
	}
	for _, c := range cases {
		_, img := makeTo(t, withOrientation(src, c.orientation))
		b := img.Bounds()
		if b.Dx() != c.w || b.Dy() != c.h {
			t.Errorf("orientation %d: size = %dx%d, want %dx%d", c.orientation, b.Dx(), b.Dy(), c.w, c.h)
			continue
		}
		if !isRed(img.At(c.red.X, c.red.Y)) || !isBlue(img.At(c.blue.X, c.blue.Y)) {
			t.Errorf("orientation %d: red at %v = %v, blue at %v = %v", c.orientation, c.red, img.At(c.red.X, c.red.Y), c.blue, img.At(c.blue.X, c.blue.Y))
		}
	}
}

func TestMake_WebP(t *testing.T) {
	webp := []byte("RIFF\x1a\x00\x00\x00WEBPVP8L\x0d\x00\x00\x00\x2f\x00\x00\x00\x10\x07\x10\x11\x11\x88\x88\xfe\x07\x00")
	_, img := makeTo(t, webp)
	if b := img.Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Errorf("size = %v", b)
	}
}

func TestMake_TooLarge(t *testing.T) {
	// SOI + APP0 JFIF + SOF0 с размером 8000x7000 (56 Мпикс), без данных скана.
	sof := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	sof = append(sof, 0xFF, 0xC0, 0x00, 0x11, 0x08)
	sof = binary.BigEndian.AppendUint16(sof, 7000)
	sof = binary.BigEndian.AppendUint16(sof, 8000)
	sof = append(sof, 3, 1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1)

	dst := filepath.Join(t.TempDir(), "big.jpg")
	err := Make(bytes.NewReader(sof), dst)
	if err == nil || !strings.Contains(err.Error(), ErrTooLarge.Error()) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Error("файл не должен быть создан")
	}
}

func TestMake_Garbage(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "bad.jpg")
	if err := Make(strings.NewReader("definitely not an image"), dst); err == nil {
		t.Fatal("ожидали ошибку")
	}
	if entries, _ := os.ReadDir(filepath.Dir(dst)); len(entries) != 0 {
		t.Errorf("остались файлы: %v", entries)
	}
}

func TestMake_Idempotent(t *testing.T) {
	data := encodeJPEG(t, halves(1000, 700))
	dst := filepath.Join(t.TempDir(), "sub", "1.jpg")
	for i := 0; i < 2; i++ {
		if err := Make(bytes.NewReader(data), dst); err != nil {
			t.Fatalf("Make #%d: %v", i+1, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(dst))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "1.jpg" {
		t.Errorf("в каталоге должен остаться только 1.jpg: %v", entries)
	}
	f, _ := os.Open(dst)
	defer f.Close()
	if _, err := jpeg.Decode(f); err != nil {
		t.Errorf("результат не читается: %v", err)
	}
}

func TestJPEGOrientation_Parser(t *testing.T) {
	plain := encodeJPEG(t, halves(8, 8))
	if got := jpegOrientation(plain); got != 1 {
		t.Errorf("без EXIF: %d", got)
	}
	for _, bo := range []binary.AppendByteOrder{binary.BigEndian, binary.LittleEndian} {
		b := append([]byte{0xFF, 0xD8}, exifAPP1(6, bo)...)
		b = append(b, plain[2:]...)
		if got := jpegOrientation(b); got != 6 {
			t.Errorf("%v: got %d, want 6", bo, got)
		}
	}
	trunc := append([]byte{0xFF, 0xD8}, exifAPP1(6, binary.BigEndian)[:20]...)
	if got := jpegOrientation(trunc); got != 1 {
		t.Errorf("обрезанный EXIF: %d", got)
	}
	if got := jpegOrientation(withOrientation(plain, 9)); got != 1 {
		t.Errorf("значение вне 1–8: %d", got)
	}
	if got := jpegOrientation([]byte("not jpeg")); got != 1 {
		t.Errorf("не JPEG: %d", got)
	}
}

func TestPath(t *testing.T) {
	got := Path(42)
	want := filepath.Join("web", "static", "uploads", "thumbs", "42.jpg")
	if got != want {
		t.Errorf("Path(42) = %q, want %q", got, want)
	}
}

func TestAcquireRelease_Limit(t *testing.T) {
	Acquire()
	Acquire()
	done := make(chan struct{})
	go func() {
		Acquire()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("третий Acquire не должен пройти")
	case <-time.After(50 * time.Millisecond):
	}
	Release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Acquire не разблокировался после Release")
	}
	Release()
	Release()
}

func BenchmarkMake_12Mpx(b *testing.B) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, halves(4032, 3024), &jpeg.Options{Quality: 90}); err != nil {
		b.Fatal(err)
	}
	dst := filepath.Join(b.TempDir(), "b.jpg")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Make(bytes.NewReader(buf.Bytes()), dst); err != nil {
			b.Fatal(err)
		}
	}
}
