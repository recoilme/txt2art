package main

import "testing"

func TestEng(t *testing.T) {
	test := `Ultra-detailed, photorealistic FullHD, fisheye lens, cinematic lighting, wet process, cinematic postprocess, wide gamut colors, heavy contrast, overexposed, underexposed, of 

		FUTURAMA ONE LOVE`
	got := hasNonEnglish(test)
	if got == true {
		t.Error("has")
	}
}
