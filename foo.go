package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func foo() {
	// load url info
	urlsFile, err := os.Open("/home/reus/urls.data")
	ce(err, "open urls file")
	defer urlsFile.Close()
	r := bufio.NewReaderSize(urlsFile, 1024*1024*512)
	scanner := bufio.NewScanner(r)

	type Url struct {
		Url        string
		Sha512_16k string
		Length     string
	}

	n := 0
	urlInfos := make(map[string]*Url)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 6 {
			panic("bad line")
		}
		urlInfos[parts[0]] = &Url{
			Url:        parts[1],
			Sha512_16k: parts[2],
			Length:     parts[5],
		}

		n++
		if n%100000 == 0 {
			pt("urls %d\n", n)
		}
	}
	ce(scanner.Err(), "scan error")
	pt("load %d urls\n", n)

	// generate images file
	imagesFile, err := os.Open("/home/reus/images.data")
	ce(err, "open images file")
	defer imagesFile.Close()
	reader := bufio.NewReaderSize(imagesFile, 1024*1024*512)
	s := bufio.NewScanner(reader)

	out, err := os.Create("/home/reus/new_images.data")
	ce(err, "new images data file")
	defer out.Close()

	n = 0
	for s.Scan() {
		parts := strings.Split(s.Text(), "\t")
		if len(parts) != 6 {
			panic("bad line")
		}

		urlInfo, ok := urlInfos[parts[1]]
		if !ok {
			panic("no url info")
		}

		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n",
			parts[0],
			parts[1],
			parts[2],
			urlInfo.Url,
			urlInfo.Sha512_16k,
			urlInfo.Length,
		)
		ce(err, "write line")

		n++
		if n%10000 == 0 {
			pt("images %d\n", n)
		}
	}
	ce(s.Err(), "scan error")
	pt("write %d images entries\n", n)

	time.Sleep(time.Second)
}
