package main

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

var semSize int

func init() {
	semSizeStr := os.Args[1]
	var err error
	semSize, err = strconv.Atoi(semSizeStr)
	if err != nil {
		panic("invalid sem size")
	}
}

type ShopInfo struct {
	Qq            string
	Authenticated int
	Ww_nickname   string // 旺旺号
	Wechat        string
	Contacts_name string
	Telephone     json.RawMessage
	MarketName    string // 市场
	Name          string // 档口名
	Id            int
	Position      string // 档口
	Floor         int    // 市场楼层
	Bid           int    // ?
	Shop_category string // 主营
	Cid           int    // ?
	Status        int    // ?
}

type Image struct {
	GoodId int `db:"good_id"`
	Url    string
	Sha512 []byte
}

type Url struct {
	UrlId  int `db:"url_id"`
	Url    string
	Sha512 []byte
}

func main() {
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "shops":
			collectShops()
		case "goods":
			collectGoods()
		case "images":
			collectImageInfos()
		case "group":
			groupGoods()
		case "fixgroup":
			groupByInternalId()
		case "foo":
			foo()
		case "rank":
			collectRankings()
		case "size":
			collectImageSize()
		case "length":
			collectImageLength()
		}

	} else {
		collectShops()
		collectGoods()
		collectImageInfos()
		groupGoods()
		collectRankings()
	}

	time.Sleep(time.Second)
}
