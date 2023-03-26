module wechatgpt

go 1.19

require (
	github.com/eatmoreapple/openwechat v1.4.1
	github.com/go-rod/rod v0.112.6
	github.com/gomarkdown/markdown v0.0.0-20230313173142-2ced44d5b584
	github.com/joho/godotenv v1.5.1
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/samber/lo v1.37.0
	github.com/sashabaranov/go-openai v1.5.2
	github.com/syndtr/goleveldb v1.0.0
)

require golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect

require (
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/xeodou/go-sqlcipher v0.0.0-20200727080346-d681773ef093
	github.com/ysmood/goob v0.4.0 // indirect
	github.com/ysmood/gson v0.7.3 // indirect
	github.com/ysmood/leakless v0.8.0 // indirect
	golang.org/x/exp v0.0.0-20220303212507-bbda1eaf7a17 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

replace github.com/xeodou/go-sqlcipher => github.com/safiir/go-sqlcipher v0.0.0-20230326101051-abf681aaa980
