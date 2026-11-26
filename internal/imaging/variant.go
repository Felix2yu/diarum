// Package imaging 负责生成媒体缩略图与中等尺寸变体。
package imaging

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

// Variant 描述一个媒体变体的目标尺寸与编码质量。
type Variant struct {
	Name    string
	Width   int
	Height  int
	Quality int
}

// Result 是生成后的变体数据。
type Result struct {
	Data        []byte
	ContentType string
	Extension   string
}

var presetVariants = map[string]Variant{
	"thumb":  {Name: "thumb", Width: 400, Height: 400, Quality: 82},
	"medium": {Name: "medium", Width: 1200, Height: 1200, Quality: 85},
}

const (
	maxSourceBytes = 60 * 1024 * 1024
	minVariantSide = 16
	maxVariantSide = 1600
)

// ResolveVariant 解析预设变体名（thumb / medium）。
func ResolveVariant(raw string) (Variant, bool) {
	variant, ok := presetVariants[strings.ToLower(strings.TrimSpace(raw))]
	return variant, ok
}

// ResolveSize 解析 "300x300" 形式的尺寸参数，并限制在安全范围内。
func ResolveSize(raw string) (Variant, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	parts := strings.Split(raw, "x")
	if len(parts) != 2 {
		return Variant{}, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return Variant{}, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return Variant{}, false
	}
	if width <= 0 || height <= 0 {
		return Variant{}, false
	}
	width = clamp(width, minVariantSide, maxVariantSide)
	height = clamp(height, minVariantSide, maxVariantSide)
	return Variant{
		Name:    fmt.Sprintf("thumb-%dx%d", width, height),
		Width:   width,
		Height:  height,
		Quality: 82,
	}, true
}

// Generate 读取原始图片并生成变体。SVG 等无法解码的格式会返回 ErrUnsupported。
func Generate(reader io.Reader, variant Variant) (*Result, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxSourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSourceBytes {
		return nil, fmt.Errorf("source image exceeds %d bytes", maxSourceBytes)
	}

	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	// GIF 会丢失动画，WebP 的透明通道无法用标准库编码，统一回退原图。
	if format == "gif" || format == "webp" {
		return nil, fmt.Errorf("variant unsupported for %s", format)
	}

	orientation := 1
	if format == "jpeg" {
		orientation = jpegOrientation(data)
	}
	scaled := scaleWithin(orient(src, orientation), variant.Width, variant.Height)

	switch format {
	case "png":
		var buf bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(&buf, scaled); err != nil {
			return nil, err
		}
		return &Result{Data: buf.Bytes(), ContentType: "image/png", Extension: "png"}, nil
	default:
		var buf bytes.Buffer
		quality := variant.Quality
		if quality <= 0 || quality > 100 {
			quality = 82
		}
		if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		return &Result{Data: buf.Bytes(), ContentType: "image/jpeg", Extension: "jpg"}, nil
	}
}

func scaleWithin(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}

	scale := 1.0
	if width > maxWidth {
		scale = float64(maxWidth) / float64(width)
	}
	if height > maxHeight {
		if ratio := float64(maxHeight) / float64(height); ratio < scale {
			scale = ratio
		}
	}
	if scale >= 1 {
		return src
	}

	targetWidth := max(1, int(float64(width)*scale))
	targetHeight := max(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

// orient 应用 EXIF 方向。浏览器会按原图 EXIF 显示，而变体重新编码后不再携带
// EXIF，因此必须把旋转烘焙进像素，否则手机照片的缩略图会横躺。
func orient(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	swap := orientation >= 5
	dstWidth, dstHeight := width, height
	if swap {
		dstWidth, dstHeight = height, width
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var dx, dy int
			switch orientation {
			case 2:
				dx, dy = width-1-x, y
			case 3:
				dx, dy = width-1-x, height-1-y
			case 4:
				dx, dy = x, height-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = height-1-y, x
			case 7:
				dx, dy = height-1-y, width-1-x
			case 8:
				dx, dy = y, width-1-x
			}
			dst.Set(dx, dy, img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return dst
}

// jpegOrientation 读取 EXIF 方向标签，缺失或非法时返回 1。
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		segment := data[i+4 : i+2+size]
		if marker == 0xE1 && len(segment) > 6 && string(segment[:6]) == "Exif\x00\x00" {
			return exifOrientation(segment[6:])
		}
		i += 2 + size
	}
	return 1
}

func exifOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	offset := int(order.Uint32(tiff[4:8]))
	if offset+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	for i := 0; i < count; i++ {
		entry := offset + 2 + i*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 {
			value := int(order.Uint16(tiff[entry+8 : entry+10]))
			if value >= 1 && value <= 8 {
				return value
			}
			return 1
		}
	}
	return 1
}
