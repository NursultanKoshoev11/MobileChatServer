package service

import (
	"fmt"
	"log"
)

func safeGo(logger *log.Logger, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				if logger != nil {
					logger.Printf("background task panic recovered: %q", fmt.Sprint(recovered))
				}
			}
		}()
		fn()
	}()
}
