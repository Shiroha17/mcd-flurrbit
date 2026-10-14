package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"log"
	"os"

	"flurrbit/internal/sprite"
)

const (
	logoPath = "docs/logo.png"
	logoSize = 512
	iconPath = "cmd/flurrbit/flurrbit.ico"
)

var iconSizes = []int{16, 24, 32, 48, 64, 128, 256}

const (
	icoHeaderSize         = 6
	icoDirectoryEntrySize = 16
	icoResourceTypeIcon   = 1
	icoColorPlanes        = 1
	icoBitsPerPixel       = 32
)

func main() {
	logo, err := encodeIconPNG(logoSize)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(logoPath, logo, 0o644); err != nil {
		log.Fatal(err)
	}

	icon, err := buildICO(iconSizes)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(iconPath, icon, 0o644); err != nil {
		log.Fatal(err)
	}
}

func encodeIconPNG(size int) ([]byte, error) {
	var buffer bytes.Buffer
	err := png.Encode(&buffer, sprite.Icon(size))

	return buffer.Bytes(), err
}

func buildICO(sizes []int) ([]byte, error) {
	var directory, images bytes.Buffer
	binary.Write(&directory, binary.LittleEndian, [3]uint16{0, icoResourceTypeIcon, uint16(len(sizes))})

	imageOffset := icoHeaderSize + icoDirectoryEntrySize*len(sizes)
	for _, size := range sizes {
		imageData, err := encodeIconPNG(size)
		if err != nil {
			return nil, err
		}

		dimension := icoDimension(size)
		directory.Write([]byte{dimension, dimension, 0, 0})
		binary.Write(&directory, binary.LittleEndian, [2]uint16{icoColorPlanes, icoBitsPerPixel})
		binary.Write(&directory, binary.LittleEndian, [2]uint32{uint32(len(imageData)), uint32(imageOffset)})

		imageOffset += len(imageData)
		images.Write(imageData)
	}

	return append(directory.Bytes(), images.Bytes()...), nil
}

func icoDimension(size int) byte {
	if size >= 256 {
		return 0
	}

	return byte(size)
}
