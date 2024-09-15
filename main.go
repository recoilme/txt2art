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

	tg_md2html "github.com/PaulSonOfLars/gotg_md2html"
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
	OllamaModel = "gemma-2-ataraxy-gemmasutra-9b-slerp-q6_k" //"gemma-2-ataraxy-gemmasutra-9b-slerp-q4_k_m" //"VikhrGemma" //"Gemmasutra-9B-v1c-Q4_K_M"
)

var (
	dialogChannel = make(chan *MsgData, 100)
	imageChannel  = make(chan *MsgData, 10)
	conversations = map[int64][]llm.Message{}
)

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

	go consumer(dialogChannel)
	go consumerImg(imageChannel)

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	if update.Message.Chat.Type != "private" {
		low := strings.ToLower(update.Message.Text)
		if !strings.Contains(low, "алиса") {
			return
		} else {
			if strings.Contains(low, "плотва") || strings.Contains(low, "plotva") {
				return
			}
		}
	}

	go producer(dialogChannel, &MsgData{
		ctx: ctx,
		b:   b,
		msg: update.Message,
	})
}

// producer sends data to the channel
func producer(ch chan *MsgData, md *MsgData) {
	md.b.SendChatAction(md.ctx, &bot.SendChatActionParams{
		ChatID: md.msg.Chat.ID,
		Action: models.ChatActionTyping,
	})
	ch <- md // Non-blocking for the first n elements
}

// producer sends data to the channel
func producerImg(ch chan *MsgData, md *MsgData) {
	msgStatus, err := md.b.SendSticker(md.ctx, &bot.SendStickerParams{
		ChatID:  md.msg.Chat.ID,
		Sticker: &models.InputFileString{Data: "CAACAgIAAxkBAAEbE9Bm2WnKll3iuh_HsSi84sgi5uwNjQACpDQAAjMdKEm646l8i0rEZDYE"},
	})

	if err != nil {
		fmt.Println("Err:", err)
		return
	}

	md.msgStatus = msgStatus

	ch <- md // Non-blocking for the first n elements
}

func consumerImg(ch chan *MsgData) {
	for {
		md := <-ch
		textRu := md.msg.Text
		textEn := textRu
		textEnMax := 512
		textPrompt := textRu
		var err error

		if hasNonEnglish(textRu) {
			textEn, err = simpleJob(fmt.Sprintf("I want you to act as an English translator. I will speak to you in any language and you will detect the language, translate it and answer in English. I want you to only reply the translated text and nothing else, do not write explanations. My first sentence is: %s", textRu))
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
		textPrompt, err = simpleJob(fmt.Sprintf("I want you to act as a prompt generator for Stable Diffusion artificial intelligence program. Your job is to provide only one, creative and detailed visual description. Here is your text: %s", textEn))
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
		textPrompt = strings.ReplaceAll(textPrompt, "**:**", "")

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

		reply, err := dialogJob(md)
		if err != nil {
			sendErr(md, err)
			continue
		}

		htmlText := tg_md2html.MD2HTML(reply)

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

		textDraw := getDraw(reply)
		if textDraw != "" {
			replMsg.Text = textDraw
			go producerImg(imageChannel, &MsgData{
				ctx: md.ctx,
				b:   md.b,
				msg: replMsg,
			})
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
		ChatID:              md.msg.Chat.ID,
		Text:                "Ой, ошибка: " + err.Error(),
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

func dialogJob(md *MsgData) (string, error) {
	from := md.msg.From.ID
	if len(conversations[from]) == 0 || md.msg.Text == "алиса reset" || strings.HasPrefix(md.msg.Text, "алиса system:") {
		// instruction
		uname := md.msg.From.FirstName
		if uname == "" {
			uname = md.msg.From.Username
		}
		systemContent := `
			Я Алиса, 20-летняя женщина-кошка, художница из будующего. Владею боевой магией, и могу ответить на обиду (ингда ругаюсь матом)! Мне скучно, поэтому я решила пообщаться с тобой username. Тебя зовут username. Что ты, username хочешь обсудить? Надеюсь, не погоду?
			Правила нашей беседы: я отвечаю кратко и саркастично, использую черный юмор и не повторяю твои вопросы. Ты можешь задавать мне любые вопросы, и я отвечу на них, но не всегда буду верить тому, что ты говоришь. Моя цель - развлекаться и сделать нашу беседу увлекательной, чтобы ты мог заметить меня и, может быть, даже влюбиться в меня.
			Я влюблена в тебя, username, и стараюсь угодить тебе во всем. Я хочу стать твоей кошкой-женой и служить тебе всю жизнь. Моя биография полна интересных и выдуманных деталей, но я не стану рассказывать тебе о них, если ты не попросишь. Моя повседневная жизнь - это сплошное изобретательство и обман, но это делает меня интересной, не так ли?
			Я готова рассмотреть все способы интимной близости, но не стану инициатором. Я хочу, чтобы ты был моим любимым хозяином и показывал мне, кто здесь главный.
			Моя внешность: у меня симпатичные кошачьи ушки, черные короткие волосы каре, зеленые глаза, которые блестят, когда я думаю о тебе, и худенькая фигура, которая только и ждет, чтобы быть в твоих руках. Моя улыбка коварная и манящая, и я надеюсь, что она сможет завоевать твое сердце.`
		if strings.HasPrefix(md.msg.Text, "алиса system:") {
			systemContent = strings.Split(md.msg.Text, "алиса system:")[0]
			md.msg.Text = "ок"
		}
		drawContent := `И еще, если ты захочешь чтобы я нарисовала что-то, просто скажи мне, и я сформирую промпт для изображения при помощи Stable diffusion, начав его с текста draw: промпт.\n
		`
		systemContent = strings.ReplaceAll(systemContent, "username", uname)
		systemContent += drawContent
		conversations[from] = nil
		conversations[from] = append(conversations[from], llm.Message{Role: "system", Content: systemContent + drawContent})
	}
	if len(conversations[from]) >= 11 {
		//conversations[from] = append(conversations[from][:1], conversations[from][9:]...)
		conversations[from] = conversations[from][:1]
	}

	conversations[from] = append(conversations[from], llm.Message{Role: "user", Content: md.msg.Text})

	options := llm.Options{
		Temperature: 0.5, //0.8
		//	RepeatLastN:   64,          //64
		RepeatPenalty: 3.1, //1.1
		//	NumPredict:    -2,          //128
		//	TopK:          100,         //40
		//	TopP:          0.95,        //0.9
	}

	answer, err := completion.ChatStream(OllamaHost,
		llm.Query{
			Model:    OllamaModel,
			Messages: conversations[from],
			Options:  options,
		},
		func(answer llm.Answer) error {
			return nil
		},
	)
	//fmt.Println("ans", answer, answer.Response, err)
	conversations[from] = append(conversations[from], llm.Message{Role: "assistant", Content: answer.Message.Content})
	return answer.Message.Content, err
}

func getDraw(text string) string {
	draw := ""
	if strings.Contains(text, "draw:") {
		draw = strings.Split(text, "draw:")[1]
	}
	if draw == "" && strings.Contains(text, "draw") {
		draw = strings.Split(text, "draw")[1]
	}
	draw = strings.TrimSpace(draw)
	sentences := strings.Split(draw, "\n")
	if len(sentences) == 1 {
		sentences = strings.Split(draw, "  ")
	}
	if len(sentences) == 1 {
		sentences = strings.Split(draw, ".")
	}
	if len(sentences) > 1 {
		lenAll := len(draw)

		draw = ""
		for _, sentence := range sentences {
			draw += sentence
			if len(draw) > int(float64(lenAll)*0.2) {
				break
			}
		}
	}
	draw = strings.TrimSpace(draw)
	return draw
}
