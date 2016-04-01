package main

import (
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

func downloadImages() {
	// download images
	var nBytes int64
	go func() {
		for range time.NewTicker(time.Second).C {
			pt("%d\n", atomic.SwapInt64(&nBytes, 0))
		}
	}()
	fileExists := make(map[string]bool)
	fileExistsLock := new(sync.Mutex)
	for {
		var images []Image
		pt("foo\n")
		err := db.Select(&images, `SELECT * FROM images 
			WHERE sha512 IS NULL 
			AND url <> ""
			LIMIT 4096`)
		ce(err, "select images")
		pt("collecting %d images\n", len(images))
		wg := new(sync.WaitGroup)
		wg.Add(len(images))
		sem := make(chan bool, 4)
		for _, image := range images {
			sem <- true
			image := image
			go func() {
				defer func() {
					<-sem
					wg.Done()
				}()
				ce(downloadImage(image, &nBytes, fileExists, fileExistsLock), "download image")
			}()
		}
		wg.Wait()
		if len(images) == 0 {
			break
		}
	}

}

func downloadImage(image Image, nBytes *int64, fileExists map[string]bool, fileExistsLock *sync.Mutex) (err error) {
	defer ct(&err)
	// get image
	body, err := getBody(image.Url)
	ce(err, "get image content %s %d", image.Url, image.GoodId)
	// sum
	sumAry := sha512.Sum512(body)
	sum := sumAry[:]
	sumHex := fmt.Sprintf("%x", sum)
	// write to tmp
	var exists bool
	fileExistsLock.Lock()
	if _, exists = fileExists[sumHex]; exists { // only one goroutine should be write to file
	} else {
		fileExists[sumHex] = true
	}
	fileExistsLock.Unlock()
	if !exists {
		fileName := filepath.Join("images", fmt.Sprintf("%x%s", sum,
			filepath.Ext(image.Url)))
		tmpFile, err := os.Create(fileName + ".tmp")
		ce(err, "create tmp file")
		_, err = tmpFile.Write(body)
		ce(err, "write tmp file")
		tmpFile.Close()
		// rename
		ce(os.Rename(fileName+".tmp", fileName), "rename file")
	}
	// update db
	_, err = db.Exec(`UPDATE images SET sha512 = ?
				WHERE good_id = ?`,
		sum, image.GoodId)
	ce(err, "update hash sum")
	// stat
	atomic.AddInt64(nBytes, int64(len(body)))
	return
}
