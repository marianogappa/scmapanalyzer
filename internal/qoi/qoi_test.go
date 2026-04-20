package qoi

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTripRGBA(t *testing.T) {
	const w, h = 17, 23
	pix := make([]byte, w*h*4)
	for i := range pix {
		pix[i] = byte((i * 13) & 0xff)
	}
	enc, err := EncodeRGBA(w, h, pix)
	if err != nil {
		t.Fatal(err)
	}
	img, err := DecodeToNRGBA(enc)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("dims got %dx%d want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}
	if !bytes.Equal(pix, img.Pix) {
		t.Fatal("pixel mismatch")
	}
}
