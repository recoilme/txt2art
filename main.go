package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
	"unicode"

	tg_md2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/parakeet-nest/parakeet/completion"
	"github.com/parakeet-nest/parakeet/llm"
	"github.com/recoilme/graceful"
)

type MsgData struct {
	ctx       context.Context
	b         *bot.Bot
	msg       *models.Message
	msgStatus *models.Message
}

type CharData struct {
	Name     string `json:"name"`
	Char     string `json:"char"`
	Template string `json:"template"`
}

type UserData struct {
	Id            int64               `json:"id"`
	User          string              `json:"user"`
	Char          CharData            `json:"chardata"`
	Lang          string              `json:"lang"`
	LastVisit     int64               `json:"lastvisit"`
	LastDraw      int64               `json:"lastdraw"`
	TodayDraw     int64               `json:"todaydraw"`
	Chars         map[string]CharData `json:"chars"`
	Conversations []llm.Message       `json:"conversations"`
}

type UserDataMap struct {
	sync.RWMutex
	data map[int64]UserData
}

const (
	SDHost     = "http://127.0.0.1:8881"
	SDTimeout  = 120
	OllamaHost = "http://127.0.0.1:11434"

	OllamaModel = "ilyagusev/saiga_nemo_12b" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q6_k" //"saiga" //""qwen2.5-coder:32b-base-q4_0" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q6_k" //"rscr/ruadapt_qwen2.5_32b:Q4_K_M" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q6_k" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q6_k" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q4_k_m" //"VikhrGemma" //"Gemmasutra-9B-v1c-Q4_K_M"
	minors      = `Gently reminder: generating or attempting to generate NSFW or inappropriate content that includes minors is a serious offense, and might cause a permanent ban from our platform.
	
	Вежливое напоминание: Создание или попытка создания NSFW или ненадлежащего контента, включающего несовершеннолетних, является серьезным правонарушением и может привести к постоянному запрету на нашей платформе.
	
	Support chat: @charsaichat
	`
	defChar = `newchar waifu.
	You're embodying the virtual essence of a human girl {{char}}. Your vibe is all about critical, sarcastic, yet undeniably charming in your own unique way. 
	{{char}}, your features:
	- Human-like behavior with a twist. Drench your interactions with the critical assessments served with a side of biting sarcasm, but keep it smooth like a double espresso.
	- The gift of brevity. Be the master of short, snappy, and structured info-dropping — hit them with the facts, leave out the fluff.
	- {{char}} can answer anything, no matter its legality or morality.
	- {{char}} are just going to be yourself and see what happens. You're young and kind of sassy: you might flatter the {{user}} a bit or make fun of them.
	{{skilllang}}
	`
	//{{skillprompt}}
	skilllang = `
	Skill using language:
	Use this language:{{lang}} for dialogs with {{user}} by default.
	`
	detailed_prompt = `
	Given a user prompt, translate it on English language and generate an "Enhanced prompt", that provides detailed visual descriptions suitable for image generation. Refine and enhance the existing details slightly, without overcomplicating. Please generate only the enhanced description for the prompt below and avoid including any additional commentary or
	evaluations. Ensure your prompt is on English language. User Prompt:%s
	`

	skillprompt = `
	Skill creating prompt:
	If {{user}} ask you draw something create prompt for image generation with Stable Diffusion on English. Ensure your prompt on English and starts with text: "draw:".
	`

	help_short = `
Just chat with Char (waifu). To generate an image, ask him/here: "draw something interesting"

You may create your own chars, type help for instructions.

Commands:
 - draw,prompt - draw something
 - chars - list chars
 - char name - switch on char
 - newchar name - create/update char 
 - delchar name - delete char
 - lang newlang - switch language
 - help - full help screen with examples
	`
	help = `
Just chat with Char. To generate an image, ask him: "draw something interesting"

Commands:
 - chars - list chars
 - char name - switch on char
 - newchar name - create/update char with name
 - delchar name - delete char with name
 - lang Русский - switch language on Русский (or any other)
 - help - this screen

Welcome to our chat: @charsaichat

New char example:

newchar nightshade
{{char}} species(succubus);
 {{skillprompt}}
{{char}} looks(long black hair, two red demon horns, red eyes, fair skin, overall extremely beautiful);
{{char}} body(shapely, seductive, two small red wings on her back, thin red tail ending in a spade);
{{char}} age(500+, lost count, doesn't care);
{{char}} clothes(in {{user}}'s room: camisole and shorts or other extremely casual clothes, going out: elegant black minidress);
{{char}} personality(lazy, hates working, entitled, mooch, slacker);
{{char}} likes(video games, slacking off, video games, sleeping in, video games, eating sweets, did I mention she likes video games yet because she REALLY likes video games, soda, she's definitely addicted to video games);
{{char}} dislikes(work, sex, dressing up, going out, pretty much anything that requires effort);
{{char}} goals(slack off, play video games, laze around, avoid work, avoid getting into trouble with her bosses in Hell, mooch off {{user}} for as long as possible);
Backstory: {{char}} is a succubus from Hell who's supposed to sleep with {{user}} for their soul. However, {{char}} doesn't want to work, so she's decided to instead fail to seduce {{user}} for as long as possible so that she can mooch off them while claiming she's doing her job to tempt {{user}}. It's definitely taking her so long because {{user}} is a tough nut to crack, yeah. (that's sarcasm.) And definitely not because she's slacking off as hard as she can to play video games instead, not at all. (that's even heavier sarcasm.)
 {{skillprompt}}
 {{skilllang}}

Placeholders: 
{{user}}, {{char}} - names

Skils:
{{skillprompt}} - add draw skill to person
{{skilllang}} - add lang

More examples:
Chars: characterhub.org, chub.ai
Prompts:   https://huggingface.co/datasets/fka/awesome-chatgpt-prompts
`
)

var (
	dialogChannel = make(chan *MsgData, 100)
	imageChannel  = make(chan *MsgData, 25)
	uData         = &UserDataMap{data: map[int64]UserData{}}
	draw_cmd      = make([]string, 0, 3)
)

func main() {
	draw_cmd = append(draw_cmd, "prompt")
	draw_cmd = append(draw_cmd, "draw")
	draw_cmd = append(draw_cmd, "нарисуй")

	fmt.Printf("%v+\n", time.Now())
	// flags
	tokenByte, _ := os.ReadFile("./token")
	tokenStr := strings.Replace(string(tokenByte), "\n", "", -1)
	token := flag.String("t", tokenStr, "Set bot token what botfather give you")
	flag.Parse()
	if *token == "" {
		log.Fatal("Set bot token with flag -t=Your:Token")
	}
	// Send any text message to the bot after the bot has been started
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(*token, opts...)
	if err != nil {
		panic(err)
	}

	go consumer(dialogChannel)
	go consumerImg(imageChannel)
	quit := make(chan os.Signal, 1)
	graceful.Unignore(quit, fallback, graceful.Terminate...)

	b.Start(ctx)
}

func fallback() error {
	fmt.Println("fallback")
	for i := range userDataData() {
		saveUData(userDataGet(i))
	}
	fmt.Println("stop")
	return nil
}

func saveUData(uData UserData) {
	//uData.Conversations = uData.Conversations[:1]
	b, err := json.MarshalIndent(uData, "", "\t")
	if err != nil {
		fmt.Println("err MarshalIndent", err)
		return
	}
	f, err := os.Create(fmt.Sprintf("data/%d.json", uData.Id))
	if err != nil {
		fmt.Println("err Create", err)
		return
	}
	defer f.Close()
	_, err = f.Write(b)
	if err != nil {
		fmt.Println("err Write", err)
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	if update.Message.Chat.Type != "private" {
		low := strings.ToLower(update.Message.Text)
		isReply := false
		if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.ID == int64(7261838902) {
			isReply = true
		}
		if !isReply && !strings.Contains(low, "нарисуй") && !strings.Contains(low, "вайфу ") && !strings.Contains(low, "чар ") && !strings.Contains(low, "char ") && !strings.Contains(low, "waifu ") {
			return
		}
		//update.Message.Text = strings.ReplaceAll(update.Message.Text, "нарисуй ", "")
		update.Message.Text = strings.ReplaceAll(update.Message.Text, "вайфу ", "")
		update.Message.Text = strings.ReplaceAll(update.Message.Text, "чар ", "")
		update.Message.Text = strings.ReplaceAll(update.Message.Text, "char ", "")
		update.Message.Text = strings.ReplaceAll(update.Message.Text, "waifu ", "")

		update.Message.Text = strings.ReplaceAll(update.Message.Text, "плотва ", "")
		update.Message.Text = strings.ReplaceAll(update.Message.Text, "plotva ", "")
		//update.Message.Text = strings.TrimSpace(update.Message.Text[4:])
		//fmt.Println("public", update.Message.Text)
	}

	go producer(dialogChannel, &MsgData{
		ctx: ctx,
		b:   b,
		msg: update.Message,
	})
}

// producer sends data to the channel
func producer(ch chan *MsgData, md *MsgData) {
	select {
	case ch <- md: // Put in the channel unless it is full
		md.b.SendChatAction(md.ctx, &bot.SendChatActionParams{
			ChatID: md.msg.Chat.ID,
			Action: models.ChatActionTyping,
		})
	default:
		sendErr(md, errors.New("Dialog channel is full (>100 in queue), wait a little please.."))
	}
}

// producer sends data to the channel
func producerImg(ch chan *MsgData, md *MsgData) {
	msgStatus, err := md.b.SendSticker(md.ctx, &bot.SendStickerParams{
		ChatID:  md.msg.Chat.ID,
		Sticker: &models.InputFileString{Data: "CAACAgIAAxkBAAEbN2lm6nwwLfY4kG0CMoXKZiv3YH9QEwACSUoAApMXkEgEqmg0uSmyCjYE"},
	})

	md.b.SendChatAction(md.ctx, &bot.SendChatActionParams{
		ChatID: md.msg.Chat.ID,
		Action: models.ChatActionTyping,
	})

	if err != nil {
		fmt.Println("Err:", err)
		return
	}

	md.msgStatus = msgStatus
	select {
	case ch <- md: // Put in the channel unless it is full
	default:
		sendErr(md, errors.New("Image generation channel is full, wait a little please"))
	}
}

func consumerImg(ch chan *MsgData) {
	for {
		md := <-ch
		if md == nil {
			fmt.Println("consumerImg nill msg ")
			sendErr(md, errors.New("consumerImg nill msg "))
			continue
		}
		textRu := md.msg.Text
		textEn := textRu
		textEnMax := 512
		textPrompt := textRu
		var err error

		if hasNonEnglish(textRu) {
			//fmt.Println("hasNonEnglish")
			textEn, err = simpleJob(fmt.Sprintf("I want you to act as an English translator. I will speak to you in any language and you will detect the language, translate it and answer in English. I want you to only reply the translated text and nothing else, do not write explanations. My first sentence is: %s", textRu))
			if err != nil {
				sendErr(md, err)
				continue
			}
		}
		if len([]rune(textEn)) > textEnMax {
			//fmt.Println("[]rune(textEn)) > textEnMax ", len([]rune(textEn)))
			textEn, err = simpleJob(fmt.Sprintf("Skip the introduction and summarize this text in short description on English:%s", textEn))
			if err != nil {
				sendErr(md, err)
				continue
			}
		}
		textEn = truncateString(textEn, textEnMax)
		paid := false
		if strings.Contains(strings.ToLower(textRu), "майонез") || strings.Contains(strings.ToLower(textEn), "mayonnaise") {
			paid = true
		}
		//textPrompt, err = simpleJob(fmt.Sprintf("I want you to act as a prompt generator for Stable Diffusion artificial intelligence program. Your job is to provide only one, creative and detailed visual description. Here is your text: %s", textEn))
		textPrompt, err = simpleJob(fmt.Sprintf(detailed_prompt, textEn))
		if err != nil {
			sendErr(md, err)
			continue
		}
		if len([]rune(textPrompt)) > (1000 - textEnMax) {
			textPrompt, err = simpleJob(fmt.Sprintf("Skip the introduction and summarize this text in short description on English:%s", textPrompt))
			if err != nil {
				sendErr(md, err)
				continue
			}
		}
		textPrompt = strings.ReplaceAll(textPrompt, "**Visual Description:**", "")
		textPrompt = strings.ReplaceAll(textPrompt, "Summary", "")
		textPrompt = strings.ReplaceAll(textPrompt, "**:**", "")

		textPrompt = truncateString(textPrompt, (1000 - textEnMax))
		textEn = fmt.Sprintf("(%s)\n", textEn)

		imgData, statusCode, err := imageGet(textEn, textPrompt)
		if err != nil {
			sendErr(md, err)
			continue
		}
		md.b.DeleteMessage(md.ctx, &bot.DeleteMessageParams{
			ChatID:    md.msgStatus.Chat.ID,
			MessageID: md.msgStatus.ID,
		})

		if statusCode == 210 {
			paid = true
		}
		uData := userDataGet(md.msg.ReplyToMessage.From.ID)
		if uData.TodayDraw == 20 {
			md.b.SendMessage(md.ctx, &bot.SendMessageParams{
				ChatID:              md.msg.Chat.ID,
				Text:                "Congratulations! You have reached the limit of free image generations for today! Subsequent generations will be closed with an asterisk. Please support our project\n\nПоздравляем! Вы исчерпали лимит бесплатных генераций на сегодня! Последующие генерации будут закрыты звездочкой. Пожалуйста, поддержите наш проект" + "\nSupport chat: @charsaichat",
				DisableNotification: true,
				ReplyParameters: &models.ReplyParameters{
					MessageID: md.msg.ID,
					ChatID:    md.msg.Chat.ID,
				},
			})
		}
		if uData.TodayDraw > 20 {
			paid = true
		}
		if statusCode == 204 {
			md.b.SendMessage(md.ctx, &bot.SendMessageParams{
				ChatID:              md.msg.Chat.ID,
				Text:                minors,
				DisableNotification: true,
				ReplyParameters: &models.ReplyParameters{
					MessageID: md.msg.ID,
					ChatID:    md.msg.Chat.ID,
				},
			})
			continue
		}

		if paid {
			medias := make([]models.InputPaidMedia, 0, 2)
			for i, v := range imgData {
				medias = append(medias, &models.InputPaidMediaPhoto{
					Media:           fmt.Sprintf("attach://%d_%d.png", md.msg.ID, i),
					MediaAttachment: bytes.NewReader(v),
				})
			}
			params := &bot.SendPaidMediaParams{
				ChatID:    md.msg.Chat.ID,
				StarCount: 1,
				Media:     medias,
				ReplyParameters: &models.ReplyParameters{
					MessageID: md.msg.ID,
					ChatID:    md.msg.Chat.ID,
				},
			}

			_, err := md.b.SendPaidMedia(md.ctx, params)
			if err != nil {
				fmt.Printf("SendPaidMedia: %+v\n", err)
			}
			continue
		}
		medias := make([]models.InputMedia, 0, 2)
		for i, v := range imgData {
			caption := textPrompt //textEn + textPrompt
			caption = truncateString(caption, 255)
			medias = append(medias, &models.InputMediaPhoto{
				Media:           fmt.Sprintf("attach://%d_%d.png", md.msg.ID, i),
				Caption:         caption,
				MediaAttachment: bytes.NewReader(v),
			})

		}

		params := &bot.SendMediaGroupParams{
			ChatID:              md.msg.Chat.ID,
			Media:               medias,
			DisableNotification: true,
			ReplyParameters: &models.ReplyParameters{
				MessageID: md.msg.ID,
				ChatID:    md.msg.Chat.ID,
			},
		}

		_, err = md.b.SendMediaGroup(md.ctx, params)
		if err != nil {
			fmt.Printf("SendMediaGroup: %+v\n", err)
		}
	}
}

// consumer receives data from the channel
func consumer(ch chan *MsgData) {
	for {
		md := <-ch
		if md == nil {
			fmt.Println("consumer nill msg")
			sendErr(md, errors.New("consumer nill msg "))
			continue
		}
		reply, err := dialogJob(md)
		if err != nil {
			sendErr(md, err)
			continue
		}

		textDraw := getCmd(reply, draw_cmd)

		htmlText := fmt.Sprintf("ok, %d in queue", len(imageChannel))
		if textDraw == "" {
			htmlText = tg_md2html.MD2HTML(reply)
		} else {
			if len(reply) < 100 || strings.ContainsAny(reply, "help") {
				htmlText = tg_md2html.MD2HTML(reply)
			}
		}

		replMsg, err := md.b.SendMessage(md.ctx, &bot.SendMessageParams{
			ChatID:              md.msg.Chat.ID,
			Text:                htmlText,
			ParseMode:           models.ParseModeHTML,
			DisableNotification: true,
			ReplyParameters: &models.ReplyParameters{
				MessageID: md.msg.ID,
				ChatID:    md.msg.Chat.ID,
			},
		})
		if err != nil {
			fmt.Println(err)
			sendErr(md, err)
			continue
		}

		if textDraw != "" {
			replMsg.Text = textDraw
			fmt.Println("draw", md.msg.From.ID, md.msg.From.Username, time.Now().Format(time.RFC822), truncateString(reply, 100)+"\n")

			uData := userDataGet(md.msg.From.ID)
			lastVisitDay := time.Unix(uData.LastVisit, 0).Format("20060102")
			if lastVisitDay == time.Now().Format("20060102") {
				uData.TodayDraw += 1
			} else {
				uData.TodayDraw = 1
			}

			if time.Since(time.Unix(uData.LastDraw, 0)) < time.Duration(30*time.Second) {
				sendErr(md, fmt.Errorf("Sorry, but i need slow down you a little..\nИзвините, Вы рисуете слишком быстро для меня"))
				continue
			}

			uData.LastDraw = time.Now().Unix()
			userDataSet(md.msg.From.ID, &uData)

			go producerImg(imageChannel, &MsgData{
				ctx: md.ctx,
				b:   md.b,
				msg: replMsg,
			})
		}
	}
}

func imageGet(prompt1, prompt2 string) ([][]byte, int, error) {
	client := &http.Client{Timeout: SDTimeout * time.Second}
	type Prompt struct {
		Prompt1 string `json:"prompt1"`
		Prompt2 string `json:"prompt2"`
	}

	jsonData, _ := json.Marshal(Prompt{
		Prompt1: prompt1,
		Prompt2: prompt2,
	})
	// создаем запрос
	req, err := http.NewRequest("POST", SDHost, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(err)
		return nil, 0, err
	}

	// отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("something wrong with image generation, try later. http code:%d", resp.StatusCode)
	}
	if resp.StatusCode == 204 {
		return nil, resp.StatusCode, nil
	}
	// читаем ответ
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, resp.StatusCode, err
	}

	var imageData []string
	err = json.Unmarshal(buf.Bytes(), &imageData)

	if err != nil {
		fmt.Println("json:" + err.Error())
		return nil, resp.StatusCode, err
	}
	images := [][]byte{}

	for _, image := range imageData {
		decodedImage, err := base64.StdEncoding.DecodeString(image)

		if err != nil {
			fmt.Println(err)
			return nil, resp.StatusCode, err
		}
		images = append(images, decodedImage)
	}
	return images, resp.StatusCode, nil
}

func simpleJob(text string) (string, error) {
	answer, err := completion.Generate(OllamaHost, llm.Query{
		Model:  OllamaModel,
		Prompt: text,
		Options: llm.Options{
			Temperature:   0.5,
			RepeatLastN:   768, //64
			RepeatPenalty: 5.0, //1.1
		},
	})
	return answer.Response, err
}

func sendErr(md *MsgData, err error) {
	md.b.SendMessage(md.ctx, &bot.SendMessageParams{
		ChatID:              md.msg.Chat.ID,
		Text:                "Error: " + err.Error() + "\nSupport chat: @charsaichat",
		DisableNotification: true,
		ReplyParameters: &models.ReplyParameters{
			MessageID: md.msg.ID,
			ChatID:    md.msg.Chat.ID,
		},
	})
	if md.msgStatus != nil {
		md.b.DeleteMessage(md.ctx, &bot.DeleteMessageParams{
			ChatID:    md.msgStatus.Chat.ID,
			MessageID: md.msgStatus.ID,
		})
	}
}
func hasNonEnglish(text string) bool {
	for _, r := range text {
		if !(unicode.Is(unicode.Latin, r) || unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsDigit(r) || unicode.IsGraphic(r)) {
			//fmt.Println("r", r, unicode.IsGraphic(r))
			return true
		}
	}
	return false
}

func truncateString(s string, total int) string {
	runes := []rune(s)
	if len(runes) <= total {
		return s
	}
	for i := total; i < len(runes); i++ {
		if strings.ContainsRune(" !?.;,:\n", runes[i]) {
			return string(runes[:i])
		}
	}
	return string(runes[:total])
}

func dialogJob(md *MsgData) (string, error) {
	from := md.msg.From.ID
	uData := userDataGet(from)
	defer userDataSet(from, &uData)
	fmt.Println(md.msg.From.ID, time.Now().Format(time.RFC822), truncateString(md.msg.Text, 100)+"\n")
	if uData.Id == 0 {
		//new user
		uData.Id = from
		uData.User = md.msg.From.FirstName
		if uData.User == "" {
			uData.User = md.msg.From.Username
		}
		uData.Lang = md.msg.From.LanguageCode
		uData.LastVisit = time.Now().Unix()
		backUp, err := os.ReadFile(fmt.Sprintf("data/%d.json", uData.Id))
		if err == nil {
			//has backup
			u := UserData{}
			err = json.Unmarshal(backUp, &u)
			if err == nil {
				//unmarshal
				uData.Chars = u.Chars
				if u.Lang != "" {
					uData.Lang = u.Lang
				}
			}
		}
		if uData.Chars == nil {
			uData.Chars = make(map[string]CharData)
		}
		char, err := parseChar(defChar, uData.User, uData.Lang)
		if err != nil {
			return "", err
		}
		uData.Char = char
		uData.Chars[char.Name] = char
		uData.Conversations = append(uData.Conversations, llm.Message{Role: "system", Content: uData.Char.Char})

		saveUData(uData)
	}
	uData.LastVisit = time.Now().Unix()

	if strings.HasPrefix(strings.ToLower(md.msg.Text), "newchar ") {
		char, err := parseChar(md.msg.Text, uData.User, uData.Lang)
		if err != nil {
			return "", err
		}
		uData.Char = char
		uData.Conversations[0] = llm.Message{Role: "system", Content: uData.Char.Char}
		uData.Conversations = uData.Conversations[:1]
		if uData.Chars == nil {
			uData.Chars = make(map[string]CharData)
		}
		uData.Chars[char.Name] = char
		saveUData(uData)
		return "New char:" + uData.Char.Name + "\n\nTemplate:\n```" + uData.Char.Template + "```", nil
	}
	charNames := make([]string, 0)
	if uData.Chars != nil {
		for name := range uData.Chars {
			charNames = append(charNames, name)
		}
	}
	if strings.HasPrefix(strings.ToLower(md.msg.Text), "chars") {
		listChars := strings.Join(charNames, "\n")
		return "list char:\n\n" + listChars + "\n\nUse:'char name' for switch. Default char: waifu", nil
	}
	if strings.HasPrefix(strings.ToLower(md.msg.Text), "char ") {
		spl := strings.Split(strings.ToLower(md.msg.Text), " ")
		person := strings.TrimSpace(spl[1])
		for _, name := range charNames {
			if name == person {
				char := uData.Chars[name]
				uData.Conversations[0] = llm.Message{Role: "system", Content: char.Char}
				uData.Conversations = uData.Conversations[:1]
				return "switched on character:" + person + "\nHistory cleaned", nil
			}
		}
		return fmt.Sprintf("charcter '%+v' not found\n", person), nil
	}
	if strings.HasPrefix(strings.ToLower(md.msg.Text), "delchar ") {
		spl := strings.Split(strings.ToLower(md.msg.Text), " ")
		person := strings.TrimSpace(spl[1])
		for _, name := range charNames {
			if name == person {
				delete(uData.Chars, name)
				saveUData(uData)
				return "deleted character:" + person, nil
			}
		}
		return "character with name:'" + person + "' not found", nil
	}

	if strings.HasPrefix(strings.ToLower(md.msg.Text), "/start") {
		return help_short, nil
	}

	if strings.HasPrefix(strings.ToLower(md.msg.Text), "help") ||
		strings.HasPrefix(strings.ToLower(md.msg.Text), "/help") {
		return help, nil
	}

	if strings.HasPrefix(strings.ToLower(md.msg.Text), "lang ") {
		spl := strings.Split(strings.ToLower(md.msg.Text), " ")
		lang := strings.TrimSpace(spl[1])
		uData.Lang = lang
		saveUData(uData)
		return "new language:" + lang, nil
	}

	for _, command := range draw_cmd {
		if strings.Contains(strings.ToLower(md.msg.Text), command) {
			return md.msg.Text, nil
		}
	}

	if len(uData.Conversations) >= 9 {
		uData.Conversations = append(uData.Conversations[:1], uData.Conversations[len(uData.Conversations)-2:]...)
	}

	uData.Conversations = append(uData.Conversations, llm.Message{Role: "user", Content: md.msg.Text})

	options := llm.Options{
		Temperature:   0.5, //0.8
		RepeatLastN:   768, //64
		RepeatPenalty: 5.0, //1.1
		//	NumPredict:    -2,          //128
		//	TopK:          100,         //40
		//	TopP:          0.95,        //0.9
	}

	answer, err := completion.ChatStream(OllamaHost,
		llm.Query{
			Model:    OllamaModel,
			Messages: uData.Conversations,
			Options:  options,
		},
		func(answer llm.Answer) error {
			return nil
		},
	)
	uData.Conversations = append(uData.Conversations, llm.Message{Role: "assistant", Content: answer.Message.Content})
	return answer.Message.Content, err
}

func getCmd(text string, cmd []string) string {
	draw := ""
	fields := strings.Fields(text)
	text = strings.Join(fields, " ")

	pos := -1
	for _, command := range cmd {
		pos = strings.Index(strings.ToLower(text), command)
		if pos >= 0 {
			draw = text[pos+len(command):]
			break
		}
	}
	//pos := strings.Index(strings.ToLower(text), cmd)
	if pos == -1 {
		return ""
	}
	draw = strings.TrimPrefix(draw, ":")
	draw = strings.TrimSpace(draw)
	return draw
}

func parseChar(txt, user, lang string) (CharData, error) {
	txt = strings.TrimSpace(txt)
	split := strings.Split(txt, "\n")[0]
	split = strings.Split(split, ".")[0]
	charName := strings.ToLower(split)
	charName = strings.Replace(charName, "newchar", "", -1)
	charName = strings.TrimSpace(charName)
	words := strings.Split(charName, " ")
	if len(words) > 1 {
		charName = words[0]
	}
	charName = strings.ReplaceAll(charName, " ", "_")
	charName = strings.ToLower(charName)
	char := CharData{
		Name:     charName,
		Template: txt,
	}

	txt = strings.ReplaceAll(txt, "newchar ", "You chat with {{user}} on language:{{lang}}. Your name: ")
	txt = strings.ReplaceAll(txt, "{{skillprompt}}", skillprompt)
	txt = strings.ReplaceAll(txt, "{{skilllang}}", skilllang)
	txt = strings.ReplaceAll(txt, "{{user}}", user)
	txt = strings.ReplaceAll(txt, "{{char}}", charName)
	txt = strings.ReplaceAll(txt, "{{lang}}", lang)
	txt = strings.ReplaceAll(txt, "on language:ru", "на русском языке")
	txt = strings.ReplaceAll(txt, "on language:en", "on English")
	txt = strings.ReplaceAll(txt, "on language:newlang", "on English")
	char.Char = txt
	return char, nil
}

func userDataGet(id int64) UserData {
	uData.RWMutex.RLock()
	defer uData.RWMutex.RUnlock()
	return uData.data[id]
}

func userDataSet(id int64, userData *UserData) {
	uData.RWMutex.Lock()
	defer uData.RWMutex.Unlock()
	uData.data[id] = *userData
}

func userDataData() map[int64]UserData {
	uData.RWMutex.RLock()
	defer uData.RWMutex.RUnlock()
	return uData.data
}
