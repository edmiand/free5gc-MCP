package timeconv

import (
	"fmt"
	"time"
)

// Request represents the expected input for the convert_time tool.
type Request struct {
	Time         string `json:"time"`
	From         string `json:"from"`
	To           string `json:"to"`
	Layout       string `json:"layout"`
	OutputLayout string `json:"output_layout"`
}

// Response captures the structured output for a converted time operation.
type Response struct {
	Input struct {
		Time   string `json:"time"`
		Layout string `json:"layout"`
		Zone   string `json:"zone"`
	} `json:"input"`
	Result struct {
		Formatted     string `json:"formatted"`
		RFC3339       string `json:"rfc3339"`
		Unix          int64  `json:"unix"`
		UnixMillis    int64  `json:"unix_millis"`
		Zone          string `json:"zone"`
		OffsetSeconds int    `json:"offset_seconds"`
	} `json:"result"`
	Metadata struct {
		FromOffsetSeconds int `json:"from_offset_seconds"`
		ToOffsetSeconds   int `json:"to_offset_seconds"`
	} `json:"metadata"`
}

// Convert executes the timezone conversion according to the supplied request.
func Convert(req Request) (*Response, error) {
	layout := req.Layout
	if layout == "" {
		layout = time.RFC3339
	}
	outputLayout := req.OutputLayout
	if outputLayout == "" {
		outputLayout = layout
	}
	fromZone := req.From
	if fromZone == "" {
		fromZone = "UTC"
	}
	toZone := req.To
	if toZone == "" {
		toZone = "UTC"
	}

	fromLoc, err := time.LoadLocation(fromZone)
	if err != nil {
		return nil, fmt.Errorf("invalid source timezone: %w", err)
	}
	toLoc, err := time.LoadLocation(toZone)
	if err != nil {
		return nil, fmt.Errorf("invalid target timezone: %w", err)
	}

	parsed, err := time.ParseInLocation(layout, req.Time, fromLoc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse time: %w", err)
	}

	converted := parsed.In(toLoc)
	_, fromOffset := parsed.Zone()
	zoneName, toOffset := converted.Zone()

	resp := &Response{}
	resp.Input.Time = req.Time
	resp.Input.Layout = layout
	resp.Input.Zone = fromZone
	resp.Result.Formatted = converted.Format(outputLayout)
	resp.Result.RFC3339 = converted.Format(time.RFC3339)
	resp.Result.Unix = converted.Unix()
	resp.Result.UnixMillis = converted.UnixMilli()
	resp.Result.Zone = zoneName
	resp.Result.OffsetSeconds = toOffset
	resp.Metadata.FromOffsetSeconds = fromOffset
	resp.Metadata.ToOffsetSeconds = toOffset

	return resp, nil
}
