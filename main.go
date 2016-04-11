package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const semSize = 2

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

type Url struct {
	UrlId  int `db:"url_id"`
	Url    string
	Sha512 []byte
}

func main() {
	//collectShops()
	//collectGoods()
	hashImages()
}

func collectShops() {
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

	skip := make(map[int]bool)
	var ids []int
	ts := time.Now().Add(-time.Hour * 8).Unix()
	err := db.Select(&ids, `SELECT shop_id
		FROM shops 
		WHERE last_update_time > $1`,
		ts)
	ce(err, "select skip shop ids")
	for _, id := range ids {
		skip[id] = true
	}

	sem := make(chan bool, semSize)
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
			collectShop(skip, i, shop)
		}()
	}
	wg.Wait()

	pt("shops collected\n")
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

func collectShop(skip map[int]bool, i int, shop ShopInfo) {
	// 其他市场的不管
	//if _, ok := selectedMarkets[shop.MarketName]; !ok {
	//	return
	//}

	pt("%50s %d\n", "shop", i)

	// 近期采集过的不管
	if _, ok := skip[shop.Id]; ok {
		return
	}

	db.MustExec(`INSERT INTO shops (
				shop_id,
				name
			) VALUES ($1, $2)
			ON CONFLICT (shop_id) DO UPDATE SET name = $2`,
		shop.Id,
		shop.Name,
	)

	// set existing goods' status to 0
	db.MustExec(`UPDATE goods SET
		status = 0
		WHERE shop_id = $1`,
		shop.Id)

	// collect in sale goods
	maxPage := 9999
	page := 1
	for {
		if page > maxPage {
			break
		}

		var data struct {
			Code int
			Data struct {
				//CurrentPage int
				PageCount int // 总页数
				//PageSize    int
				//RecordCount int // 总商品数
				RecordList []struct {
					Discount_price interface{} // 拿货价
					//Tid            string  // ??
					//Is_shop_auth   int     // ?
					//Price          float64 // 原价
					Id string
					//Art_no         string // 货号
					//Sub_name       string // 市场名
					//Shop_name      string // 档口名
					//Shop_id        int
					Up_time int64 // 上架时间，millisecond
					//Position       string  // 档口位置
					//Upload_num     int     // ?
					Is_tx int // 是否退现
					//Is_df          int     // 是否代发
					//Is_sp          int     // 是否实拍
					//Index_img_url  string  // 主图地址
					Title string // 标题
					//Bname          string  // ?
					//Bid            string  // ?
					Tcid       string  // 分类id
					Score      float64 // 分数 ？
					Sort_score float64 // 排序分数 ？
				}
			}
		}

		url := fmt.Sprintf("http://www.vvic.com/rest/shop/search-item?shop_id=%d&q=&currentPage=%d",
			shop.Id, page)
		retry := 5
	decode:
		err := decodeFromUrl(url, &data)
		if err != nil {
			if retry > 0 {
				retry--
				goto decode
			}
			ce(err, "decode data %s", url)
		}
		if page == 1 { // 第一页
			maxPage = data.Data.PageCount
		}
		ce(withTx(db, func(tx *sqlx.Tx) (err error) {
			defer ct(&err)
			for _, item := range data.Data.RecordList {

				//if item.Is_tx != 1 { // 不支持退现的不理
				//	continue
				//}

				if item.Discount_price == nil {
					continue
				}
				var price float64
				switch p := item.Discount_price.(type) {
				case string:
					price, err = strconv.ParseFloat(p, 64)
					ce(err, "parse price")
				case float64:
					price = p
				default:
					panic(fmt.Sprintf("invalid price %T", item.Discount_price))
				}
				if price == 0 { // 没有批发价的不理
					continue
				}

				_, err = tx.Exec(`INSERT INTO goods (
					good_id,
					price,
					shop_id,
					added_at,
					category,
					score,
					sort_score,
					title,
					status
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
					ON CONFLICT (good_id) DO UPDATE SET
					price = $2,
					score = $6,
					sort_score = $7,
					title = $8,
					status = 1
				`,
					item.Id,
					price,
					shop.Id,
					time.Unix(item.Up_time/1000, 0).Format("2006-01-02"),
					item.Tcid,
					item.Score,
					item.Sort_score,
					item.Title,
				)
				ce(err, "insert goods")

			}
			return
		}), "tx")
		page++
	}

	// 更新
	db.MustExec(`UPDATE shops SET last_update_time = $1
		WHERE shop_id = $2`,
		time.Now().Unix(),
		shop.Id)
}
