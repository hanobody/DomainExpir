package scheduler

import (
	"log"
	"time"
)

func ScheduleDailyAt(hour, min int, job func()) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			wait := next.Sub(now)
			log.Printf("距离下次任务还有: %v", wait)
			time.Sleep(wait)
			job()
		}
	}()
}
