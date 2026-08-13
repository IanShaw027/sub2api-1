package service

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var serviceGinTestModeOnce sync.Once

func setGinTestMode() {
	serviceGinTestModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}
