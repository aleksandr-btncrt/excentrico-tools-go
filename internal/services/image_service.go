package services

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

type ImageService struct {
	maxWidth  int
	maxHeight int
	quality   int
}

func NewImageServiceWithConfig(maxWidth, maxHeight, quality int) *ImageService {
	return &ImageService{
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
		quality:   quality,
	}
}

func (s *ImageService) ResizeImage(inputPath, outputPath string) error {

	src, err := imaging.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}

	resized := imaging.Fit(src, s.maxWidth, s.maxHeight, imaging.Lanczos)

	err = s.saveImage(resized, outputPath)
	if err != nil {
		return fmt.Errorf("failed to save resized image: %v", err)
	}

	log.Printf("Resized image: %s -> %s", inputPath, outputPath)
	return nil
}

func (s *ImageService) saveImage(img image.Image, outputPath string) error {

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: s.quality})
	case ".png":
		return png.Encode(file, img)
	default:
		return fmt.Errorf("unsupported image format: %s", ext)
	}
}
