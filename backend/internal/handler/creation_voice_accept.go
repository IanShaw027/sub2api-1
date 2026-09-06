package handler

import (
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

func acceptVoiceRealtime(c *gin.Context) (*coderws.Conn, error) {
	return coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
}
