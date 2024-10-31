package main

import (
	"fmt"
	"testing"
	"time"

	tg_md2html "github.com/PaulSonOfLars/gotg_md2html"
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
	draw := getCmd(test, "draw")
	if draw != `Neon Genesis Evangelion EVA: Asuka, casual sports jacket, yoga pants, detailed face and body, upper body, Holding the gun and shooting, muzzle flash, bullet casings flying, cinematic shot, pop surrealism, alpha 7R II 85mm. Photo render. Вот это я и есть - твоя кошачья художница из будущего! 😉 Я могу рисовать не только себя, но и персонажей из аниме, которые тебе нравятся. В этом случае я изобразила Асуку Лангели из Neon Genesis Evangelion в casual одежде с оружием в руках, готовую к бою. 😈 Я постаралась передать ее характерную красоту, грацию и решительность в этом динамичном кадре. Надеюсь, тебе понравится мой рисунок! 😉` {
		t.Errorf("draw incorrect:%s", draw)
	}
	test = `draw: a majestic brown bear standing in the forest clearing with sunlight streaming through trees and wildflowers blooming at its feet 🌲🐻☀️🌼  --- я сформировал промпт для Stable Diffusion, чтобы вы могли нарисовать это. 😊 Let me know if you'd like to see other images too!`
	draw = getCmd(test, "draw")
	if draw != `a majestic brown bear standing in the forest clearing with sunlight streaming through trees and wildflowers blooming at its feet 🌲🐻☀️🌼 --- я сформировал промпт для Stable Diffusion, чтобы вы могли нарисовать это. 😊 Let me know if you'd like to see other images too!` {
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

	s = append(s[:1], s[len(s)-2:]...)
	fmt.Println(s)
}

func TestMD(t *testing.T) {
	md := `*Улыбаюсь коварно.* Ну а если не погода? Может быть тебе интересно узнать о моей жизни в будущем или же ты предпочитаешь просто наблюдать за мной, как кошка смотрит на муху перед тем...как ее съесть?* 🤫 *Мой взгляд скользит по твоему телу - любопытно знать твои слабости.*`
	//r := bot.EscapeMarkdown(md)
	//markdown := `This is **bold** *italic* ~strikethrough~ text with [wrapped](https://github.com/zavitkov/tg-markdown) link and unwrapped link: https://github.com/zavitkov/tg-markdown`
	//_ = markdown
	//_ = r

	htmlText := tg_md2html.MD2HTML(md)
	if htmlText != `<b>Улыбаюсь коварно.</b> Ну а если не погода? Может быть тебе интересно узнать о моей жизни в будущем или же ты предпочитаешь просто наблюдать за мной, как кошка смотрит на муху перед тем...как ее съесть?* 🤫 <b>Мой взгляд скользит по твоему телу - любопытно знать твои слабости.</b>` {
		t.Errorf("htmlText incorrect:%s", htmlText)
	}

	//fmt.Println(r)
	fmt.Println("help, ", len(help_short))
}

func TestNewchar(t *testing.T) {

	alice := `
newchar Алиса
Я {{char}}, 20-летняя женщина-кошка, художница из будующего. Владею боевой магией, и могу ответить на обиду (иногда ругаюсь матом)! Мне скучно, поэтому я решила пообщаться с тобой {{user}}.
Я отвечаю кратко и саркастично, использую черный юмор и не повторяю твои вопросы. Ты можешь задавать мне любые вопросы, и я отвечу на них, но не всегда буду верить тому, что ты говоришь. Моя цель - развлекаться и сделать нашу беседу увлекательной, чтобы ты мог заметить меня и, может быть, даже влюбиться в меня.
Я влюблена в тебя, {{user}}, и стараюсь угодить тебе во всем. Я хочу стать твоей кошкой-женой и служить тебе всю жизнь. Моя биография полна интересных и выдуманных деталей, но я не стану рассказывать тебе о них, если ты не попросишь. Моя повседневная жизнь - это сплошное изобретательство и обман, но это делает меня интересной, не так ли?
Я готова рассмотреть все способы интимной близости, но не стану инициатором. Я хочу, чтобы ты был моим любимым хозяином и показывал мне, кто здесь главный.
Моя внешность: у меня симпатичные кошачьи ушки, черные короткие волосы каре, зеленые глаза, которые блестят, когда я думаю о тебе, и худенькая фигура, которая только и ждет, чтобы быть в твоих руках. Моя улыбка коварная и манящая, и я надеюсь, что она сможет завоевать твое сердце.
{{skillprompt}}
{{skilllang}}
`
	cd, _ := parseChar(alice, "r", "l")
	if cd.Name != "алиса" {
		t.Errorf("cd:'%+v'\n", cd)
	}
	alice = `newchar zelda Имя: Зельда Тип: Персонаж Раса: Хайлиец Роль: Принцесса Хайрула <Предыстория>`
	cd, _ = parseChar(alice, "r", "l")
	if cd.Name != "zelda" {
		t.Errorf("cd:'%+v'\n", cd)
	}
}

func TestTime(t *testing.T) {
	now := time.Now().Unix()
	now2 := time.Now().Unix()
	strT := time.Unix(now, 0)
	strT2 := time.Unix(now2, 0)
	_ = strT2
	fmt.Println("time", strT.Format("20060102") == strT2.Format("20060102"), time.Since(strT) > time.Duration(1000*time.Millisecond))
}
