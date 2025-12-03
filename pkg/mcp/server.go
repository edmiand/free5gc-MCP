package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1317540161/free5gc-MCP/pkg/control"
)

const protocolVersion = "2025-03-26"

// Server exposes an MCP-compliant JSON-RPC handler on top of the existing REST API.
type Server struct {
	client *control.Free5GCClient
}

func NewServer(c *control.Free5GCClient) *Server {
	return &Server{client: c}
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
		"instructions": "Expose free5GC subscriber and tenant management tools via MCP.",
	}
	return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) handleListTools(req jsonRPCRequest) *jsonRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "tenant_users_get",
			"description": "Get all users for a tenant from free5GC WebUI. Calls GET /api/tenant/:tenantId/user on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tenantId": map[string]string{
						"type":        "string",
						"description": "The tenant ID to retrieve users for",
					},
				},
				"required": []string{"tenantId"},
			},
		},
		// Subscriber Management Tools
		{
			"name":        "subscriber_list",
			"description": "Get all subscribers from free5GC WebUI. Calls GET /api/subscriber on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "subscriber_get",
			"description": "Get a specific subscriber by UE ID and PLMN ID. Calls GET /api/subscriber/:ueId/:servingPlmnId on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The UE ID (IMSI) of the subscriber, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
				},
				"required": []string{"ueId", "servingPlmnId"},
			},
		},
		{
			"name":        "subscriber_create",
			"description": "Create a new subscriber. Calls POST /api/subscriber/:ueId/:servingPlmnId on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The UE ID (IMSI) of the subscriber, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
					"subscriberData": map[string]interface{}{
						"type":        "object",
						"description": "The subscriber data object containing all subscriber configuration",
					},
				},
				"required": []string{"ueId", "servingPlmnId", "subscriberData"},
			},
		},
		{
			"name":        "subscriber_create_multiple",
			"description": "Create multiple subscribers at once. Calls POST /api/subscriber/:ueId/:servingPlmnId/:userNumber on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The base UE ID (IMSI) of the first subscriber, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
					"userNumber": map[string]interface{}{
						"type":        "integer",
						"description": "The number of subscribers to create",
					},
					"subscriberData": map[string]interface{}{
						"type":        "object",
						"description": "The subscriber data template for all new subscribers",
					},
				},
				"required": []string{"ueId", "servingPlmnId", "userNumber", "subscriberData"},
			},
		},
		{
			"name":        "subscriber_update",
			"description": "Update a subscriber (full replacement). Calls PUT /api/subscriber/:ueId/:servingPlmnId on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The UE ID (IMSI) of the subscriber, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
					"subscriberData": map[string]interface{}{
						"type":        "object",
						"description": "The complete subscriber data object to replace existing data",
					},
				},
				"required": []string{"ueId", "servingPlmnId", "subscriberData"},
			},
		},
		{
			"name":        "subscriber_patch",
			"description": "Partially update a subscriber. Calls PATCH /api/subscriber/:ueId/:servingPlmnId on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The UE ID (IMSI) of the subscriber, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
					"patchData": map[string]interface{}{
						"type":        "object",
						"description": "The partial subscriber data to update (only fields to modify)",
					},
				},
				"required": []string{"ueId", "servingPlmnId", "patchData"},
			},
		},
		{
			"name":        "subscriber_delete",
			"description": "Delete a specific subscriber. Calls DELETE /api/subscriber/:ueId/:servingPlmnId on the webconsole backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ueId": map[string]string{
						"type":        "string",
						"description": "The UE ID (IMSI) of the subscriber to delete, e.g., imsi-208930000000001",
					},
					"servingPlmnId": map[string]string{
						"type":        "string",
						"description": "The serving PLMN ID, e.g., 20893",
					},
				},
				"required": []string{"ueId", "servingPlmnId"},
			},
		},
		{
			"name":        "subscriber_delete_multiple",
			"description": "Delete multiple subscribers at once. Calls DELETE /api/subscriber on the webconsole backend with a list of subscribers.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"subscribers": map[string]interface{}{
						"type":        "array",
						"description": "Array of subscriber identifiers to delete, each with ueId and servingPlmnId",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"ueId":          map[string]string{"type": "string"},
								"servingPlmnId": map[string]string{"type": "string"},
							},
						},
					},
				},
				"required": []string{"subscribers"},
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
	case "tenant_users_get":
		return s.callTenantUsersGet(req.ID, params.Arguments)
	case "subscriber_list":
		return s.callSubscriberList(req.ID)
	case "subscriber_get":
		return s.callSubscriberGet(req.ID, params.Arguments)
	case "subscriber_create":
		return s.callSubscriberCreate(req.ID, params.Arguments)
	case "subscriber_create_multiple":
		return s.callSubscriberCreateMultiple(req.ID, params.Arguments)
	case "subscriber_update":
		return s.callSubscriberUpdate(req.ID, params.Arguments)
	case "subscriber_patch":
		return s.callSubscriberPatch(req.ID, params.Arguments)
	case "subscriber_delete":
		return s.callSubscriberDelete(req.ID, params.Arguments)
	case "subscriber_delete_multiple":
		return s.callSubscriberDeleteMultiple(req.ID, params.Arguments)
	default:
		return s.errorResponse(req.ID, -32601, "unknown tool", params.Name)
	}
}

func (s *Server) callTenantUsersGet(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	tenantId, _ := args["tenantId"].(string)
	if tenantId == "" {
		return s.errorResponse(id, -32602, "missing tenantId parameter", nil)
	}

	resp, err := s.client.GetTenantUsers(tenantId)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberList(id interface{}) *jsonRPCResponse {
	resp, err := s.client.GetSubscribers()
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberGet(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}

	resp, err := s.client.GetSubscriberByID(ueId, servingPlmnId)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberCreate(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)
	subscriberData, _ := args["subscriberData"].(map[string]interface{})

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}
	if subscriberData == nil {
		return s.errorResponse(id, -32602, "missing subscriberData parameter", nil)
	}

	body, err := json.Marshal(subscriberData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscriberData", err.Error())
	}

	resp, err := s.client.CreateSubscriber(ueId, servingPlmnId, bytes.NewReader(body))
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberCreateMultiple(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)
	userNumberFloat, _ := args["userNumber"].(float64)
	userNumber := int(userNumberFloat)
	subscriberData, _ := args["subscriberData"].(map[string]interface{})

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}
	if userNumber <= 0 {
		return s.errorResponse(id, -32602, "userNumber must be a positive integer", nil)
	}
	if subscriberData == nil {
		return s.errorResponse(id, -32602, "missing subscriberData parameter", nil)
	}

	body, err := json.Marshal(subscriberData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscriberData", err.Error())
	}

	resp, err := s.client.CreateMultipleSubscribers(ueId, servingPlmnId, userNumber, bytes.NewReader(body))
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberUpdate(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)
	subscriberData, _ := args["subscriberData"].(map[string]interface{})

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}
	if subscriberData == nil {
		return s.errorResponse(id, -32602, "missing subscriberData parameter", nil)
	}

	body, err := json.Marshal(subscriberData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscriberData", err.Error())
	}

	resp, err := s.client.UpdateSubscriber(ueId, servingPlmnId, bytes.NewReader(body))
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberPatch(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)
	patchData, _ := args["patchData"].(map[string]interface{})

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}
	if patchData == nil {
		return s.errorResponse(id, -32602, "missing patchData parameter", nil)
	}

	body, err := json.Marshal(patchData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal patchData", err.Error())
	}

	resp, err := s.client.PatchSubscriber(ueId, servingPlmnId, bytes.NewReader(body))
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberDelete(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	ueId, _ := args["ueId"].(string)
	servingPlmnId, _ := args["servingPlmnId"].(string)

	if ueId == "" {
		return s.errorResponse(id, -32602, "missing ueId parameter", nil)
	}
	if servingPlmnId == "" {
		return s.errorResponse(id, -32602, "missing servingPlmnId parameter", nil)
	}

	resp, err := s.client.DeleteSubscriber(ueId, servingPlmnId)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

func (s *Server) callSubscriberDeleteMultiple(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	subscribers, _ := args["subscribers"].([]interface{})

	if subscribers == nil || len(subscribers) == 0 {
		return s.errorResponse(id, -32602, "missing or empty subscribers array", nil)
	}

	// Transform the input to match the webconsole API format
	// The API expects: [{"plmnID": "...", "ueId": "..."}]
	transformed := make([]map[string]string, 0, len(subscribers))
	for _, sub := range subscribers {
		subMap, ok := sub.(map[string]interface{})
		if !ok {
			continue
		}
		ueId, _ := subMap["ueId"].(string)
		// Accept both servingPlmnId and plmnID
		plmnId, _ := subMap["servingPlmnId"].(string)
		if plmnId == "" {
			plmnId, _ = subMap["plmnID"].(string)
		}
		if ueId != "" && plmnId != "" {
			transformed = append(transformed, map[string]string{
				"plmnID": plmnId,
				"ueId":   ueId,
			})
		}
	}

	if len(transformed) == 0 {
		return s.errorResponse(id, -32602, "no valid subscribers to delete", nil)
	}

	body, err := json.Marshal(transformed)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscribers", err.Error())
	}

	resp, err := s.client.DeleteMultipleSubscribers(bytes.NewReader(body))
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
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

	for {
		select {
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": ping %d\n\n", time.Now().Unix())
			flusher.Flush()
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
