package main

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"strings"
)

func collectCategories() {
	resp, err := http.Get("http://www.vvic.com/search.html")
	ce(err, "get")
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	ce(err, "new doc")

	var category1Names []string
	doc.Find(".nav-pid .types a[title]").Each(func(i int, se *goquery.Selection) {
		name := strings.TrimSpace(se.Text())
		category1Names = append(category1Names, name)
	})

	tx := db.MustBegin()
	n := 0

	nodes := doc.Find(".catid")
	if nodes.Length() != len(category1Names) {
		panic("bad top categories")
	}
	nodes.Each(func(i1 int, se *goquery.Selection) {
		se.Find(".nav-category").Each(func(i2 int, se *goquery.Selection) {
			category2Name := se.Find(".nc-key").Text()
			se.Find(".nc-value a[href ^= '#/tcid/']").Each(func(i3 int, se *goquery.Selection) {
				n++
				href, ok := se.Attr("href")
				if !ok {
					panic("bad entry")
				}
				categoryId := strings.TrimPrefix(href, "#/tcid/")
				categoryName, ok := se.Attr("title")
				if !ok {
					panic("bad entry")
				}
				pt("%s %s %s %s\n", categoryId, category1Names[i1], category2Name, categoryName)
				_, err := tx.Exec(`INSERT INTO categories
					(category_id, name1, name2, name3, display_order)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (category_id)
					DO UPDATE
					SET name1 = $2, name2 = $3, name3 = $4, display_order = $5
					`,
					categoryId,
					category1Names[i1],
					category2Name,
					categoryName,
					n,
				)
				ce(err, "insert")
			})
		})
	})

	ce(tx.Commit(), "commit")
}
