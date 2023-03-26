package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/eatmoreapple/openwechat"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	godotenv.Load()

	PROXY := os.Getenv("proxy")
	TOKEN := os.Getenv("token")

	if len(PROXY) == 0 {
		log.Fatal("Please provide proxy via env variable.")
	}

	if len(TOKEN) == 0 {
		log.Fatal("Please provide token via env variable.")
	}

	proxy, err := url.Parse(PROXY)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}
	defer db.Close()

	bot.MessageHandler = func(msg *openwechat.Message) {
		go func(msg *openwechat.Message) {
			err := ReplyMessage(msg, client, db)
			if err != nil {
				fmt.Printf("err: %v\n", err)
				sender, _ := msg.Sender()
				receiver, _ := msg.Receiver()
				if receiver.UserName == FileHelper {
					_, err := sender.Self().FileHelper().SendText(Oops)
					if err != nil {
						fmt.Printf("err: %v\n", err)
					}
				} else {
					_, err := msg.ReplyText(Oops)
					if err != nil {
						fmt.Printf("err: %v\n", err)
					}
				}
			}
		}(msg)
	}

	bot.UUIDCallback = openwechat.PrintlnQrcodeUrl

	if err := bot.Login(); err != nil {
		log.Fatal(err)
	}

	bot.Block()
}
