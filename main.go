package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var (
	start = time.Now()
)

var outputStrs = make(chan string)

func init() {
	go func() {
		for s := range outputStrs {
			print(s)
		}
	}()
}

func pt(format string, args ...interface{}) {
	outputStrs <- fmt.Sprintf("%-20v", time.Now().Sub(start)) +
		fmt.Sprintf(format, args...)
}

var db *sqlx.DB

func init() {
	var err error
	db, err = sqlx.Connect("mysql", "root:ffffff@tcp(127.0.0.1:3306)/vvic?parseTime=true&autocommit=true")
	ce(err, "connect to db")
	initSchemas()
}

type ShopInfo struct {
	Qq            string
	Authenticated int
	Ww_nickname   string // 旺旺号
	Wechat        string
	Contacts_name string
	Telephone     []string
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

func main() {
	collectPages()
	collectDetailPages()
	downloadImages()
}

func collectPages() {
	// collect pages
	page := 1
	infos := []ShopInfo{}
	for {
		pageUrl := fmt.Sprintf("http://www.vvic.com/api/shop/navigation?bid=&currentPage=%d&pageSize=500",
			page)
		var data struct {
			Code int
			Data struct {
				CurrentPage int
				PageSize    int
				PageCount   int // 无用
				RecordCount int // 不等于len(RecordList)
				RecordList  []ShopInfo
			}
		}
		ce(decodeFromUrl(pageUrl, &data), "decode")
		if len(data.Data.RecordList) == 0 {
			break
		}
		infos = append(infos, data.Data.RecordList...)
		page++
		pt("%d %d\n", page, len(infos))
	}

	sem := make(chan bool, 8)
	wg := new(sync.WaitGroup)
	wg.Add(len(infos))
	for i, shop := range infos {
		sem <- true
		i := i
		shop := shop
		go func() {
			defer func() {
				wg.Done()
				<-sem
			}()
			collectShop(i, shop)
		}()
	}
	wg.Wait()
}

func getBody(url string) (body []byte, err error) {
	retry := 5
get:
	resp, err := http.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		return nil, err
	}
	return body, nil
}

var selectedMarkets = map[string]bool{
	"国大":  true, // 1672
	"女人街": true, // 1463
	"国投":  true, // 1179
	"大西豪": true, // 794
	"宝华":  true, // 430
	"大时代": true, // 424
	"佰润":  true, // 309
	"非凡":  true, // 244
	"柏美":  true, // 144

	"新金马": true, // 735
	"富丽":  true, // 648
}

func collectShop(i int, shop ShopInfo) {
	// 其他市场的不管
	if _, ok := selectedMarkets[shop.MarketName]; !ok {
		return
	}

	pt("%50s %d\n", "shop", i)

	// 判断是否更新了
	var data struct {
		Code int
		Data struct {
			Update_time string
		}
	}
	ce(decodeFromUrl(fmt.Sprintf("http://www.vvic.com/api/shop/%d", shop.Id), &data),
		"get shop info")
	curUpdate, err := time.Parse("2006-01-02 15:04:05", data.Data.Update_time)
	ce(err, "parse current update time")
	var lastUpdates []time.Time
	err = db.Select(&lastUpdates, `SELECT update_at FROM shops
			WHERE shop_id = ? LIMIT 1`, shop.Id)
	ce(err, "select last update time")
	if len(lastUpdates) > 0 && lastUpdates[0] == curUpdate {
		pt("last update at: %v, current update at %v, skip %d\n",
			lastUpdates[0], curUpdate, shop.Id)
		return
	}

	/*
		pt("%d\n", shop.Id)
		pt("%s\n", shop.Qq)
		pt("%s\n", shop.Ww_nickname)
		pt("%v\n", shop.Telephone)
		pt("%s\n", shop.Name)
		pt("%s %d楼 %s\n", shop.MarketName, shop.Floor, shop.Position)
		pt("%s\n", shop.Shop_category)
		pt("===\n\n")
	*/

	db.MustExec(`INSERT INTO shops (
				shop_id,
				name
			) VALUES (
				?,
				?
			)
			ON DUPLICATE KEY UPDATE name=name`,
		shop.Id,
		shop.Name,
	)

	maxPage := 9999
	page := 1
	for {
		if page > maxPage {
			break
		}

		var data struct {
			Code int
			Data struct {
				CurrentPage int
				PageCount   int // 总页数
				PageSize    int
				RecordCount int // 总商品数
				RecordList  []struct {
					Discount_price string  // 拿货价
					Tid            string  // ??
					Is_shop_auth   int     // ?
					Price          float64 // 原价
					Id             string
					Art_no         string // 货号
					Sub_name       string // 市场名
					Shop_name      string // 档口名
					Shop_id        int
					Up_time        int64   // 上架时间，millisecond
					Position       string  // 档口位置
					Upload_num     int     // ?
					Is_tx          int     // 是否退现
					Is_df          int     // 是否代发
					Is_sp          int     // 是否实拍
					Index_img_url  string  // 主图地址
					Title          string  // 标题
					Bname          string  // ?
					Bid            string  // ?
					Tcid           string  // 分类id
					Score          float64 // 分数 ？
					Sort_score     float64 // 排序分数 ？
				}
			}
		}

		ce(decodeFromUrl(fmt.Sprintf("http://www.vvic.com/rest/shop/search-item?shop_id=%d&q=&currentPage=%d",
			shop.Id, page), &data), "decode")
		if page == 1 { // 第一页
			maxPage = data.Data.PageCount
		}
		tx := db.MustBegin()
		for _, item := range data.Data.RecordList {

			if item.Is_tx != 1 { // 不支持退现的不理
				continue
			}

			pt("%s\n", item.Title)
			imageUrl := item.Index_img_url
			if !strings.HasPrefix(imageUrl, "http") && len(imageUrl) > 0 {
				imageUrl = "http:" + imageUrl
			}

			if imageUrl != "" {
				_, err := tx.Exec(`INSERT INTO images (
					good_id,
					url
				) VALUES (
					?,
					?
				) ON DUPLICATE KEY UPDATE good_id=good_id`, item.Id, imageUrl)
				ce(err, "insert image")
			}

		exec:
			_, err = tx.Exec(`INSERT INTO goods (
					good_id,
					price,
					shop_id,
					added_at,
					category,
					score,
					sort_score,
					title
				) VALUES (
					?,
					?,
					?,
					?,
					?,
					?,
					?,
					?
				) ON DUPLICATE KEY UPDATE 
					price=price,
					score=score,
					sort_score=sort_score,
					title=title
				`,
				item.Id,
				item.Discount_price,
				shop.Id,
				time.Unix(item.Up_time/1000, 0).Format("2006-01-02"),
				item.Tcid,
				item.Score,
				item.Sort_score,
				item.Title,
			)
			if err != nil {
				if err, ok := err.(*mysql.MySQLError); ok {
					if err.Number == 1216 {
						goto exec
					}
				}
				ce(err, "exec error")
			}

		}
		ce(tx.Commit(), "commit")
		page++
	}

	// 更新update_at
	db.MustExec(`UPDATE shops SET update_at = ?
			WHERE shop_id = ?`,
		curUpdate,
		shop.Id)
}

func decodeFromUrl(path string, target interface{}) (err error) {
	retry := 20
retry:
	pageResp, err := http.Get(path)
	if err != nil {
		if retry < 0 {
			return err
		}
		retry--
		goto retry
	}
	defer pageResp.Body.Close()
	err = json.NewDecoder(pageResp.Body).Decode(target)
	if err != nil {
		if retry < 0 {
			return err
		}
		retry--
		goto retry
	}
	return nil
}
