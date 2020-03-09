package logreceiver

import (
	"container/list"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/aortizgu/worker"
	"github.com/grandcat/zeroconf"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite" //import
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

// Version : LogReceiver Version
const Version = "0.0.0"

// LogReceiver : main object
type LogReceiver struct {
	name           string
	service        string
	domain         string
	dbPath         string
	port           int
	cleanPeriodMs  int
	maxLogs        int
	zeroconfserver *zeroconf.Server
	db             *gorm.DB
	syslogServer   *syslog.Server
	syslogChan     syslog.LogPartsChannel
	clients        map[*Client]bool
	broadcast      chan []byte
	register       chan *Client
	unregister     chan *Client
	startTime      time.Time
	cleanerWorker  worker.Worker
	writerWorker   worker.Worker
	iface          net.Interface
	logQueue       *list.List
	mutex          *sync.Mutex
}

// NewLogReceiver : log server object with functional modules
// |------DB
// |------DB Writer/Cleaner
// |------Syslog Server
// |------WebSocket Server
// |------mDNS Server
func NewLogReceiver(name, service, domain, dbPath string, port, cleanPeriodMs, maxLogs int, iface net.Interface) *LogReceiver {
	l := LogReceiver{
		name:          name,
		service:       service,
		domain:        domain,
		dbPath:        dbPath,
		port:          port,
		cleanPeriodMs: cleanPeriodMs,
		maxLogs:       maxLogs,
		broadcast:     make(chan []byte),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		clients:       make(map[*Client]bool),
		startTime:     time.Now(),
		iface:         iface,
	}
	l.mutex = &sync.Mutex{}
	l.logQueue = list.New()
	return &l
}

// Runable handler for Syslog Server
func (l *LogReceiver) runSyslogHandler() {
	log.Println("syslog chanel starts")
	for logParts := range l.syslogChan {
		logMsg, ok := NewLogEntry(logParts)
		if ok {
			l.mutex.Lock()
			l.logQueue.PushBack(logMsg)
			l.mutex.Unlock()
			//l.db.Save(logMsg)
			logsJSON, err := json.Marshal(logMsg)
			if err == nil {
				logBYTES := []byte(logsJSON)
				for client := range l.clients {
					select {
					case client.send <- logBYTES:
					default:
						close(client.send)
						delete(l.clients, client)
					}
				}
			}
		}
	}
	log.Println("syslog chanel stops")
}

// Runable handler for Websocket Server
func (l *LogReceiver) runWebsocketClientHandler() {
	for {
		select {
		case client := <-l.register:
			l.clients[client] = true
		case client := <-l.unregister:
			if _, ok := l.clients[client]; ok {
				delete(l.clients, client)
				close(client.send)
			}
			//do nothing when client message
			//case message := <-l.broadcast:
		}
	}
}

// Runable handler for Worker
func (l *LogReceiver) runDbWriterWork() bool {
	l.mutex.Lock()
	var queue = l.logQueue
	l.logQueue = list.New()
	l.mutex.Unlock()
	for element := queue.Front(); element != nil; element = element.Next() {
		l.db.Save(element.Value)
	}
	for element := queue.Front(); element != nil; element = element.Next() {
		queue.Remove(element)
	}
	return true
}

// Runable handler for Worker
func (l *LogReceiver) runDbCleanerWork() bool {
	count := 0
	l.db.Model(&LogEntry{}).Count(&count)
	if count > 0 {
		toDelete := count - l.maxLogs
		if toDelete > 0 {
			l.db.Exec("delete from log_entries where id IN (SELECT id from log_entries order by id asc limit ?)", toDelete)
			log.Println("LogReceiver::runDbCleaner: deleted oldest", toDelete, "entries")
		}
	}
	return true
}

// Start functional members
func (l *LogReceiver) Start() {
	ifaces := []net.Interface{l.iface}
	//starts zeroconf server
	zeroconfserver, err := zeroconf.Register(l.name, l.service, l.domain, l.port, nil, ifaces)
	if err != nil {
		panic(err)
	}
	l.zeroconfserver = zeroconfserver

	//starts db
	db, err := gorm.Open("sqlite3", l.dbPath)
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&LogEntry{})
	l.db = db
	l.db.LogMode(true)

	//starts syslog server
	l.syslogChan = make(syslog.LogPartsChannel)
	syslogHandler := syslog.NewChannelHandler(l.syslogChan)
	l.syslogServer = syslog.NewServer()
	l.syslogServer.SetFormat(syslog.RFC5424)
	l.syslogServer.SetHandler(syslogHandler)
	l.syslogServer.ListenUDP("0.0.0.0:" + strconv.Itoa(l.port))
	l.syslogServer.Boot()
	go l.runSyslogHandler()

	//starts workers
	l.cleanerWorker.Start(l.cleanPeriodMs, l.runDbCleanerWork)
	l.writerWorker.Start(5000, l.runDbWriterWork)

	//start websocket handler
	go l.runWebsocketClientHandler()
}

// Search : search in db
func (l *LogReceiver) Search(app, hostname string, from, to, severity, maxEntries, offsetEntries int64) []LogEntry {
	var logs []LogEntry
	filter := l.db.Where("")
	if app != "" {
		filter = filter.Where("app LIKE ?", "%"+app+"%")
	}
	if hostname != "" {
		filter = filter.Where("hostname LIKE ?", "%"+hostname+"%")
	}
	if from != 0 && to != 0 {
		filter = filter.Where("timestamp >= ?", from)
		filter = filter.Where("timestamp <= ?", to)
	}
	filter = filter.Where("severity <= ?", severity)
	if maxEntries != 0 {
		filter.Offset(offsetEntries).Limit(maxEntries).Find(&logs)
	} else {
		filter.Offset(offsetEntries).Find(&logs)
	}
	return logs
}

// GetInfo : get info about object
func (l *LogReceiver) GetInfo() *Info {
	count := 0
	l.db.Model(&LogEntry{}).Count(&count)
	info := &Info{
		Version:             Version,
		LogCount:            count,
		MaxLogs:             l.maxLogs,
		OpenWebSocketsCount: len(l.clients),
		Port:                l.port,
		Uptime:              int64(time.Since(l.startTime) / time.Second),
	}
	return info
}
