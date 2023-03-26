# wechatgpt

wechat &amp; chatgpt rendezvous

> **How to run?**

1. `brew install go`
2. `git clone https://github.com/safiir/wechatgpt`
3. create a `.env` file, fill it with the following configuration

```env
proxy="http://localhost:7890" # http proxy to speed up the connection
token="your token" # generate token at https://platform.openai.com/account/api-keys
wechat_key="your wechat encryption key" # you can got sqlcipher encryption raw key by running dumper/run.sh script
```

4. `CGO_ENABLE=1 CGO_LDFLAGS="-L/usr/local/opt/openssl/lib" CGO_CPPFLAGS="-I/usr/local/opt/openssl/include" go run .`
5. `CGO_ENABLE=1 CGO_LDFLAGS="-L/usr/local/opt/openssl/lib" CGO_CPPFLAGS="-I/usr/local/opt/openssl/include" go build`

# Got Wechat key

> **Only work on Mac OS**

1. Run `dumper/run.sh`
2. After `(lldb) continue` occurs, click `Login in` button on wechat to proceed
3. If everything is ok, the raw key will be printed on the console and copied to the pasteboard
4. Set above key as `wechat_key` env variable
