package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/samber/lo"

	"github.com/eatmoreapple/openwechat"
	openai "github.com/sashabaranov/go-openai"
)

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

	// 注册消息处理函数
	bot.MessageHandler = func(msg *openwechat.Message) {
		if msg.IsText() {
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
			msg.ReplyText(reply)
		}
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
