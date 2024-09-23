package report_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/koupleless/virtual-kubelet/common/utils"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type LogRequest struct {
	TraceID  string            `json:"traceID"`
	Scene    string            `json:"scene"`
	Event    string            `json:"event"`
	TimeUsed int64             `json:"timeUsed,omitempty"`
	Result   string            `json:"result"`
	Message  string            `json:"message"`
	Code     string            `json:"code"`
	Labels   map[string]string `json:"labels"`
}

type DingtalkMessage struct {
	MsgType  string `json:"msgtype"`
	MarkDown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
}

func formatMap(input map[string]string) string {
	var sb strings.Builder
	sb.WriteString("[")
	for key, value := range input {
		sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	sb.WriteString("]")
	return sb.String()
}

var webhooks []string

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

	text := fmt.Sprintf("## MC告警\n\n- **Trace ID**: %s\n- **工单类型**: %s\n- **节点**: %s\n- **结果**: %s\n- **失败信息**: %s\n- **错误代码**: %s\n- **部署信息**: %s",
		logRequest.TraceID, logRequest.Scene, logRequest.Event, logRequest.Result, logRequest.Message, logRequest.Code, formatMap(logRequest.Labels))

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

func InitReportServer() {
	reportHooks := utils.GetEnv("REPORT_HOOKS", "")

	webhooks = strings.Split(reportHooks, ";")

	http.HandleFunc("/log", logHandler)
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
