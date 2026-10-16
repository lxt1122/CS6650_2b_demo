package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// product represents a product as defined by the Product schema in api.yaml.
// Weight is a pointer so that a missing field can be told apart from 0,
// which is a valid weight (minimum: 0).
type product struct {
	ProductID    int32  `json:"product_id" binding:"required,min=1"`
	SKU          string `json:"sku" binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer" binding:"required,min=1,max=200"`
	CategoryID   int32  `json:"category_id" binding:"required,min=1"`
	Weight       *int32 `json:"weight" binding:"required,min=0"`
	SomeOtherID  int32  `json:"some_other_id" binding:"required,min=1"`
}

// errorResponse represents the Error schema in api.yaml.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// productStore keeps products in memory, keyed by product ID.
// The RWMutex lets many GETs read at once while a POST writes alone.
type productStore struct {
	mu       sync.RWMutex
	products map[int32]product
}

// seedCount is how many products exist when the server starts.
// POST can only update these; any other ID returns 404.
const seedCount = 1000

var store = newProductStore(seedCount)

// newProductStore creates a store pre-filled with n sample products (IDs 1..n).
func newProductStore(n int32) *productStore {
	s := &productStore{products: make(map[int32]product, n)}
	for id := int32(1); id <= n; id++ {
		weight := id * 10
		s.products[id] = product{
			ProductID:    id,
			SKU:          fmt.Sprintf("SKU-%05d", id),
			Manufacturer: "Acme Corporation",
			CategoryID:   id%50 + 1,
			Weight:       &weight,
			SomeOtherID:  id,
		}
	}
	return s
}

func (s *productStore) get(id int32) (product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

// update replaces an existing product and reports whether it existed.
func (s *productStore) update(p product) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[p.ProductID]; !ok {
		return false
	}
	s.products[p.ProductID] = p
	return true
}

// main registers the product routes and starts the server on port 8080.
// Every request runs Logger and Recovery first, then simulateError, then its handler.
func main() {
	router := gin.New()                                            // empty router, no default middleware
	router.Use(gin.Logger(), gin.CustomRecovery(recoverWithError)) // global middleware
	products := router.Group("/products", simulateError)           // group middleware
	products.GET("/:productId", getProduct)
	products.POST("/:productId/details", addProductDetails)
	router.Run(":8080")

}

// getProduct responds with the product whose ID matches the productId parameter.
func getProduct(c *gin.Context) {
	id, ok := parseProductID(c.Param("productId")) // turns id to int32
	if !ok {
		// GET has no 400 in api.yaml, so an invalid ID is treated as not found.
		c.JSON(http.StatusNotFound, errorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: "Product ID must be a positive 32-bit integer",
		})
		return
	}

	p, found := store.get(id)
	if !found {
		c.JSON(http.StatusNotFound, errorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("No product with ID %d", id),
		})
		return
	}
	c.JSON(http.StatusOK, p)
}

// addProductDetails updates an existing product from the JSON request body.
func addProductDetails(c *gin.Context) {
	id, ok := parseProductID(c.Param("productId"))
	if !ok {
		c.JSON(http.StatusBadRequest, errorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "Product ID must be a positive 32-bit integer",
		})
		return
	}

	// ShouldBindJSON (not BindJSON) so we write the 400 body ourselves
	// in the Error schema. It covers malformed JSON, wrong types,
	// int32 overflow, and the binding rules on the product struct.
	var p product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: err.Error(),
		})
		return
	}

	if p.ProductID != id {
		c.JSON(http.StatusBadRequest, errorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: fmt.Sprintf("product_id %d in body does not match %d in path", p.ProductID, id),
		})
		return
	}

	if !store.update(p) {
		c.JSON(http.StatusNotFound, errorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("No product with ID %d", id),
		})
		return
	}
	c.Status(http.StatusNoContent)
}

// parseProductID converts the path parameter to an int32 that is at least 1.
func parseProductID(raw string) (int32, bool) {
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id < 1 {
		return 0, false
	}
	return int32(id), true
}

// simulateError returns 500 when the request carries X-Simulate-Error: true,
// so the 500 response defined in api.yaml can be demonstrated on demand.
func simulateError(c *gin.Context) {
	if c.GetHeader("X-Simulate-Error") == "true" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "Internal server error",
			Details: "Simulated failure triggered by X-Simulate-Error header",
		})
		return
	}
	c.Next()
}

// recoverWithError turns a panic in any handler into a 500 in the Error schema.
func recoverWithError(c *gin.Context, err any) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{
		Error:   "INTERNAL_ERROR",
		Message: "Internal server error",
	})
}
