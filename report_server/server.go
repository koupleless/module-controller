package report_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/koupleless/module_controller/common/zaplogger"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/koupleless/virtual-kubelet/common/utils"
)

// LogRequest represents a log request with various fields
type LogRequest struct {
	TraceID  string            `json:"traceID"`            // Unique identifier for the log request
	Scene    string            `json:"scene"`              // Scene or context of the log request
	Event    string            `json:"event"`              // Event that triggered the log request
	TimeUsed int64             `json:"timeUsed,omitempty"` // Time taken for the event to complete
	Result   string            `json:"result"`             // Result of the event
	Message  string            `json:"message"`            // Additional message for the log request
	Code     string            `json:"code"`               // Error code if the event failed
	Labels   map[string]string `json:"labels"`             // Additional labels for the log request
}

// DingtalkMessage represents a message to be sent to Dingtalk
type DingtalkMessage struct {
	MsgType  string `json:"msgtype"` // Message type
	MarkDown struct {
		Title string `json:"title"` // Title of the message
		Text  string `json:"text"`  // Content of the message
	} `json:"markdown"` // Markdown content of the message
}

// formatMap converts a map to a formatted string
func formatMap(input map[string]string) string {
	var sb strings.Builder
	sb.WriteString("[")
	for key, value := range input {
		sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	sb.WriteString("]")
	return sb.String()
}

// webhooks stores the list of webhooks to send messages to
var webhooks []string

// logHandler handles incoming log requests
func logHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var logRequest LogRequest
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := json.Unmarshal(body, &logRequest); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Construct the message to be sent
	text := fmt.Sprintf("## MC告警\n\n- **Trace ID**: %s\n- **工单类型**: %s\n- **节点**: %s\n- **结果**: %s\n- **失败信息**: %s\n- **错误代码**: %s\n- **部署信息**: %s",
		logRequest.TraceID, logRequest.Scene, logRequest.Event, logRequest.Result, logRequest.Message, logRequest.Code, formatMap(logRequest.Labels))

	// Prepare the Dingtalk message
	dingtalkMessage := DingtalkMessage{
		MsgType: "markdown",
		MarkDown: struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		}{
			Title: "Log Notification",
			Text:  text,
		},
	}
	jsonMessage, err := json.Marshal(dingtalkMessage)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Iterate over all webhooks and send messages
	for _, webhook := range webhooks {
		if webhook == "" {
			continue
		}
		_, err = http.Post(webhook, "application/json", bytes.NewBuffer(jsonMessage))
		if err != nil {
			log.Println("Error sending to webhook:", err.Error())
		}
	}
	w.WriteHeader(http.StatusOK)
}

// InitReportServer initializes the report server
func InitReportServer() {
	reportHooks := utils.GetEnv("REPORT_HOOKS", "")

	webhooks = strings.Split(reportHooks, ";")

	http.HandleFunc("/log", logHandler)
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		zaplogger.GetLogger().Error(err, "Server failed: %v")
	}
}
