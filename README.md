# wechatgpt
wechat &amp; chatgpt rendezvous

How to run?

1. `brew install go`
2. `git clone https://github.com/safiir/wechatgpt`
3. create a `.env` file, fill it with the following configuration
```env
proxy="http://localhost:7890" # http proxy to speed up the connection
token="your token" # generate token at https://platform.openai.com/account/api-keys
```
4. `go run .`
