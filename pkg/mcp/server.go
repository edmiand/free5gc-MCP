package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	return &Server{
		client: c,
	}
}

// getDefaultSubscriberData returns the default subscriber data template for creating new subscribers.
// This template is based on the free5GC default subscriber configuration.
// The ueId and servingPlmnId will be populated with the provided values.
func getDefaultSubscriberData(ueId, servingPlmnId string) map[string]interface{} {
	return map[string]interface{}{
		"plmnID": servingPlmnId,
		"ueId":   ueId,
		"AuthenticationSubscription": map[string]interface{}{
			"authenticationManagementField": "8000",
			"authenticationMethod":          "5G_AKA",
			"milenage": map[string]interface{}{
				"op": map[string]interface{}{
					"encryptionAlgorithm": 0,
					"encryptionKey":       0,
					"opValue":             "",
				},
			},
			"opc": map[string]interface{}{
				"encryptionAlgorithm": 0,
				"encryptionKey":       0,
				"opcValue":            "8e27b6af0e692e750f32667a3b14605d",
			},
			"permanentKey": map[string]interface{}{
				"encryptionAlgorithm": 0,
				"encryptionKey":       0,
				"permanentKeyValue":   "8baf473f2f8fd09487cccbd7097c6862",
			},
			"sequenceNumber": "000000000000",
		},
		"AccessAndMobilitySubscriptionData": map[string]interface{}{
			"gpsis": []string{"msisdn-"},
			"nssai": map[string]interface{}{
				"defaultSingleNssais": []map[string]interface{}{
					{"sst": 1, "sd": "010203"},
				},
				"singleNssais": []map[string]interface{}{
					{"sst": 1, "sd": "112233"},
				},
			},
			"subscribedUeAmbr": map[string]interface{}{
				"uplink":   "1 Gbps",
				"downlink": "2 Gbps",
			},
		},
		"SessionManagementSubscriptionData": []map[string]interface{}{
			{
				"singleNssai": map[string]interface{}{"sst": 1, "sd": "010203"},
				"dnnConfigurations": map[string]interface{}{
					"internet": map[string]interface{}{
						"pduSessionTypes": map[string]interface{}{
							"defaultSessionType":  "IPV4",
							"allowedSessionTypes": []string{"IPV4"},
						},
						"sscModes": map[string]interface{}{
							"defaultSscMode":  "SSC_MODE_1",
							"allowedSscModes": []string{"SSC_MODE_2", "SSC_MODE_3"},
						},
						"5gQosProfile": map[string]interface{}{
							"5qi":           9,
							"priorityLevel": 8,
							"arp": map[string]interface{}{
								"priorityLevel": 8,
								"preemptCap":    "",
								"preemptVuln":   "",
							},
						},
						"sessionAmbr": map[string]interface{}{
							"uplink":   "1000 Mbps",
							"downlink": "1000 Mbps",
						},
					},
				},
			},
			{
				"singleNssai": map[string]interface{}{"sst": 1, "sd": "112233"},
				"dnnConfigurations": map[string]interface{}{
					"internet": map[string]interface{}{
						"pduSessionTypes": map[string]interface{}{
							"defaultSessionType":  "IPV4",
							"allowedSessionTypes": []string{"IPV4"},
						},
						"sscModes": map[string]interface{}{
							"defaultSscMode":  "SSC_MODE_1",
							"allowedSscModes": []string{"SSC_MODE_2", "SSC_MODE_3"},
						},
						"5gQosProfile": map[string]interface{}{
							"5qi":           8,
							"priorityLevel": 8,
							"arp": map[string]interface{}{
								"priorityLevel": 8,
								"preemptCap":    "",
								"preemptVuln":   "",
							},
						},
						"sessionAmbr": map[string]interface{}{
							"uplink":   "1000 Mbps",
							"downlink": "1000 Mbps",
						},
					},
				},
			},
		},
		"SmfSelectionSubscriptionData": map[string]interface{}{
			"subscribedSnssaiInfos": map[string]interface{}{
				"01010203": map[string]interface{}{
					"dnnInfos": []map[string]interface{}{
						{"dnn": "internet"},
					},
				},
				"01112233": map[string]interface{}{
					"dnnInfos": []map[string]interface{}{
						{"dnn": "internet"},
					},
				},
			},
		},
		"AmPolicyData": map[string]interface{}{
			"subscCats": []string{"free5gc"},
		},
		"SmPolicyData": map[string]interface{}{
			"smPolicySnssaiData": map[string]interface{}{
				"01010203": map[string]interface{}{
					"snssai": map[string]interface{}{"sst": 1, "sd": "010203"},
					"smPolicyDnnData": map[string]interface{}{
						"internet": map[string]interface{}{"dnn": "internet"},
					},
				},
				"01112233": map[string]interface{}{
					"snssai": map[string]interface{}{"sst": 1, "sd": "112233"},
					"smPolicyDnnData": map[string]interface{}{
						"internet": map[string]interface{}{"dnn": "internet"},
					},
				},
			},
		},
		"FlowRules": []map[string]interface{}{
			{
				"filter":     "1.1.1.1/32",
				"precedence": 128,
				"snssai":     "01010203",
				"dnn":        "internet",
				"qosRef":     1,
			},
			{
				"filter":     "1.1.1.1/32",
				"precedence": 127,
				"snssai":     "01112233",
				"dnn":        "internet",
				"qosRef":     2,
			},
		},
		"QosFlows": []map[string]interface{}{
			{
				"snssai": "01010203",
				"dnn":    "internet",
				"qosRef": 1,
				"5qi":    8,
				"mbrUL":  "208 Mbps",
				"mbrDL":  "208 Mbps",
				"gbrUL":  "108 Mbps",
				"gbrDL":  "108 Mbps",
			},
			{
				"snssai": "01112233",
				"dnn":    "internet",
				"qosRef": 2,
				"5qi":    7,
				"mbrUL":  "407 Mbps",
				"mbrDL":  "407 Mbps",
				"gbrUL":  "207 Mbps",
				"gbrDL":  "207 Mbps",
			},
		},
		"ChargingDatas": []map[string]interface{}{
			{
				"snssai":         "01010203",
				"dnn":            "",
				"filter":         "",
				"chargingMethod": "Offline",
				"quota":          "0",
				"unitCost":       "1",
			},
			{
				"snssai":         "01010203",
				"dnn":            "internet",
				"filter":         "1.1.1.1/32",
				"qosRef":         1,
				"chargingMethod": "Offline",
				"quota":          "0",
				"unitCost":       "1",
			},
			{
				"snssai":         "01112233",
				"dnn":            "",
				"filter":         "",
				"chargingMethod": "Online",
				"quota":          "100000",
				"unitCost":       "1",
			},
			{
				"snssai":         "01112233",
				"dnn":            "internet",
				"filter":         "1.1.1.1/32",
				"qosRef":         2,
				"chargingMethod": "Online",
				"quota":          "5000",
				"unitCost":       "1",
			},
		},
	}
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
			"description": "Create a new subscriber. Calls POST /api/subscriber/:ueId/:servingPlmnId on the webconsole backend. If subscriberData is not provided, default values will be used.",
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
						"description": "The subscriber data object containing all subscriber configuration. If not provided, default values will be used with K=8baf473f2f8fd09487cccbd7097c6862, OPC=8e27b6af0e692e750f32667a3b14605d, and default slice/DNN configurations.",
					},
				},
				"required": []string{"ueId", "servingPlmnId"},
			},
		},
		{
			"name":        "subscriber_create_multiple",
			"description": "Create multiple subscribers at once. Calls POST /api/subscriber/:ueId/:servingPlmnId/:userNumber on the webconsole backend. If subscriberData is not provided, default values will be used.",
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
						"description": "The subscriber data template for all new subscribers. If not provided, default values will be used.",
					},
				},
				"required": []string{"ueId", "servingPlmnId", "userNumber"},
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
		// free5GC Control Tools
		{
			"name":        "local_free5gc_start",
			"description": "Start all free5GC network functions (NRF, AMF, SMF, UPF, etc.) and the webconsole. This executes the run.sh script in the free5GC directory and starts the webconsole.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum time to wait for NFs to start (default: 30 seconds)",
					},
				},
				"required": []string{},
			},
			"annotations": map[string]interface{}{
				"title":           "Start free5GC",
				"readOnlyHint":    false,
				"destructiveHint": false,
			},
		},
		{
			"name":        "local_free5gc_stop",
			"description": "Stop all free5GC network functions and webconsole gracefully. This executes the force_kill.sh script to terminate all NFs (NRF, AMF, SMF, UPF, etc.) and stops the webconsole. Use with caution as this will disconnect all UEs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, force kill without confirmation (default: false)",
					},
				},
				"required": []string{},
			},
			"annotations": map[string]interface{}{
				"title":           "Stop free5GC",
				"readOnlyHint":    false,
				"destructiveHint": true,
			},
		},
		{
			"name":        "local_free5gc_status",
			"description": "Get the current status of all free5GC network functions and webconsole. Returns running/stopped status for each NF (NRF, AMF, SMF, UDR, PCF, UDM, NSSF, AUSF, UPF, CHF, NEF) and the webconsole.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
			"annotations": map[string]interface{}{
				"title":        "Get free5GC Status",
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
	case "local_free5gc_start":
		return s.callFree5GCStart(req.ID, params.Arguments)
	case "local_free5gc_stop":
		return s.callFree5GCStop(req.ID, params.Arguments)
	case "local_free5gc_status":
		return s.callFree5GCStatus(req.ID)
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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
	// Use default subscriber data if not provided
	if subscriberData == nil {
		subscriberData = getDefaultSubscriberData(ueId, servingPlmnId)
	}

	body, err := json.Marshal(subscriberData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscriberData", err.Error())
	}

	resp, err := s.client.CreateSubscriber(ueId, servingPlmnId, body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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
	// Use default subscriber data if not provided
	if subscriberData == nil {
		subscriberData = getDefaultSubscriberData(ueId, servingPlmnId)
	}

	body, err := json.Marshal(subscriberData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal subscriberData", err.Error())
	}

	resp, err := s.client.CreateMultipleSubscribers(ueId, servingPlmnId, userNumber, body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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

	resp, err := s.client.UpdateSubscriber(ueId, servingPlmnId, body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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

	// First, GET the current subscriber data
	getResp, err := s.client.GetSubscriberByID(ueId, servingPlmnId)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to get current subscriber data", err.Error())
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		return s.errorResponse(id, -32001, fmt.Sprintf("failed to get subscriber (status %d)", getResp.StatusCode), string(b))
	}

	// Decode current subscriber data
	var currentData map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&currentData); err != nil {
		return s.errorResponse(id, -32001, "failed to decode current subscriber data", err.Error())
	}

	// Deep merge patchData into currentData
	mergeMaps(currentData, patchData)

	// Marshal the merged data
	body, err := json.Marshal(currentData)
	if err != nil {
		return s.errorResponse(id, -32602, "failed to marshal merged data", err.Error())
	}

	// Debug: log the body being sent
	fmt.Printf("PATCH body being sent: %s\n", string(body))

	// Send PATCH request with complete merged data
	resp, err := s.client.PatchSubscriber(ueId, servingPlmnId, body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

// mergeMaps recursively merges src into dst
func mergeMaps(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// If both are maps, merge recursively
			if dstMap, dstOk := dstVal.(map[string]interface{}); dstOk {
				if srcMap, srcOk := srcVal.(map[string]interface{}); srcOk {
					mergeMaps(dstMap, srcMap)
					continue
				}
			}
		}
		// Otherwise, overwrite with src value
		dst[key] = srcVal
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

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

	resp, err := s.client.DeleteMultipleSubscribers(body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to call webconsole backend", err.Error())
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.errorResponse(id, -32001, "failed to read response body", err.Error())
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"status":  resp.Status,
		},
	}
}

// ============================================
// Core Control Tool Handlers
// ============================================

func (s *Server) callFree5GCStatus(id interface{}) *jsonRPCResponse {
	status, err := s.client.GetFree5GCStatus()
	if err != nil {
		return s.errorResponse(id, -32001, "failed to get free5gc status", err.Error())
	}

	// Build a user-friendly status message
	var statusText strings.Builder
	statusText.WriteString(fmt.Sprintf("## free5GC Status: %s\n\n", strings.ToUpper(status.Overall)))
	statusText.WriteString(fmt.Sprintf("**Summary:** %s\n\n", status.Message))
	
	if status.StartTime != "" {
		statusText.WriteString(fmt.Sprintf("**Started:** %s\n\n", status.StartTime))
	}
	
	statusText.WriteString("### Network Functions\n\n")
	statusText.WriteString("| NF | Status | Address |\n")
	statusText.WriteString("|:---|:------:|:--------|\n")
	
	for _, nf := range status.NFs {
		statusIcon := "🔴 Stopped"
		if nf.Running {
			statusIcon = "🟢 Running"
		}
		addrStr := fmt.Sprintf("%s:%d", nf.IP, nf.Port)
		statusText.WriteString(fmt.Sprintf("| %s | %s | %s |\n", 
			strings.ToUpper(nf.Name), statusIcon, addrStr))
	}
	
	// Webconsole status
	statusText.WriteString("\n### Webconsole\n\n")
	wcStatus := "🔴 Stopped"
	wcAddr := fmt.Sprintf("%s:%d", status.Webconsole.IP, status.Webconsole.Port)
	if status.Webconsole.Running {
		wcStatus = fmt.Sprintf("🟢 Running (PID: %d, Address: %s)", status.Webconsole.PID, wcAddr)
	} else {
		wcStatus = fmt.Sprintf("🔴 Stopped (Address: %s when running)", wcAddr)
	}
	statusText.WriteString(fmt.Sprintf("**Status:** %s\n", wcStatus))

	// Also return structured data
	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return s.errorResponse(id, -32002, "failed to marshal status to JSON", err.Error())
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": statusText.String()},
			},
			"structuredData": status,
			"rawJSON":        string(statusJSON),
		},
	}
}

func (s *Server) callFree5GCStart(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	// Get timeout from args, default to 30 seconds
	timeoutSec := 30
	if t, ok := args["timeout_seconds"].(float64); ok && t > 0 {
		timeoutSec = int(t)
		// Enforce reasonable bounds
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		if timeoutSec > 300 { // Max 5 minutes
			timeoutSec = 300
		}
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	
	result, err := s.client.StartFree5GC(ctx)
	cancel()
	if err != nil && result == nil {
		return s.errorResponse(id, -32001, "failed to start free5gc", err.Error())
	}

	// Build user-friendly response
	var responseText strings.Builder
	
	if result.Success {
		responseText.WriteString("## ✅ free5GC Started Successfully\n\n")
	} else {
		responseText.WriteString("## ⚠️ free5GC Start - Attention Needed\n\n")
	}
	
	responseText.WriteString(fmt.Sprintf("**Result:** %s\n\n", result.Message))
	
	if len(result.Details) > 0 {
		responseText.WriteString("### Details\n")
		for _, detail := range result.Details {
			responseText.WriteString(fmt.Sprintf("- %s\n", detail))
		}
		responseText.WriteString("\n")
	}
	
	if len(result.Warnings) > 0 {
		responseText.WriteString("### ⚠️ Warnings\n")
		for _, warning := range result.Warnings {
			responseText.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		responseText.WriteString("\n")
	}
	
	if len(result.NFs) > 0 {
		responseText.WriteString("### Network Function Status\n\n")
		responseText.WriteString("| NF | Status | Address |\n")
		responseText.WriteString("|:---|:------:|:--------|\n")
		
		for _, nf := range result.NFs {
			statusIcon := "🔴"
			if nf.Running {
				statusIcon = "🟢"
			}
			addrStr := fmt.Sprintf("%s:%d", nf.IP, nf.Port)
			responseText.WriteString(fmt.Sprintf("| %s | %s | %s |\n", 
				strings.ToUpper(nf.Name), statusIcon, addrStr))
		}
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling result to JSON: %v\n", err)
		resultJSON = []byte("{}")
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": responseText.String()},
			},
			"success":        result.Success,
			"structuredData": result,
			"rawJSON":        string(resultJSON),
		},
	}
}

func (s *Server) callFree5GCStop(id interface{}, args map[string]interface{}) *jsonRPCResponse {
	// Check for force flag
	force, _ := args["force"].(bool)
	
	// If not forced, get current status first to show what will be stopped
	if !force {
		status, _ := s.client.GetFree5GCStatus()
		if status != nil && status.Overall == "stopped" && !status.Webconsole.Running {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: map[string]interface{}{
					"content": []map[string]string{
						{"type": "text", "text": "## ℹ️ free5GC Already Stopped\n\nAll network functions and webconsole are already inactive. No action needed."},
					},
					"success": true,
				},
			}
		}
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	
	result, err := s.client.StopFree5GC(ctx)
	cancel()
	if err != nil && result == nil {
		return s.errorResponse(id, -32001, "failed to stop free5gc", err.Error())
	}

	// Build user-friendly response
	var responseText strings.Builder
	
	if result.Success {
		responseText.WriteString("## ✅ free5GC Stopped Successfully\n\n")
	} else {
		responseText.WriteString("## ❌ free5GC Stop - Issues Encountered\n\n")
	}
	
	responseText.WriteString(fmt.Sprintf("**Result:** %s\n\n", result.Message))
	
	if len(result.Details) > 0 {
		responseText.WriteString("### Details\n")
		for _, detail := range result.Details {
			responseText.WriteString(fmt.Sprintf("- %s\n", detail))
		}
		responseText.WriteString("\n")
	}
	
	if len(result.Warnings) > 0 {
		responseText.WriteString("### ⚠️ Warnings\n")
		for _, warning := range result.Warnings {
			responseText.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		responseText.WriteString("\n")
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return s.errorResponse(id, -32002, "failed to marshal stop result to JSON", err.Error())
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": responseText.String()},
			},
			"success":        result.Success,
			"structuredData": result,
			"rawJSON":        string(resultJSON),
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