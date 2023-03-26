package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
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

func ToJson(v any) string {
	b, _ := json.MarshalIndent(v, " ", "\t")
	return string(b)
}

func Last[T any](xs []T, limit int) []T {
	length := len(xs)
	start := length - limit
	if start < 0 {
		start = 0
	}
	return xs[start:length]
}

func DownloadRemote(url string) (io.ReadCloser, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, errors.New("received non 200 response code")
	}
	return response.Body, nil
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
	defer file.Close()
	ioutil.WriteFile(file.Name(), []byte(html), fs.ModeTemporary)

	png, err := ioutil.TempFile("", "prefix.*.png")
	if err != nil {
		return nil, err
	}

	page := rod.New().MustConnect().MustPage(fmt.Sprintf("file:///%s", file.Name()))
	page.MustWaitLoad().MustScreenshot(png.Name())

	return png, nil
}
