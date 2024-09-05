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

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type MsgData struct {
	ctx       context.Context
	b         *bot.Bot
	msg       *models.Message
	msgStatus *models.Message
}

const (
	SDHost    = "https://wqzhut3bfr6t3v-8882.proxy.runpod.net/"
	SDTimeout = 60
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
	fmt.Println("Produced:", md)
}

// consumer receives data from the channel
func consumer(ch chan *MsgData) {
	for {
		md := <-ch
		fmt.Println("Consumed:", md)

		imgData, err := imageGet(md.msg.Text)
		md.b.DeleteMessage(md.ctx, &bot.DeleteMessageParams{
			ChatID:    md.msgStatus.Chat.ID,
			MessageID: md.msgStatus.ID,
		})
		if err != nil {
			md.b.SendMessage(md.ctx, &bot.SendMessageParams{
				ChatID: md.msg.Chat.ID,
				Text:   "Ой, ошибка: " + err.Error(),
				ReplyParameters: &models.ReplyParameters{
					MessageID: md.msg.ID,
					ChatID:    md.msg.Chat.ID,
				},
			})
		} else {
			//TODO:  https://github.com/go-telegram/bot/blob/main/examples/send_media_group/main.go
			medias := make([]models.InputMedia, 0, 2)
			for i, v := range imgData {
				medias = append(medias, &models.InputMediaPhoto{
					Media:           fmt.Sprintf("attach://%d_%d.png", md.msg.ID, i),
					Caption:         md.msg.Text,
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

			md.b.SendMediaGroup(md.ctx, params)
		}
	}
}

func imageGet(prompt string) ([][]byte, error) {
	client := &http.Client{Timeout: SDTimeout * time.Second}

	// создаем запрос
	req, err := http.NewRequest("POST", SDHost, bytes.NewBufferString(prompt))
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
