package wechat_dumper

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/xeodou/go-sqlcipher"
)

var _ = godotenv.Load()

var WechatSqliteRawKey = func() string {
	key := os.Getenv("wechat_key")
	return fmt.Sprintf("x'%s'", key)
}()

var WechatHome = func() string {
	home := os.Getenv("HOME")
	return fmt.Sprintf("%s/Library/Containers/com.tencent.xinWeChat/Data/Library/Application Support/com.tencent.xinWeChat/", home)
}()

func GenerateFuneTuningDataset(keyword string) (*os.File, error) {
	paths := []string{}
	err := filepath.Walk(WechatHome, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".db" && strings.HasPrefix(info.Name(), "msg_") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		fmt.Printf("searching %v\n", filepath.Base(path))
		msgs, err := ScanDatabase(path, keyword)
		if err != nil {
			return nil, err
		}
		if msgs != nil {
			fmt.Println("table found")
			fmt.Printf("total Messages: %v\n", len(*msgs))
			if false {
				for _, msg := range *msgs {
					if msg.Self {
						fmt.Printf("A: %v\n", msg.Content)
					} else {
						fmt.Printf("B: %v\n", msg.Content)
					}
				}
			}
			return ComposeDataset(msgs)
		}
	}
	return nil, errors.New("chat not found")
}

func ComposeDataset(msgs *[]Message) (*os.File, error) {
	doc, err := ioutil.TempFile("", "prefix.*.jsonl")
	if err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(doc)
	for i := 0; i < len(*msgs)-1; i++ {
		prompt := (*msgs)[i]
		completion := (*msgs)[i+1]
		if !prompt.Self && completion.Self {
			x := PromptCompletion{
				Prompt:     prompt.Content,
				Completion: completion.Content,
			}
			bytes, _ := json.Marshal(x)
			_, _ = writer.WriteString(string(bytes) + "\n")
		}
	}
	writer.Flush()
	return doc, nil
}

func ScanDatabase(path string, keyword string) (*[]Message, error) {
	dsn := fmt.Sprintf("file:%s?_key=%s&_cipher_compatibility=3", path, WechatSqliteRawKey)
	db, err := sql.Open("sqlite3", dsn)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT name 
		FROM sqlite_master
		WHERE type = 'table'
			AND name NOT LIKE 'sqlite_%'
			AND name LIKE 'Chat_%'
			AND name NOT LIKE '%_dels';
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []string{}

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		msgs, err := ScanTable(db, table, keyword)
		if err != nil {
			return nil, err
		}
		if msgs != nil {
			return msgs, nil
		}
	}
	return nil, nil
}

func ScanTable(db *sql.DB, table string, keyword string) (*[]Message, error) {
	rows, err := db.Query(fmt.Sprintf(`SELECT msgContent, mesDes FROM %s;`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs := []Message{}

	for rows.Next() {
		var msgDest int64
		var msgContent string
		err = rows.Scan(&msgContent, &msgDest)
		if err != nil {
			return nil, err
		}
		if strings.Contains(msgContent, "<?xml") {
			continue
		}
		if strings.Contains(msgContent, "<msg>") {
			continue
		}
		msgs = append(msgs, Message{
			Self:    msgDest == 0,
			Content: msgContent,
		})
		if strings.Contains(msgContent, keyword) {
			return &msgs, nil
		}
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return nil, nil
}
