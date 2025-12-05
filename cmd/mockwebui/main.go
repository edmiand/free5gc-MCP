package main

import (
    "net/http"
    "sync"
    "github.com/gin-gonic/gin"
)

type subscriber struct {
    ID   string                 `json:"id"`
    Data map[string]interface{} `json:"data"`
}

type store struct {
    mu    sync.RWMutex
    items map[string]subscriber
}

func newStore() *store {
    return &store{items: make(map[string]subscriber)}
}

func (s *store) list(c *gin.Context) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]subscriber, 0, len(s.items))
    for _, v := range s.items {
        out = append(out, v)
    }
    c.JSON(http.StatusOK, out)
}

func (s *store) create(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    id := ueId + "-" + servingPlmnId
    
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    
    s.mu.Lock()
    s.items[id] = subscriber{ID: id, Data: body}
    s.mu.Unlock()
    c.JSON(http.StatusCreated, gin.H{"ueId": ueId, "servingPlmnId": servingPlmnId, "data": body})
}

func (s *store) get(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    id := ueId + "-" + servingPlmnId
    
    s.mu.RLock()
    defer s.mu.RUnlock()
    if v, ok := s.items[id]; ok {
        c.JSON(http.StatusOK, v)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *store) update(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    id := ueId + "-" + servingPlmnId
    
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if v, ok := s.items[id]; ok {
        // PUT replaces entire data
        v.Data = body
        s.items[id] = v
        c.JSON(http.StatusOK, v)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *store) patch(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    id := ueId + "-" + servingPlmnId
    
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if v, ok := s.items[id]; ok {
        // PATCH merges into existing data
        if v.Data == nil { v.Data = map[string]interface{}{} }
        for k, val := range body {
            v.Data[k] = val
        }
        s.items[id] = v
        c.JSON(http.StatusOK, v)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *store) delete(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    id := ueId + "-" + servingPlmnId
    
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.items[id]; ok {
        delete(s.items, id)
        c.Status(http.StatusNoContent)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *store) createMultiple(c *gin.Context) {
    ueId := c.Param("ueId")
    servingPlmnId := c.Param("servingPlmnId")
    userNumber := c.Param("userNumber")
    
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "message": "created multiple subscribers",
        "ueId": ueId,
        "servingPlmnId": servingPlmnId,
        "count": userNumber,
    })
}

func (s *store) deleteMultiple(c *gin.Context) {
    var body []map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    deleted := 0
    for _, item := range body {
        ueId, _ := item["ueId"].(string)
        plmnId, _ := item["servingPlmnId"].(string)
        if ueId != "" && plmnId != "" {
            id := ueId + "-" + plmnId
            if _, ok := s.items[id]; ok {
                delete(s.items, id)
                deleted++
            }
        }
    }
    
    c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func randomID() string {
    // very small pseudo id for test
    const letters = "0123456789abcdef"
    b := make([]byte, 16)
    for i := range b { b[i] = letters[i%len(letters)] }
    return string(b)
}

func main() {
    r := gin.Default()
    s := newStore()

    api := r.Group("/api")
    
    // Login endpoint for authentication
    api.POST("/login", func(c *gin.Context) {
        var body map[string]interface{}
        if err := c.ShouldBindJSON(&body); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
            return
        }
        // Mock login always succeeds and returns a fake token
        c.JSON(http.StatusOK, gin.H{"access_token": "mock-token-12345"})
    })
    
    // Tenant users endpoint
    api.GET("/tenant/:tenantId/user", func(c *gin.Context) {
        tenantId := c.Param("tenantId")
        c.JSON(http.StatusOK, gin.H{"tenantId": tenantId, "users": []interface{}{}})
    })
    
    // Subscriber endpoints matching free5GC WebUI API
    api.GET("/subscriber", s.list)
    api.GET("/subscriber/:ueId/:servingPlmnId", s.get)
    api.POST("/subscriber/:ueId/:servingPlmnId", s.create)
    api.POST("/subscriber/:ueId/:servingPlmnId/:userNumber", s.createMultiple)
    api.PUT("/subscriber/:ueId/:servingPlmnId", s.update)
    api.PATCH("/subscriber/:ueId/:servingPlmnId", s.patch)
    api.DELETE("/subscriber/:ueId/:servingPlmnId", s.delete)
    api.DELETE("/subscriber", s.deleteMultiple)

    r.Run(":5050")
}
