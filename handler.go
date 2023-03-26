package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	dumper "wechatgpt/dumper"
	"wechatgpt/helper"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/eatmoreapple/openwechat"
	openai "github.com/sashabaranov/go-openai"
)

const FileHelper = "filehelper"
const Oops = "oops, something went wrong"
const EnableMarkdown2Image = false

var History = cmap.New[*[]*openai.ChatCompletionMessage]()
var QuotePattern = regexp.MustCompile(`「(\w+)：(\w+)」\n- - - - - - - - - - - - - - -\n(.+)`)

func HandleMessageText(client *openai.Client, msg *openwechat.Message) error {
	content := strings.TrimSpace(msg.Content)
	if len(content) == 0 {
		return nil
	}
	sender, _ := msg.Sender()
	receiver, _ := msg.Receiver()

	subhistory, ok := History.Get(sender.ID())
	if !ok {
		subhistory = &[]*openai.ChatCompletionMessage{}
		History.Set(sender.ID(), subhistory)
	}

	requirements := []string{
		"You are ChatGPT, a large language model trained by OpenAI. Answer as concisely as possible.\nKnowledge cutoff: 2021-09-01\nCurrent date: 2023-03-02",
		"Please use chinese.",
	}

	if len(*subhistory) == 0 {
		*subhistory = append(*subhistory, lo.Map(requirements, func(requirement string, _ int) *openai.ChatCompletionMessage {
			return &openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: requirement,
			}
		})...)
	}

	*subhistory = append(*subhistory, &openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: content,
	})

	// models, err := helper.Retry(3, func() (response openai.ModelsList, err error) {
	// 	return client.ListModels(
	// 		context.Background(),
	// 	)
	// })
	// fmt.Printf("Inspect(models): %v\n", helper.ToJson(models))
	// if err != nil {
	// 	return err
	// }

	var model string = openai.GPT3Dot5Turbo0301

	if false {
		tunes, err := helper.Retry(3, func() (response openai.FineTuneList, err error) {
			return client.ListFineTunes(
				context.Background(),
			)
		})
		if err != nil {
			return err
		}
		availableModels := lo.Filter(tunes.Data, func(tune openai.FineTune, _ int) bool {
			return strings.Contains(tune.FineTunedModel, ":ft-personal:"+sender.ID()) && tune.Status == "succeeded"
		})
		if len(availableModels) > 0 {
			model = availableModels[0].FineTunedModel
		}
	}

	resp, err := helper.Retry(3, func() (response openai.ChatCompletionResponse, err error) {
		msgs := helper.Last(*subhistory, 5)
		return client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: model,
				Messages: lo.Map(msgs, func(msg *openai.ChatCompletionMessage, _ int) openai.ChatCompletionMessage {
					return *msg
				}),
				User: sender.ID(),
			},
		)
	})
	if err != nil {
		return err
	}

	reply := strings.TrimSpace(resp.Choices[0].Message.Content)

	*subhistory = append(*subhistory, &openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: reply,
	})

	if EnableMarkdown2Image && strings.Contains(reply, "```") {
		png, err := helper.ConvertMarkdownToPNG(reply)
		if err != nil {
			return err
		}
		defer os.Remove(png.Name())
		_, err = msg.ReplyImage(png)
		if err != nil {
			return err
		}
	} else {
		if receiver.UserName == FileHelper {
			_, err = sender.Self().FileHelper().SendText(reply)
			if err != nil {
				return err
			}
		} else {
			_, err = msg.ReplyText(reply)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func HandleMessageFineTune(client *openai.Client, msg *openwechat.Message) error {
	sender, _ := msg.Sender()
	receiver, _ := msg.Receiver()

	file, err := dumper.GenerateFuneTuningDataset("keyword")
	if err != nil {
		return err
	}
	defer file.Close()

	resp1, err := helper.Retry(3, func() (response openai.File, err error) {
		return client.CreateFile(
			context.Background(),
			openai.FileRequest{
				FileName: fmt.Sprintf("%s_ft_ds.json", sender.ID()),
				FilePath: file.Name(),
				Purpose:  "fine-tune",
			},
		)
	})
	if err != nil {
		return err
	}
	reply := fmt.Sprintf(`
上传成功
Name: %s
ID: %s`, resp1.FileName, resp1.ID)

	if receiver.UserName == FileHelper {
		sender.Self().FileHelper().SendText(strings.TrimSpace(reply))
	} else {
		msg.ReplyText(strings.TrimSpace(reply))
	}

	resp2, err := helper.Retry(3, func() (response openai.FineTune, err error) {
		return client.CreateFineTune(
			context.Background(),
			openai.FineTuneRequest{
				TrainingFile: resp1.ID,
				Suffix:       sender.ID(),
			},
		)
	})
	if err != nil {
		return err
	}

	events := lo.Map(resp2.FineTuneEventList, func(item openai.FineTuneEvent, _ int) string {
		return item.Message
	})

	reply = fmt.Sprintf(`
fine tuning is started
ID: %s
%s`, resp2.ID, strings.Join(events, "\n"))
	if receiver.UserName == FileHelper {
		sender.Self().FileHelper().SendText(strings.TrimSpace(reply))
	} else {
		msg.ReplyText(strings.TrimSpace(reply))
	}
	return nil
}

func HandleMessageListTune(client *openai.Client, msg *openwechat.Message) error {
	sender, _ := msg.Sender()
	receiver, _ := msg.Receiver()

	resp, err := helper.Retry(3, func() (response openai.FineTuneList, err error) {
		return client.ListFineTunes(context.Background())
	})
	if err != nil {
		return err
	}

	text := helper.ToJson(resp.Data)

	if receiver.UserName == FileHelper {
		sender.Self().FileHelper().SendText(text)
	} else {
		msg.ReplyText(text)
	}
	return nil
}

func HandleMessageVoice(client *openai.Client, msg *openwechat.Message) error {
	resp, err := msg.GetVoice()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	audio, err := ioutil.TempFile("", "prefix.*.mp3")
	if err != nil {
		return err
	}
	defer audio.Close()

	io.Copy(audio, resp.Body)

	resp1, err := helper.Retry(3, func() (response openai.AudioResponse, err error) {
		return client.CreateTranslation(
			context.Background(),
			openai.AudioRequest{
				Model:    openai.GPT3Dot5Turbo,
				FilePath: audio.Name(),
			},
		)
	})
	if err != nil {
		return err
	}

	sender, _ := msg.Sender()
	receiver, _ := msg.Receiver()

	if receiver.UserName == FileHelper {
		sender.Self().FileHelper().SendText(resp1.Text)
	} else {
		msg.ReplyText(resp1.Text)
	}

	return nil
}

func HandleMessageImage(client *openai.Client, msg *openwechat.Message) error {
	sender, _ := msg.Sender()
	receiver, _ := msg.Receiver()
	resp, err := helper.Retry(3, func() (response openai.ImageResponse, err error) {
		return client.CreateImage(context.Background(), openai.ImageRequest{
			Prompt:         strings.TrimSpace(msg.Content),
			N:              1,
			Size:           openai.CreateImageSize512x512,
			ResponseFormat: openai.CreateImageResponseFormatURL,
			User:           sender.ID(),
		})
	})
	if err != nil {
		return err
	}
	for _, image := range resp.Data {
		go func(image openai.ImageResponseDataInner) {
			if receiver.UserName == FileHelper {
				func() {
					resp, err := helper.DownloadRemote(image.URL)
					if err != nil {
						return
					}
					defer resp.Close()
					_, err = sender.Self().FileHelper().SendImage(resp)
					if err != nil {
						return
					}
				}()
			} else {
				func() {
					resp, err := helper.DownloadRemote(image.URL)
					if err != nil {
						return
					}
					defer resp.Close()
					_, err = msg.ReplyImage(resp)
					if err != nil {
						return
					}
				}()
			}
		}(image)
	}
	return nil
}

func ReplyMessage(msg *openwechat.Message, client *openai.Client, db *leveldb.DB) error {
	if msg.MsgType == 51 {
		return nil
	}

	if msg.IsSendByGroup() {
		return nil
	}

	sender, err := msg.Sender()
	if err != nil {
		return err
	}

	receiver, err := msg.Receiver()
	if err != nil {
		return err
	}

	if sender.IsSelf() && receiver.NickName != "" {
		return err
	}

	flag := fmt.Sprintf("%s_freeze", sender.ID())

	switch strings.ToLower(strings.TrimSpace(msg.Content)) {
	case "begin", "start", "resume", "continue", "继续", "开始":
		err = db.Delete([]byte(flag), nil)
		if err != nil {
			return err
		}
		_, err := msg.ReplyText("received, type stop to terminate")
		return err
	case "end", "stop", "break", "暂停", "停止", "停", "停停", "停停停", "停停停停":
		err = db.Put([]byte(flag), []byte("true"), nil)
		if err != nil {
			return err
		}
		_, err := msg.ReplyText("received, type start to resume")
		return err
	case "create fine tune":
		return HandleMessageFineTune(client, msg)
	case "list fine tune":
		return HandleMessageListTune(client, msg)
	default:
		if QuotePattern.MatchString(msg.Content) {
			matches := QuotePattern.FindStringSubmatch(msg.Content)
			command := strings.TrimSpace(matches[3])
			if command == "repeat" || command == "重复" || command == "ref" || command == "" {
				msg.Content = matches[2]
			}
		}
	}

	_, err = db.Get([]byte(flag), nil)
	if err == nil {
		// it represents that the flag can be found
		return err
	}

	switch msg.MsgType {
	case openwechat.MsgTypeText:
		if strings.Contains(msg.Content, "图") || strings.Contains(msg.Content, "照片") {
			return HandleMessageImage(client, msg)
		} else {
			return HandleMessageText(client, msg)
		}
	case openwechat.MsgTypeImage:
		_, err := msg.ReplyText(openwechat.Emoji.Doge)
		return err
	case openwechat.MsgTypeVoice:
		return HandleMessageVoice(client, msg)
	case openwechat.MsgTypeVerify:
	case openwechat.MsgTypePossibleFriend:
	case openwechat.MsgTypeShareCard:
		card, _ := msg.Card()
		_, err := msg.ReplyText(card.UserName)
		if err != nil {
			return err
		}
	case openwechat.MsgTypeVideo:
		_, err := msg.ReplyText(openwechat.Emoji.Doge)
		if err != nil {
			return err
		}
	case openwechat.MsgTypeEmoticon:
		_, err := msg.ReplyText(openwechat.Emoji.Doge)
		if err != nil {
			return err
		}
	case openwechat.MsgTypeLocation:
	case openwechat.MsgTypeApp:
	case openwechat.MsgTypeVoip:
	case openwechat.MsgTypeVoipNotify:
	case openwechat.MsgTypeVoipInvite:
	case openwechat.MsgTypeMicroVideo:
	case openwechat.MsgTypeSys:
	case openwechat.MsgTypeRecalled:
		_, err := msg.ReplyText("你干嘛要撤回")
		if err != nil {
			return err
		}
	default:
	}
	return nil
}
