package dbs

import "time"

func timeNowMs() int64 {
	return time.Now().UnixMilli()
}
