package wechat_dumper

type Message struct {
	Self    bool
	Content string
}

type PromptCompletion struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}
