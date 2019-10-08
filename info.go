package logreceiver

// Info : info struct for the logreceiver object
type Info struct {
	Version             string `json:"version"`
	Port                int    `json:"port"`
	MaxLogs             int    `json:"max_logs"`
	LogCount            int    `json:"log_count"`
	OpenWebSocketsCount int    `json:"open_web_sockets_count"`
	Uptime              int64  `json:"uptime"`
}
