package utils

import (
	"testing"
)

func TestCommonLogger_Log(t *testing.T) {
	log := NewFromContext("module_controller")

	kvList := []map[string]interface{}{
		{"hello": "word"},
		{"foo": "bar"},
	}

	log.Info(kvList, "This is an ordered message.")
	log.InfoKV(kvList, "This is a key-value message.")
	log.InfoJSON(kvList, "This is a JSON message.")

	log.Error(kvList, "This is an ordered message.")
	log.ErrorKV(kvList, "This is a key-value message.")
	log.ErrorJSON(kvList, "This is a JSON message.")
}
