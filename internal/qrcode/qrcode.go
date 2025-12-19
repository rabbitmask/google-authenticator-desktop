package qrcode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"

	"github.com/liyue201/goqr"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	qr "github.com/skip2/go-qrcode"
	"golang.org/x/image/draw"
)

// ScanQRCode scans a QR code from an image and returns the decoded text
func ScanQRCode(img image.Image) (string, error) {
	// 预处理：确保无透明背景 + 添加白边
	img = removeAlpha(img)
	img = addQuietZone(img, 20)

	// 如果图片太小，先放大
	bounds := img.Bounds()
	if bounds.Dx() < 400 || bounds.Dy() < 400 {
		img = scaleImage(img, 4) // 放大4倍
	}

	// 方法1: 使用 goqr 库（更稳定）
	result, err := tryGoQR(img)
	if err == nil && result != "" {
		return result, nil
	}

	// 方法2: 使用 gozxing 原始图像
	result, err = tryGozxing(img)
	if err == nil && result != "" {
		return result, nil
	}

	// 方法3: 转换为灰度图后使用 gozxing
	grayImg := toGrayscale(img)
	result, err = tryGozxing(grayImg)
	if err == nil && result != "" {
		return result, nil
	}

	// 方法4: 使用 goqr 处理灰度图
	result, err = tryGoQR(grayImg)
	if err == nil && result != "" {
		return result, nil
	}

	// 方法5: 二值化处理后使用 gozxing（尝试不同阈值）
	for _, threshold := range []uint8{128, 100, 150, 80, 180, 60, 200} {
		binImg := toBinary(grayImg, threshold)
		result, err = tryGozxing(binImg)
		if err == nil && result != "" {
			return result, nil
		}
		result, err = tryGoQR(binImg)
		if err == nil && result != "" {
			return result, nil
		}
	}

	return "", fmt.Errorf("failed to decode QR code after multiple attempts")
}

// removeAlpha 移除透明通道，用白色填充
func removeAlpha(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	// 先填充白色背景
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, color.White)
		}
	}

	// 绘制原图（透明部分会显示白色背景）
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			if a > 0 {
				// 混合透明度
				alpha := float64(a) / 65535.0
				white := 65535.0 * (1 - alpha)
				newR := uint8((float64(r)*alpha + white) / 256)
				newG := uint8((float64(g)*alpha + white) / 256)
				newB := uint8((float64(b)*alpha + white) / 256)
				dst.Set(x, y, color.RGBA{newR, newG, newB, 255})
			}
		}
	}

	return dst
}

// addQuietZone 添加白边（QR码需要的静默区）
func addQuietZone(img image.Image, padding int) image.Image {
	bounds := img.Bounds()
	newWidth := bounds.Dx() + padding*2
	newHeight := bounds.Dy() + padding*2

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 填充白色背景
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			dst.Set(x, y, color.White)
		}
	}

	// 将原图绘制到中心
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X+padding, y-bounds.Min.Y+padding, img.At(x, y))
		}
	}

	return dst
}

// scaleImage 放大图片
func scaleImage(img image.Image, scale int) image.Image {
	bounds := img.Bounds()
	newWidth := bounds.Dx() * scale
	newHeight := bounds.Dy() * scale

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}

// tryGoQR 使用 goqr 库解码
func tryGoQR(img image.Image) (string, error) {
	qrCodes, err := goqr.Recognize(img)
	if err != nil {
		return "", err
	}
	if len(qrCodes) == 0 {
		return "", fmt.Errorf("no QR code found")
	}
	return string(qrCodes[0].Payload), nil
}

// tryGozxing 使用 gozxing 库解码
func tryGozxing(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}

	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}

	return result.GetText(), nil
}

// toGrayscale converts an image to grayscale
func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}

	return gray
}

// toBinary converts a grayscale image to binary (black and white)
func toBinary(img *image.Gray, threshold uint8) *image.Gray {
	bounds := img.Bounds()
	binary := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.GrayAt(x, y)
			if c.Y > threshold {
				binary.SetGray(x, y, color.Gray{Y: 255})
			} else {
				binary.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return binary
}

// ScanQRCodeFromBytes scans a QR code from image bytes
func ScanQRCodeFromBytes(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	return ScanQRCode(img)
}

// GenerateQRCode generates a QR code image from text
func GenerateQRCode(text string, size int) ([]byte, error) {
	if size == 0 {
		size = 512 // Default size
	}

	// Generate QR code
	png, err := qr.Encode(text, qr.High, size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	return png, nil
}

// GenerateQRCodeBase64 generates a QR code and returns it as base64 data URL
func GenerateQRCodeBase64(text string, size int) (string, error) {
	png, err := GenerateQRCode(text, size)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(png)
	return fmt.Sprintf("data:image/png;base64,%s", encoded), nil
}

// GenerateQRCodeImage generates a QR code and returns it as an image.Image
func GenerateQRCodeImage(text string, size int) (image.Image, error) {
	png, err := GenerateQRCode(text, size)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		return nil, fmt.Errorf("failed to decode generated QR code: %w", err)
	}

	return img, nil
}
