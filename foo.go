package main

import (
	"time"
)

func foo() {
	groupId := 92794
	var groupHashes StringArray
	ce(db.Get(&groupHashes, `SELECT hashes FROM groups WHERE group_id = $1`, groupId), "get hashes")
	groupHashSet := make(map[string]bool)
	for _, hash := range groupHashes {
		groupHashSet[hash] = true
	}

	goodId := 2559801
	var goodHashes StringArray
	ce(db.Get(&goodHashes, `SELECT encode(sha512_16k, 'hex')
		FROM images i
		LEFT JOIN urls u ON i.url_id = u.url_id
		WHERE i.good_id = $1
		`,
		goodId,
	), "get good hashes")

	count := 0
	for _, hash := range goodHashes {
		if _, ok := groupHashSet[hash]; ok {
			count++
		}
	}
	pt("%d %d\n", len(groupHashSet), count)

	time.Sleep(time.Second)
}
