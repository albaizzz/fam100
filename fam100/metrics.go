package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/uber-go/zap"
)

func postEvent(what, tags, data string) error {
	if graphiteWebURL == "" {
		return nil
	}

	payload := struct {
		What string `json:"what"`
		Tags string `json:"tags"`
		Data string `json:"data"`
	}{what, tags, data}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)
	_, err := http.DefaultClient.Post(graphiteWebURL+"/events/", "application/json", &buf)
	if err != nil {
		log.Error("failed sending event to graphite", zap.Error(err))
	}

	return err
}
