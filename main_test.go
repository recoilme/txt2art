package main

import (
	"testing"
)

func TestEng(t *testing.T) {
	test := `Ultra-detailed, photorealistic FullHD, fisheye lens, cinematic lighting, wet process, cinematic postprocess, wide gamut colors, heavy contrast, overexposed, underexposed, of 

		FUTURAMA ONE LOVE`
	got := hasNonEnglish(test)
	if got == true {
		t.Error("has")
	}
}

func TestExtrDraw(t *testing.T) {
	test := `draw: Neon Genesis Evangelion EVA: Asuka, casual sports jacket, yoga pants, detailed face and body, upper body, Holding the gun and shooting, muzzle flash, bullet casings flying, cinematic shot, pop surrealism, alpha 7R II 85mm. Photo render.

Вот это я и есть - твоя кошачья художница из будущего! 😉 Я могу рисовать не только себя, но и персонажей из аниме, которые тебе нравятся. В этом случае я изобразила Асуку Лангели из Neon Genesis Evangelion в casual одежде с оружием в руках, готовую к бою. 😈

Я постаралась передать ее характерную красоту, грацию и решительность в этом динамичном кадре. Надеюсь, тебе понравится мой рисунок! 😉`
	draw := getDraw(test)
	if draw != "Neon Genesis Evangelion EVA: Asuka, casual sports jacket, yoga pants, detailed face and body, upper body, Holding the gun and shooting, muzzle flash, bullet casings flying, cinematic shot, pop surrealism, alpha 7R II 85mm. Photo render." {
		t.Errorf("draw incorrect:%s", draw)
	}
}
