// Package thumbs строит серверные миниатюры фото (JPEG, длинная сторона ≤ MaxSide).
package thumbs

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxSide     = 480
	Quality     = 80
	maxPixels   = 30_000_000
	concurrency = 2
)

var ErrTooLarge = errors.New("изображение слишком большое")

var sem = make(chan struct{}, concurrency)

// Acquire занимает слот генерации (не более concurrency одновременно).
func Acquire() { sem <- struct{}{} }

// Release освобождает слот, занятый Acquire.
func Release() { <-sem }

// Path — путь миниатюры фото относительно рабочего каталога приложения.
func Path(photoID uint) string {
	return filepath.Join("web", "static", "uploads", "thumbs", strconv.FormatUint(uint64(photoID), 10)+".jpg")
}

// Make читает JPEG/PNG/WebP из src и атомарно записывает миниатюру в dst.
func Make(src io.Reader, dst string) error {
	var head bytes.Buffer
	cfg, format, err := image.DecodeConfig(io.TeeReader(src, &head))
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return errors.New("пустое изображение")
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return fmt.Errorf("%w: %dx%d", ErrTooLarge, cfg.Width, cfg.Height)
	}

	orientation := 1
	if format == "jpeg" {
		orientation = jpegOrientation(head.Bytes())
	}

	img, _, err := image.Decode(io.MultiReader(bytes.NewReader(head.Bytes()), src))
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	out := orient(shrink(img), orientation)
	return writeAtomic(dst, out)
}

// shrink уменьшает изображение до MaxSide по длинной стороне; маленькое лишь копируется.
func shrink(img image.Image) *image.RGBA {
	b := img.Bounds()
	tw, th := fit(b.Dx(), b.Dy())

	// Грубое целочисленное уменьшение box-фильтром: CatmullRom держит в памяти
	// буфер dstW×srcH×32 байт, для 12-Мпикс фото это ~46 МБ.
	if k := min(b.Dx()/tw, b.Dy()/th) / 2; k >= 2 {
		img = boxShrink(img, k)
		b = img.Bounds()
	}

	out := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	if b.Dx() == tw && b.Dy() == th {
		draw.Draw(out, out.Bounds(), img, b.Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Over, nil)
	}
	return out
}

func fit(w, h int) (int, int) {
	if w <= MaxSide && h <= MaxSide {
		return w, h
	}
	if w >= h {
		return MaxSide, max(1, (h*MaxSide+w/2)/w)
	}
	return max(1, (w*MaxSide+h/2)/h), MaxSide
}

// boxShrink усредняет блоки k×k пикселей; результат premultiplied RGBA.
func boxShrink(img image.Image, k int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx()/k, b.Dy()/k
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	n := uint32(k * k)

	ycc, isYCC := img.(*image.YCbCr)
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*k
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*k
			var r, g, bl, a uint32
			if isYCC {
				for j := 0; j < k; j++ {
					for i := 0; i < k; i++ {
						c := ycc.YCbCrAt(sx+i, sy+j)
						rr, gg, bb := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)
						r += uint32(rr)
						g += uint32(gg)
						bl += uint32(bb)
					}
				}
				a = 255 * n
			} else {
				for j := 0; j < k; j++ {
					for i := 0; i < k; i++ {
						rr, gg, bb, aa := img.At(sx+i, sy+j).RGBA()
						r += rr >> 8
						g += gg >> 8
						bl += bb >> 8
						a += aa >> 8
					}
				}
			}
			o := out.PixOffset(x, y)
			out.Pix[o] = uint8(r / n)
			out.Pix[o+1] = uint8(g / n)
			out.Pix[o+2] = uint8(bl / n)
			out.Pix[o+3] = uint8(a / n)
		}
	}
	return out
}

// orient приводит картинку к нормальному виду по значению EXIF Orientation (1–8).
func orient(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var tx, ty int
			switch o {
			case 2:
				tx, ty = w-1-x, y
			case 3:
				tx, ty = w-1-x, h-1-y
			case 4:
				tx, ty = x, h-1-y
			case 5:
				tx, ty = y, x
			case 6:
				tx, ty = h-1-y, x
			case 7:
				tx, ty = h-1-y, w-1-x
			case 8:
				tx, ty = y, w-1-x
			}
			copy(dst.Pix[dst.PixOffset(tx, ty):][:4], src.Pix[src.PixOffset(x, y):][:4])
		}
	}
	return dst
}

func writeAtomic(dst string, img image.Image) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*.jpg")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: Quality}); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	os.Chmod(tmpName, 0o644)
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
