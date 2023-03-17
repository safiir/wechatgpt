package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/samber/lo"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/eatmoreapple/openwechat"
	openai "github.com/sashabaranov/go-openai"
)

func Makesure[T any](logic func() (T, error)) (T, error) {
	for {
		t, err := logic()
		if err != nil {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		return t, nil
	}
}

func HandleMsgText(client *openai.Client, msg *openwechat.Message) {
	content := strings.TrimSpace(msg.Content)
	if len(content) == 0 {
		return
	}
	resp, err := client.CreateChatCompletion(
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
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		msg.ReplyText("oops, something went wrong")
		return
	}
	reply := strings.TrimSpace(resp.Choices[0].Message.Content)

	Makesure(func() (*openwechat.SentMessage, error) {
		return msg.ReplyText(reply)
	})
}

func Reply(msg *openwechat.Message, client *openai.Client, db *leveldb.DB) {
	sender, _ := msg.Sender()
	if sender.IsSelf() {
		return
	}
	from := sender.UserName
	flag := fmt.Sprintf("%s_freeze", from)
	_, err := db.Get([]byte(flag), nil)
	if err == nil {
		return
	}
	switch msg.MsgType {
	case openwechat.MsgTypeText:
		switch msg.Content {
		case "start":
			err = db.Delete([]byte(flag), nil)
			if err != nil {
				return
			}
			Makesure(func() (*openwechat.SentMessage, error) {
				return msg.ReplyText("received")
			})
		case "end":
			err = db.Put([]byte(flag), []byte("true"), nil)
			if err != nil {
				return
			}
			Makesure(func() (*openwechat.SentMessage, error) {
				return msg.ReplyText("received")
			})
		default:
			HandleMsgText(client, msg)
		}
	case openwechat.MsgTypeImage:
	case openwechat.MsgTypeVoice:
	case openwechat.MsgTypeVerify:
	case openwechat.MsgTypePossibleFriend:
	case openwechat.MsgTypeShareCard:
	case openwechat.MsgTypeVideo:
	case openwechat.MsgTypeEmoticon:
	case openwechat.MsgTypeLocation:
	case openwechat.MsgTypeApp:
	case openwechat.MsgTypeVoip:
	case openwechat.MsgTypeVoipNotify:
	case openwechat.MsgTypeVoipInvite:
	case openwechat.MsgTypeMicroVideo:
	case openwechat.MsgTypeSys:
	case openwechat.MsgTypeRecalled:
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
		panic(err)
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

	bot := openwechat.DefaultBot(openwechat.Desktop) // 桌面模式
	bot.SyncCheckCallback = func(resp openwechat.SyncCheckResponse) {}

	db, err := leveldb.OpenFile("db", nil)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	defer db.Close()

	// 注册消息处理函数
	bot.MessageHandler = func(msg *openwechat.Message) {
		go Reply(msg, client, db)
	}
	// 注册登陆二维码回调
	bot.UUIDCallback = openwechat.PrintlnQrcodeUrl

	// 登陆
	if err := bot.Login(); err != nil {
		fmt.Println(err)
		return
	}

	// 获取登陆的用户
	self, err := bot.GetCurrentUser()
	if err != nil {
		fmt.Println(err)
		return
	}

	// 获取所有的好友
	friends, err := self.Friends()
	if err != nil {
		fmt.Println(err)
		return
	}
	friendNames := lo.Map(friends, func(friend *openwechat.Friend, _ int) string {
		return friend.NickName
	})
	fmt.Printf("%v\n", friendNames)

	// 获取所有的群组
	groups, err := self.Groups()
	if err != nil {
		fmt.Println(err)
		return
	}
	groupNames := lo.Map(groups, func(group *openwechat.Group, _ int) string {
		return group.NickName
	})
	fmt.Printf("%v\n", groupNames)

	// 阻塞主goroutine, 直到发生异常或者用户主动退出
	bot.Block()
}
