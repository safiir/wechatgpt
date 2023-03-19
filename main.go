package main

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/joho/godotenv"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/eatmoreapple/openwechat"
	openai "github.com/sashabaranov/go-openai"
)

func Once[T any](fn func() (T, error)) (T, error) {
	for {
		t, err := fn()
		if err != nil {
			time.Sleep(time.Millisecond * 500)
			continue
		}
		return t, nil
	}
}

func Retry[T any](n int, fn func() (T, error)) (T, error) {
	var t T
	var err error
	for i := 0; i < n; i++ {
		t, err = fn()
		if err != nil {
			time.Sleep(time.Millisecond * 500)
			continue
		}
		return t, nil
	}
	return t, err
}

func ConvertMarkdownToPNG(content string) (*os.File, error) {
	md := []byte(content)
	// always normalize newlines, this library only supports Unix LF newlines
	md = markdown.NormalizeNewlines(md)

	// create markdown parser
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	// parse markdown into AST tree
	doc := p.Parse(md)

	// create HTML renderer
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	html := markdown.Render(doc, renderer)

	file, err := ioutil.TempFile("", "prefix.*.html")
	if err != nil {
		return nil, err
	}
	ioutil.WriteFile(file.Name(), []byte(html), fs.ModeTemporary)

	png, err := ioutil.TempFile("", "prefix.*.png")
	if err != nil {
		return nil, err
	}

	page := rod.New().MustConnect().MustPage(fmt.Sprintf("file:///%s", file.Name()))
	page.MustWaitLoad().MustScreenshot(png.Name())

	return png, nil
}

func HandleMsgText(client *openai.Client, msg *openwechat.Message) {
	content := strings.TrimSpace(msg.Content)
	if len(content) == 0 {
		return
	}
	resp, err := Retry(3, func() (response openai.ChatCompletionResponse, err error) {
		return client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: openai.GPT3Dot5Turbo,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleUser,
						Content: content,
					},
				},
			},
		)
	})
	if err != nil {
		fmt.Println(err)
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText("oops, something went wrong")
		})
		return
	}
	reply := strings.TrimSpace(resp.Choices[0].Message.Content)

	if false && strings.Contains(reply, "```") {
		png, err := ConvertMarkdownToPNG(reply)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer os.Remove(png.Name())
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyImage(png)
		})
	} else {
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText(reply)
		})
	}
}

func Reply(msg *openwechat.Message, client *openai.Client, db *leveldb.DB) {
	if msg.MsgType == 51 {
		return
	}

	sender, err := msg.Sender()
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	receiver, err := msg.Receiver()
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	fmt.Printf("%v\n", []string{sender.NickName, receiver.NickName})

	if sender.IsSelf() && !receiver.IsSelf() {
		return
	}

	flag := fmt.Sprintf("%s_freeze", sender.ID())

	switch strings.ToLower(strings.TrimSpace(msg.Content)) {
	case "begin", "start", "resume", "continue", "继续":
		err = db.Delete([]byte(flag), nil)
		if err != nil {
			return
		}
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText("received, type stop to terminate")
		})
	case "end", "stop", "break", "暂停", "停止", "停", "停停", "停停停", "停停停停":
		err = db.Put([]byte(flag), []byte("true"), nil)
		if err != nil {
			return
		}
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText("received, type start to resume")
		})
	default:
	}

	_, err = db.Get([]byte(flag), nil)

	if err == nil {
		return
	}
	switch msg.MsgType {
	case openwechat.MsgTypeText:
		HandleMsgText(client, msg)
	case openwechat.MsgTypeImage:
		if false {
			Retry(3, func() (*openwechat.SentMessage, error) {
				file, err := os.Open("pic.jpeg")
				if err != nil {
					return nil, err
				}
				return msg.ReplyImage(file)
			})
		}
	case openwechat.MsgTypeVoice:
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText("请打字")
		})
	case openwechat.MsgTypeVerify:
	case openwechat.MsgTypePossibleFriend:
	case openwechat.MsgTypeShareCard:
	case openwechat.MsgTypeVideo:
	case openwechat.MsgTypeEmoticon:
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText(openwechat.Emoji.Doge)
		})
	case openwechat.MsgTypeLocation:
	case openwechat.MsgTypeApp:
	case openwechat.MsgTypeVoip:
	case openwechat.MsgTypeVoipNotify:
	case openwechat.MsgTypeVoipInvite:
	case openwechat.MsgTypeMicroVideo:
	case openwechat.MsgTypeSys:
	case openwechat.MsgTypeRecalled:
		Retry(3, func() (*openwechat.SentMessage, error) {
			return msg.ReplyText("你干嘛要撤回")
		})
	default:
	}
}

func main() {
	_ = godotenv.Load()

	PROXY := os.Getenv("proxy")
	TOKEN := os.Getenv("token")

	if len(PROXY) == 0 {
		fmt.Println("Please provide proxy via env variable.")
		return
	}

	if len(TOKEN) == 0 {
		fmt.Println("Please provide token via env variable.")
		return
	}

	proxy, err := url.Parse(PROXY)
	if err != nil {
		log.Fatalln(err)
	}
	config := openai.DefaultConfig(TOKEN)
	config.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: func(r *http.Request) (*url.URL, error) {
				return proxy, nil
			},
		},
	}
	client := openai.NewClientWithConfig(config)

	bot := openwechat.DefaultBot(openwechat.Desktop)
	bot.SyncCheckCallback = func(resp openwechat.SyncCheckResponse) {}

	db, err := leveldb.OpenFile("db", nil)
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	bot.MessageHandler = func(msg *openwechat.Message) {
		go Reply(msg, client, db)
	}

	bot.UUIDCallback = openwechat.PrintlnQrcodeUrl

	if err := bot.Login(); err != nil {
		log.Fatalln(err)
	}

	self, err := bot.GetCurrentUser()
	if err != nil {
		log.Fatalln(err)
	}

	friends, err := self.Friends()
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		for _, friend := range friends {
			if friend.NickName == "xxxxxxxxxx" {
				Retry(3, func() (*openwechat.SentMessage, error) {
					return self.SendTextToFriend(friend, "hello")
				})
			}
		}
	}()

	groups, err := self.Groups()
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for _, group := range groups {
			if group.NickName == "xxxxxxxxxx" {
				Retry(3, func() (*openwechat.SentMessage, error) {
					return self.SendTextToGroup(group, "hello")
				})
			}
		}
	}()

	bot.Block()
}
