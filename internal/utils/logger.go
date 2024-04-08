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

func (l *CommonLogger) log(kvList []map[string]interface{}, message, moduleName, level string, logType int) {
	filePath := moduleName + "_" + level + ".log"
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

	jsonData := map[string]interface{}{}
	var fmtMsg string
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

	fmtMsg = fmt.Sprintf("%s %s\n", l.TraceId, fmtMsg+message)
	fmt.Println(fmtMsg)
	_, err = fmt.Fprintf(file, fmtMsg)
	if err != nil {
		panic(err)
	}
}

func (l *CommonLogger) Info(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, InfoLevel, 0)
}

func (l *CommonLogger) InfoKV(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, InfoLevel, 1)
}

func (l *CommonLogger) InfoJSON(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, InfoLevel, 2)
}

func (l *CommonLogger) Error(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, ErrorLevel, 0)
}

func (l *CommonLogger) ErrorKV(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, ErrorLevel, 1)
}

func (l *CommonLogger) ErrorJSON(kvList []map[string]interface{}, message string) {
	l.log(kvList, message, l.ModuleName, ErrorLevel, 2)

}
