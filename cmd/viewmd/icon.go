// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
)

// pngDimensions reads width/height from a PNG's header without decoding the
// whole image.
func pngDimensions(data []byte) (width, height int, err error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("not a valid PNG: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// wrapPNGAsICO wraps PNG-encoded data in a single-image .ico container.
// Windows Vista and later render a PNG-compressed icon image directly, so no
// pixel decoding or format conversion is needed — just the ICONDIR /
// ICONDIRENTRY header the format expects around the untouched PNG bytes.
func wrapPNGAsICO(pngData []byte) ([]byte, error) {
	w, h, err := pngDimensions(pngData)
	if err != nil {
		return nil, err
	}
	// ICO stores each dimension in a single byte, 0 meaning 256.
	dim := func(n int) byte {
		if n <= 0 || n >= 256 {
			return 0
		}
		return byte(n)
	}

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // type: icon
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // image count
	buf.WriteByte(dim(w))
	buf.WriteByte(dim(h))
	buf.WriteByte(0)                                       // color count (0 = not palette-based)
	buf.WriteByte(0)                                       // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))  // color planes
	_ = binary.Write(buf, binary.LittleEndian, uint16(32)) // bits per pixel
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(pngData)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(22)) // offset: 6-byte header + 16-byte entry
	buf.Write(pngData)
	return buf.Bytes(), nil
}

// icnsTypeFor picks an icns icon type raw-PNG data can be stored under,
// keyed by the image's larger dimension. macOS has accepted a PNG payload
// directly under these OSTypes since Lion; there is no unambiguous @1x type
// below 128px, so anything smaller is stored (and upscaled by Finder) as
// ic07 — a soft image beats no custom icon at all.
func icnsTypeFor(size int) string {
	switch {
	case size <= 128:
		return "ic07"
	case size <= 256:
		return "ic08"
	case size <= 512:
		return "ic09"
	default:
		return "ic10"
	}
}

// wrapPNGAsICNS wraps PNG-encoded data in a single-image .icns container.
func wrapPNGAsICNS(pngData []byte) ([]byte, error) {
	w, h, err := pngDimensions(pngData)
	if err != nil {
		return nil, err
	}
	size := w
	if h > size {
		size = h
	}
	typ := icnsTypeFor(size)

	entryLen := uint32(8 + len(pngData))
	total := uint32(8) + entryLen

	buf := new(bytes.Buffer)
	buf.WriteString("icns")
	_ = binary.Write(buf, binary.BigEndian, total)
	buf.WriteString(typ)
	_ = binary.Write(buf, binary.BigEndian, entryLen)
	buf.Write(pngData)
	return buf.Bytes(), nil
}
