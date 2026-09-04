// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// testPNG encodes a solid-color square of the given size, small enough to
// keep the test fast but big enough to exercise icnsTypeFor's boundaries.
func testPNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

func TestPNGDimensions(t *testing.T) {
	data := testPNG(t, 37)
	w, h, err := pngDimensions(data)
	if err != nil {
		t.Fatalf("pngDimensions: %v", err)
	}
	if w != 37 || h != 37 {
		t.Errorf("pngDimensions = %d,%d, want 37,37", w, h)
	}
}

func TestPNGDimensionsRejectsNonPNG(t *testing.T) {
	if _, _, err := pngDimensions([]byte("not a png")); err == nil {
		t.Fatal("expected an error for non-PNG data")
	}
}

// The ICO header has to describe exactly what follows it: a single
// PNG-compressed image starting right after the 6-byte ICONDIR and 16-byte
// ICONDIRENTRY, which is what Windows Vista+ requires to render it at all.
func TestWrapPNGAsICO(t *testing.T) {
	png := testPNG(t, 64)
	ico, err := wrapPNGAsICO(png)
	if err != nil {
		t.Fatalf("wrapPNGAsICO: %v", err)
	}
	if len(ico) != 6+16+len(png) {
		t.Fatalf("ico length = %d, want %d", len(ico), 6+16+len(png))
	}
	if binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		t.Error("ICONDIR type != 1 (icon)")
	}
	if binary.LittleEndian.Uint16(ico[4:6]) != 1 {
		t.Error("ICONDIR count != 1")
	}
	if ico[6] != 64 || ico[7] != 64 {
		t.Errorf("ICONDIRENTRY dimensions = %d,%d, want 64,64", ico[6], ico[7])
	}
	if got := binary.LittleEndian.Uint32(ico[8+6 : 8+10]); got != uint32(len(png)) {
		t.Errorf("bytesInRes = %d, want %d", got, len(png))
	}
	if off := binary.LittleEndian.Uint32(ico[8+10 : 8+14]); off != 22 {
		t.Errorf("imageOffset = %d, want 22", off)
	}
	if !bytes.Equal(ico[22:], png) {
		t.Error("PNG payload was not carried through unchanged")
	}
}

// A 256px (or larger) dimension is stored as 0 — the ICO format's way of
// saying 256, since each dimension is a single byte.
func TestWrapPNGAsICOClamps256(t *testing.T) {
	png := testPNG(t, 256)
	ico, err := wrapPNGAsICO(png)
	if err != nil {
		t.Fatalf("wrapPNGAsICO: %v", err)
	}
	if ico[6] != 0 || ico[7] != 0 {
		t.Errorf("ICONDIRENTRY dimensions = %d,%d, want 0,0 (256)", ico[6], ico[7])
	}
}

func TestIcnsTypeFor(t *testing.T) {
	for _, tc := range []struct {
		size int
		want string
	}{
		{16, "ic07"},
		{128, "ic07"},
		{129, "ic08"},
		{256, "ic08"},
		{257, "ic09"},
		{512, "ic09"},
		{513, "ic10"},
		{1024, "ic10"},
	} {
		if got := icnsTypeFor(tc.size); got != tc.want {
			t.Errorf("icnsTypeFor(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

func TestWrapPNGAsICNS(t *testing.T) {
	png := testPNG(t, 256)
	icns, err := wrapPNGAsICNS(png)
	if err != nil {
		t.Fatalf("wrapPNGAsICNS: %v", err)
	}
	if string(icns[0:4]) != "icns" {
		t.Fatalf("magic = %q, want \"icns\"", icns[0:4])
	}
	total := binary.BigEndian.Uint32(icns[4:8])
	if int(total) != len(icns) {
		t.Errorf("total length = %d, want %d", total, len(icns))
	}
	if typ := string(icns[8:12]); typ != "ic08" {
		t.Errorf("entry type = %q, want ic08 for a 256px image", typ)
	}
	entryLen := binary.BigEndian.Uint32(icns[12:16])
	if int(entryLen) != 8+len(png) {
		t.Errorf("entry length = %d, want %d", entryLen, 8+len(png))
	}
	if !bytes.Equal(icns[16:], png) {
		t.Error("PNG payload was not carried through unchanged")
	}
}
