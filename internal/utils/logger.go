package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	InfoLevel  string = "INFO"
	ErrorLevel string = "ERROR"
)

type CommonLogger struct {
	ModuleName string
	TraceId    string
}

func NewFromContext(moduleName string) *CommonLogger {
	ctx := context.WithValue(context.Background(), "TraceId", strconv.FormatInt(time.Now().UnixNano(), 10))
	traceId := ctx.Value("TraceId").(string)
	return &CommonLogger{ModuleName: moduleName, TraceId: traceId}
}

func (l *CommonLogger) printLog(kvList []map[string]interface{}, message, moduleName, level string, logType int) {
	filePath := moduleName + "_" + level + ".printLog"
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	fmtLog := getFormatLog(kvList, logType)
	fmtLog = fmt.Sprintf("%s %s\n", l.TraceId, fmtLog+message)

	fmt.Println(fmtLog)
	_, err = fmt.Fprintf(file, fmtLog)
	if err != nil {
		panic(err)
	}
}

func getFormatLog(kvList []map[string]interface{}, logType int) string {
	var fmtMsg string
	jsonData := map[string]interface{}{}
	for _, kv := range kvList {
		for k, v := range kv {
			switch logType {
			case 0:
				fmtMsg += fmt.Sprintf("%v, ", v)
			case 1:
				fmtMsg += fmt.Sprintf("[%s: %v], ", k, v)
			case 2:
				jsonData[k] = v
			}
		}
	}

	if logType == 2 {
		jsonString, err := json.Marshal(jsonData)
		if err != nil {
			panic(err)
		}
		fmtMsg = string(jsonString)
	}
	return fmtMsg
}

func (l *CommonLogger) Info(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, InfoLevel, 0)
}

func (l *CommonLogger) InfoKV(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, InfoLevel, 1)
}

func (l *CommonLogger) InfoJSON(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, InfoLevel, 2)
}

func (l *CommonLogger) Error(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, ErrorLevel, 0)
}

func (l *CommonLogger) ErrorKV(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, ErrorLevel, 1)
}

func (l *CommonLogger) ErrorJSON(kvList []map[string]interface{}, message string) {
	l.printLog(kvList, message, l.ModuleName, ErrorLevel, 2)

}
