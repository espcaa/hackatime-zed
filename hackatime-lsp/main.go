package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const (
	eventDebounceMs = 50
	batchSendMs     = 120 * 1000
	maxQueueSize    = 100
	cliTimeoutSecs  = 10
)

var (
	projectRoot     string
	projectFolder   string
	wakatimeCliPath string
	heartbeatQueue  []Heartbeat
	queueMutex      sync.Mutex
	lastEventTime   map[string]time.Time
	eventMutex      sync.Mutex
	batchSendTimer  *time.Timer
	lastSentTime    time.Time
	metricsEnabled  bool
)

var (
	lastCursorPos map[string]int
	cursorMutex   sync.Mutex
	logMutex      sync.Mutex
)

func saveCursorPosition(uri string, line, pos int) {
	cursorMutex.Lock()
	defer cursorMutex.Unlock()

	if lastCursorPos == nil {
		lastCursorPos = make(map[string]int)
	}
	lastCursorPos[uri] = pos
}

func getCursorPosition(uri string) int {
	cursorMutex.Lock()
	defer cursorMutex.Unlock()

	if pos, exists := lastCursorPos[uri]; exists {
		return pos
	}
	return 0
}

func logEvent(eventType string, hb Heartbeat) {
	logMutex.Lock()
	defer logMutex.Unlock()

	logPath := filepath.Join(os.Getenv("HOME"), "hackatime-zed.log")

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"event":     eventType,
		"heartbeat": hb,
	}

	data, _ := json.Marshal(logEntry)
	fmt.Fprintf(file, "%s\n", string(data))
}

func sendHeartbeat(hb Heartbeat) error {
	cliPath := wakatimeCliPath
	if cliPath == "" {
		return errors.New("wakatime-cli path not provided")
	}

	args := buildHeartbeatArgs(hb)

	ctx, cancel := context.WithTimeout(context.Background(), cliTimeoutSecs*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, args...)
	return cmd.Run()
}

func queueHeartbeat(hb Heartbeat) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	if hb.AlternateProject == "" && projectRoot != "" {
		hb.AlternateProject = filepath.Base(projectRoot)
	}
	if hb.ProjectFolder == "" && projectFolder != "" {
		hb.ProjectFolder = projectFolder
	}

	heartbeatQueue = append(heartbeatQueue, hb)

	if len(heartbeatQueue) >= maxQueueSize {
		go flushHeartbeats()
	} else if len(heartbeatQueue) == 1 {
		scheduleBatchSend()
	}
}

func scheduleBatchSend() {
	if batchSendTimer != nil {
		return
	}

	batchSendTimer = time.AfterFunc(time.Duration(batchSendMs)*time.Millisecond, func() {
		batchSendTimer = nil
		flushHeartbeats()
	})
}

func flushHeartbeats() {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	if len(heartbeatQueue) == 0 {
		return
	}

	hb := heartbeatQueue[0]
	heartbeatQueue = heartbeatQueue[1:]

	go sendHeartbeat(hb)
	lastSentTime = time.Now()

	if len(heartbeatQueue) > 0 {
		scheduleBatchSend()
	}
}

func throttledHeartbeat(hb Heartbeat) {
	eventMutex.Lock()
	defer eventMutex.Unlock()

	if lastEventTime == nil {
		lastEventTime = make(map[string]time.Time)
	}

	now := time.Now()
	lastTime, exists := lastEventTime[hb.Entity]

	if hb.IsWrite {
		lastEventTime[hb.Entity] = now
		go queueHeartbeat(hb)
	} else if !exists || now.Sub(lastTime) >= time.Duration(eventDebounceMs)*time.Millisecond {
		lastEventTime[hb.Entity] = now
		go queueHeartbeat(hb)
	}
}

func main() {
	flag.StringVar(&wakatimeCliPath, "wakatime-cli", "", "Path to wakatime-cli binary")
	flag.Parse()

	handler := protocol.Handler{
		Initialize: func(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
			if params.RootURI != nil {
				projectRoot = cleanFileURI(*params.RootURI)
				projectFolder = projectRoot
			} else if params.RootPath != nil {
				projectRoot = filepath.Clean(*params.RootPath)
				projectFolder = projectRoot
			}

			capabilities := protocol.ServerCapabilities{
				TextDocumentSync: protocol.TextDocumentSyncKindIncremental,
			}
			return protocol.InitializeResult{Capabilities: capabilities}, nil
		},

		TextDocumentDidChange: func(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
			uri := cleanFileURI(params.TextDocument.URI)

			lines := 1
			lineNumber := 1
			cursorPos := 0

			if len(params.ContentChanges) > 0 {
				change := params.ContentChanges[0]

				if changeEvent, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
					if changeEvent.Range != nil {
						lineNumber = int(changeEvent.Range.Start.Line) + 1
						cursorPos = int(changeEvent.Range.Start.Character)
					}
					if changeEvent.Text != "" {
						lines = len(strings.Split(changeEvent.Text, "\n"))
					}
				}
			}

			saveCursorPosition(uri, lineNumber, cursorPos)

			hb := Heartbeat{
				Entity:     uri,
				EntityType: "file",
				Category:   "coding",
				Plugin:     "Zed",
				Time:       float64(time.Now().UnixMilli()) / 1000.0,
				LineNumber: lineNumber,
				CursorPos:  cursorPos,
				Lines:      lines,
			}

			logEvent("TextDocumentDidChange", hb)
			throttledHeartbeat(hb)
			return nil
		},

		TextDocumentDidSave: func(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
			uri := cleanFileURI(params.TextDocument.URI)

			lines := 1
			if params.Text != nil {
				lines = len(strings.Split(*params.Text, "\n"))
			}

			hb := Heartbeat{
				Entity:     uri,
				EntityType: "file",
				Category:   "coding",
				Plugin:     "Zed",
				Time:       float64(time.Now().UnixMilli()) / 1000.0,
				LineNumber: 1,
				Lines:      lines,
				CursorPos:  getCursorPosition(uri),
				IsWrite:    true,
			}

			logEvent("TextDocumentDidSave", hb)
			throttledHeartbeat(hb)
			return nil
		},
	}

	s := server.NewServer(&handler, "hackatime-lsp", false)
	s.RunStdio()
}

func cleanFileURI(uri string) string {
	path := strings.TrimPrefix(uri, "file://")
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return filepath.Clean(path)
}
