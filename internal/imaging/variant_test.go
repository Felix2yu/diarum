package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestResolveVariant(t *testing.T) {
	for _, name := range []string{"thumb", "THUMB", " medium "} {
		if _, ok := ResolveVariant(name); !ok {
			t.Fatalf("ResolveVariant(%q) should succeed", name)
		}
	}
	if _, ok := ResolveVariant("large"); ok {
		t.Fatal("ResolveVariant(large) should fail")
	}
}

func TestResolveSize(t *testing.T) {
	variant, ok := ResolveSize("300x200")
	if !ok || variant.Width != 300 || variant.Height != 200 {
		t.Fatalf("ResolveSize(300x200) = %+v, %v", variant, ok)
	}
	variant, ok = ResolveSize("1x9999")
	if !ok || variant.Width != minVariantSide || variant.Height != maxVariantSide {
		t.Fatalf("ResolveSize should clamp: %+v, %v", variant, ok)
	}
	for _, raw := range []string{"", "300", "abc", "300x", "0x0"} {
		if _, ok := ResolveSize(raw); ok {
			t.Fatalf("ResolveSize(%q) should fail", raw)
		}
	}
}

func TestGeneratePNGKeepsAlphaFormat(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for x := 0; x < 1000; x++ {
		for y := 0; y < 500; y++ {
			src.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(bytes.NewReader(input.Bytes()), Variant{Width: 400, Height: 400, Quality: 82})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.ContentType != "image/png" || result.Extension != "png" {
		t.Fatalf("unexpected format: %+v", result)
	}
	decoded, err := png.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if got := decoded.Bounds().Dx(); got != 400 {
		t.Fatalf("width = %d, want 400", got)
	}
	if got := decoded.Bounds().Dy(); got != 200 {
		t.Fatalf("height = %d, want 200", got)
	}
}

func TestGenerateJPEGUsesJPEG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 600))
	var input bytes.Buffer
	if err := jpeg.Encode(&input, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(bytes.NewReader(input.Bytes()), Variant{Width: 400, Height: 400, Quality: 80})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.ContentType != "image/jpeg" || result.Extension != "jpg" {
		t.Fatalf("unexpected format: %+v", result)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Width != 400 || cfg.Height != 300 {
		t.Fatalf("size = %dx%d, want 400x300", cfg.Width, cfg.Height)
	}
}

func TestGenerateRejectsInvalidData(t *testing.T) {
	if _, err := Generate(bytes.NewReader([]byte("not an image")), Variant{Width: 400, Height: 400}); err == nil {
		t.Fatal("Generate should reject invalid data")
	}
}

func TestJPEGOrientationReadsExif(t *testing.T) {
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	segment := append([]byte("Exif\x00\x00"), tiff...)
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x00}
	size := len(segment) + 2
	data[4] = byte(size >> 8)
	data[5] = byte(size)
	data = append(data, segment...)
	data = append(data, 0xFF, 0xD9)

	if got := jpegOrientation(data); got != 6 {
		t.Fatalf("jpegOrientation = %d, want 6", got)
	}
	if got := jpegOrientation([]byte("not-jpeg")); got != 1 {
		t.Fatalf("jpegOrientation(invalid) = %d, want 1", got)
	}
}

func TestOrientRotatesDimensions(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 3))
	rotated := orient(src, 6)
	if got := rotated.Bounds(); got.Dx() != 3 || got.Dy() != 2 {
		t.Fatalf("orient(6) bounds = %v, want 3x2", got)
	}
	unchanged := orient(src, 1)
	if got := unchanged.Bounds(); got.Dx() != 2 || got.Dy() != 3 {
		t.Fatalf("orient(1) bounds = %v, want 2x3", got)
	}
}
