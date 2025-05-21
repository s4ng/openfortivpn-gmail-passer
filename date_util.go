package main

import (
	"fmt"
	"log"
	"time"
)

func parseDate(dateStr string) (time.Time, error) {
	// 날짜 레이아웃 추가
	layouts := []string{
		"Mon, 2 Jan 2006 15:04:05",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 GMT",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
	}
	// 순회하면서 검사
	for _, layout := range layouts {
		t, err := time.Parse(layout, dateStr)
		if err == nil {
			return t, nil
		}
	}
	// 모든 형식에 대해 실패한 경우 에러 반환
	log.Println("date : " + dateStr)
	return time.Time{}, fmt.Errorf("unable to parse date")
}
