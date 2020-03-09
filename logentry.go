package logreceiver

import (
	"time"
)

// LogEntry : ...
type LogEntry struct {
	ID             int    `gorm:"primary_key"`
	Version        int    `json:"-"`
	ProcID         string `json:"proc_id"`
	Client         string `json:"-"`
	TLSPeer        string `json:"-"`
	Priority       int    `json:"-"`
	Facility       int    `json:"-"`
	Severity       int    `json:"severity"`
	Timestamp      int64  `json:"timestamp"`
	Hostname       string `json:"hostname"`
	App            string `json:"app"`
	Msgid          string `json:"msg_id"`
	StructuredData string `json:"-"`
	Message        string `json:"message"`
}

func getInt(logPart map[string]interface{}, i *int, name string) bool {
	intreger, ok := logPart[name].(int)
	if ok {
		*i = intreger
	}
	return ok
}

func getString(logPart map[string]interface{}, std *string, name string) bool {
	msg, ok := logPart[name].(string)
	if ok {
		*std = msg
	}
	return ok
}

// NewLogEntry : ...
func NewLogEntry(logPart map[string]interface{}) (*LogEntry, bool) {
	ok := false
	logEntry := &LogEntry{}
	if getInt(logPart, &logEntry.Version, "version") &&
		getString(logPart, &logEntry.ProcID, "proc_id") &&
		getString(logPart, &logEntry.Client, "client") &&
		getString(logPart, &logEntry.TLSPeer, "tls_peer") &&
		getInt(logPart, &logEntry.Priority, "priority") &&
		getInt(logPart, &logEntry.Facility, "facility") &&
		getInt(logPart, &logEntry.Severity, "severity") &&
		getString(logPart, &logEntry.Hostname, "hostname") &&
		getString(logPart, &logEntry.App, "app_name") &&
		getString(logPart, &logEntry.Msgid, "msg_id") &&
		getString(logPart, &logEntry.StructuredData, "structured_data") &&
		getString(logPart, &logEntry.Message, "message") {
		logEntry.Timestamp = time.Now().Unix()
		ok = true
	}
	return logEntry, ok
}
