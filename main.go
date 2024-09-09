package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/parakeet-nest/parakeet/completion"
	"github.com/parakeet-nest/parakeet/llm"
)

type MsgData struct {
	ctx       context.Context
	b         *bot.Bot
	msg       *models.Message
	msgStatus *models.Message
}

const (
	SDHost      = "https://wqzhut3bfr6t3v-8882.proxy.runpod.net/"
	SDTimeout   = 60
	OllamaHost  = "https://wqzhut3bfr6t3v-11434.proxy.runpod.net"
	OllamaModel = "VikhrGemma" //"Gemmasutra-9B-v1c-Q4_K_M"
)

var dataChannel = make(chan *MsgData, 100)

func main() {
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

	go consumer(dataChannel)

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	go producer(dataChannel, &MsgData{
		ctx: ctx,
		b:   b,
		msg: update.Message,
	})
}

// producer sends data to the channel
func producer(ch chan *MsgData, md *MsgData) {

	msgStatus, err := md.b.SendSticker(md.ctx, &bot.SendStickerParams{
		ChatID:  md.msg.Chat.ID,
		Sticker: &models.InputFileString{Data: "CAACAgIAAxkBAAEbE9Bm2WnKll3iuh_HsSi84sgi5uwNjQACpDQAAjMdKEm646l8i0rEZDYE"},
	})

	if err != nil {
		fmt.Println("Err:", err)
		return
	}
	md.msgStatus = msgStatus
	ch <- md // Non-blocking for the first 2 elements
	//fmt.Println("Produced:", md)
}

// consumer receives data from the channel
func consumer(ch chan *MsgData) {
	for {
		md := <-ch
		textRu := md.msg.Text
		textEn := textRu
		textEnMax := 512
		textPrompt := textRu
		var err error
		if hasNonEnglish(textRu) {
			textEn, err = simpleJob(fmt.Sprintf("I want you to act as an English translator. I will speak to you in any language and you will detect the language, translate it and answer in English. I want you to only reply the translated text and nothing else, do not write explanations. My first sentence is :%s", textRu))
			if err != nil {
				sendErr(md, err)
				continue
			}
		}
		if len([]rune(textEn)) > textEnMax {
			textEn, err = simpleJob(fmt.Sprintf("Skip the introduction and summarize this text in short description:%s", textEn))
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
		textPrompt, err = simpleJob(fmt.Sprintf("I want you to act as a prompt generator for Stable Diffusion artificial intelligence program. Your job is to provide only one, short and creative visual description. Here is your text: %s", textEn))
		if err != nil {
			sendErr(md, err)
			continue
		}
		if len([]rune(textPrompt)) > (1000 - textEnMax) {
			textPrompt, err = simpleJob(fmt.Sprintf("Skip the introduction and summarize this text in short description:%s", textPrompt))
			if err != nil {
				sendErr(md, err)
				continue
			}
		}
		textPrompt = strings.ReplaceAll(textPrompt, "**Visual Description:**", "")
		textPrompt = strings.ReplaceAll(textPrompt, "Summary", "")

		textPrompt = truncateString(textPrompt, (1000 - textEnMax))
		textEn = fmt.Sprintf("(%s)\n", textEn)
		imgData, err := imageGet(textEn, textPrompt)
		if err != nil {
			sendErr(md, err)
			continue
		}
		md.b.DeleteMessage(md.ctx, &bot.DeleteMessageParams{
			ChatID:    md.msgStatus.Chat.ID,
			MessageID: md.msgStatus.ID,
		})

		if paid {
			medias := make([]models.InputPaidMedia, 0, 4)
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
		medias := make([]models.InputMedia, 0, 4)
		for i, v := range imgData {
			caption := textEn + textPrompt
			caption = truncateString(caption, 876)
			medias = append(medias, &models.InputMediaPhoto{
				Media:           fmt.Sprintf("attach://%d_%d.png", md.msg.ID, i),
				Caption:         caption,
				MediaAttachment: bytes.NewReader(v),
			})

		}

		params := &bot.SendMediaGroupParams{
			ChatID: md.msg.Chat.ID,
			Media:  medias,
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

func imageGet(prompt1, prompt2 string) ([][]byte, error) {
	client := &http.Client{Timeout: SDTimeout * time.Second}
	type Prompt struct {
		Prompt1 string `json:"prompt1"`
		Prompt2 string `json:"prompt2"`
	}

	jsonData, _ := json.Marshal(Prompt{
		Prompt1: prompt1,
		Prompt2: prompt2,
	})

	//fmt.Println(string(jsonData))
	// создаем запрос
	req, err := http.NewRequest("POST", SDHost, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("service is down, http code:%d", resp.StatusCode)
	}
	// читаем ответ
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var imageData []string
	err = json.Unmarshal(buf.Bytes(), &imageData)

	if err != nil {
		fmt.Println("json:" + err.Error())
		return nil, err
	}
	images := [][]byte{}

	for _, image := range imageData {
		decodedImage, err := base64.StdEncoding.DecodeString(image)

		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		images = append(images, decodedImage)
	}
	return images, nil
}

func simpleJob(text string) (string, error) {
	answer, err := completion.Generate(OllamaHost, llm.Query{
		Model:  OllamaModel,
		Prompt: text,
		Options: llm.Options{
			Temperature: 0.5,
		},
	})
	return answer.Response, err
}

func sendErr(md *MsgData, err error) {
	md.b.SendMessage(md.ctx, &bot.SendMessageParams{
		ChatID: md.msg.Chat.ID,
		Text:   "Ой, ошибка: " + err.Error(),
		ReplyParameters: &models.ReplyParameters{
			MessageID: md.msg.ID,
			ChatID:    md.msg.Chat.ID,
		},
	})
	md.b.DeleteMessage(md.ctx, &bot.DeleteMessageParams{
		ChatID:    md.msgStatus.Chat.ID,
		MessageID: md.msgStatus.ID,
	})
}
func hasNonEnglish(text string) bool {
	for _, r := range text {
		if !(unicode.Is(unicode.Latin, r) || unicode.IsSpace(r) || unicode.IsPunct(r)) {
			//fmt.Println(string(r), r, unicode.Is(unicode.Latin, r))
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
