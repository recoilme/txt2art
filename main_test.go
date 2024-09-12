package main

import (
	"fmt"
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
	test = `draw: a majestic brown bear standing in the forest clearing with sunlight streaming through trees and wildflowers blooming at its feet 🌲🐻☀️🌼  --- я сформировал промпт для Stable Diffusion, чтобы вы могли нарисовать это. 😊 Let me know if you'd like to see other images too!`
	draw = getDraw(test)
	if draw != "a majestic brown bear standing in the forest clearing with sunlight streaming through trees and wildflowers blooming at its feet 🌲🐻☀️🌼" {
		t.Errorf("draw incorrect:%s", draw)
	}
}

func TestExtrSlice(t *testing.T) {
	s := make([]int, 0)
	s = append(s, 0)
	s = append(s, 1)
	s = append(s, 2)
	s = append(s, 3)
	s = append(s, 4)
	s = append(s, 5)
	s = append(s, 6)
	s = append(s, 7)
	s = append(s, 8)
	s = append(s, 9)
	s = append(s, 10)

	s = append(s[:1], s[9:]...)
	fmt.Println(s)

}
