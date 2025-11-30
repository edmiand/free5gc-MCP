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
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    id, _ := body["id"].(string)
    if id == "" {
        // simple auto id
        id = randomID()
    }
    s.mu.Lock()
    s.items[id] = subscriber{ID: id, Data: body}
    s.mu.Unlock()
    c.JSON(http.StatusCreated, gin.H{"id": id, "data": body})
}

func (s *store) get(c *gin.Context) {
    id := c.Param("id")
    s.mu.RLock()
    defer s.mu.RUnlock()
    if v, ok := s.items[id]; ok {
        c.JSON(http.StatusOK, v)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *store) update(c *gin.Context) {
    id := c.Param("id")
    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if v, ok := s.items[id]; ok {
        // shallow merge into v.Data
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
    id := c.Param("id")
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.items[id]; ok {
        delete(s.items, id)
        c.Status(http.StatusNoContent)
        return
    }
    c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
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
    subs := api.Group("/subscribers")
    subs.GET("", s.list)
    subs.POST("", s.create)
    subs.GET("/:id", s.get)
    subs.PUT("/:id", s.update)
    subs.DELETE("/:id", s.delete)

    r.Run(":5050")
}
