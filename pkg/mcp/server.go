package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Gthulhu/free5gc-MCP/pkg/tools/timeconv"
	"github.com/gin-gonic/gin"
)

const protocolVersion = "2025-03-26"

// Server exposes an MCP-compliant JSON-RPC handler on top of the existing REST API.
type Server struct{}

func NewServer() *Server {
	return &Server{}
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (s *Server) HandleJSONRPC(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read body"})
		return
	}

	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty request"})
		return
	}

	if body[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(body, &arr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid batch request"})
			return
		}
		responses := make([]jsonRPCResponse, 0, len(arr))
		for _, raw := range arr {
			resp := s.handleSingle(raw)
			if resp != nil {
				responses = append(responses, *resp)
			}
		}
		if len(responses) == 0 {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusOK, responses)
		return
	}

	resp := s.handleSingle(body)
	if resp == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleSingle(raw json.RawMessage) *jsonRPCResponse {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &jsonRPCError{Code: -32700, Message: "parse error", Data: err.Error()},
		}
	}

	// Notifications have no ID; process and return nil response.
	if req.ID == nil {
		s.handleNotification(req)
		return nil
	}

	if req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, -32600, "invalid jsonrpc version", nil)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "shutdown":
		return s.emptyResult(req.ID)
	case "ping":
		return s.emptyResult(req.ID)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	default:
		return s.errorResponse(req.ID, -32601, "method not found", map[string]string{"method": req.Method})
	}
}

func (s *Server) handleNotification(req jsonRPCRequest) {
	// For now we do not act on notifications beyond acknowledging them.
}

func (s *Server) handleInitialize(req jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)

	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"serverInfo": map[string]string{
			"name":    "free5gc-mcp",
			"version": "0.1.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{"listChanged": false},
		},
		"instructions": "Expose convert_time and free5GC helper tools. Use tools/call convert_time with RFC3339 timestamps.",
	}
	return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) handleListTools(req jsonRPCRequest) *jsonRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "convert_time",
			"description": "Convert timestamps between time zones and formats",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"time":          map[string]string{"type": "string", "description": "Timestamp string"},
					"from":          map[string]string{"type": "string", "description": "Source IANA timezone"},
					"to":            map[string]string{"type": "string", "description": "Target IANA timezone"},
					"layout":        map[string]string{"type": "string", "description": "Input Go layout (default RFC3339)"},
					"output_layout": map[string]string{"type": "string", "description": "Output Go layout (default same as input)"},
				},
				"required": []string{"time"},
			},
			"annotations": map[string]interface{}{
				"title":        "Convert Time",
				"readOnlyHint": true,
			},
		},
	}
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleCallTool(req jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "invalid params", err.Error())
	}

	switch params.Name {
	case "convert_time":
		return s.callConvertTime(req.ID, params.Arguments)
	default:
		return s.errorResponse(req.ID, -32601, "unknown tool", params.Name)
	}
}

func (s *Server) callConvertTime(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	argBytes, err := json.Marshal(args)
	if err != nil {
		return s.errorResponse(id, -32602, "invalid arguments", err.Error())
	}
	var req timeconv.Request
	if err := json.Unmarshal(argBytes, &req); err != nil {
		return s.errorResponse(id, -32602, "invalid arguments", err.Error())
	}
	resp, err := timeconv.Convert(req)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": fmt.Sprintf("convert_time error: %v", err)}},
				"isError": true,
			},
		}
	}
	out, _ := json.Marshal(resp)
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(out)}},
		},
	}
}

func (s *Server) HandleSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Send initial comment to establish the stream.
	fmt.Fprintf(c.Writer, ": connected\n\n")
	flusher.Flush()

	notify := c.Writer.CloseNotify()
	for {
		select {
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": ping %d\n\n", time.Now().Unix())
			flusher.Flush()
		case <-notify:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (s *Server) emptyResult(id interface{}) *jsonRPCResponse {
	return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{}}
}

func (s *Server) errorResponse(id interface{}, code int, msg string, data interface{}) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	}
}
