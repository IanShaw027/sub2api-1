package transport

type Identity string

const (
	HTTP1       Identity = "http1"
	H2          Identity = "h2"
	WebSocketH1 Identity = "websocket-http1"
	WebSocketH2 Identity = "websocket-h2"
)
