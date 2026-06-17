package service

import (
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

var serviceGinTestModeOnce sync.Once

func setGinTestMode() {
	serviceGinTestModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}

func TestMain(m *testing.M) {
	setGinTestMode()
	os.Exit(m.Run())
}
