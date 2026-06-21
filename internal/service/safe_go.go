package service

import "log"

func safeGo(logger *log.Logger, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				if logger != nil {
					logger.Printf("background task panic recovered: %v", recovered)
				}
			}
		}()
		fn()
	}()
}
