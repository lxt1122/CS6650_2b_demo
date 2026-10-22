package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// product represents a product as defined by the Product schema in product-search.yaml.
// Weight is a pointer so that a missing field can be told apart from 0,
// which is a valid weight (minimum: 0).
// Name is not in the schema: the server generates it so search can match it.
type product struct {
	ProductID    int32  `json:"product_id" binding:"required,min=1"`
	SKU          string `json:"sku" binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer" binding:"required,min=1,max=200"`
	CategoryID   int32  `json:"category_id" binding:"required,min=1"`
	Weight       *int32 `json:"weight" binding:"required,min=0"`
	SomeOtherID  int32  `json:"some_other_id" binding:"required,min=1"`
	Name         string `json:"name,omitempty" binding:"omitempty,min=1,max=200"`
}

// searchResponse represents the 200 response of GET /products/search.
type searchResponse struct {
	Products   []product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

// errorResponse represents the Error schema in product-search.yaml.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// productStore keeps products in memory, keyed by product ID (int32 -> product).
// sync.Map fits this workload: every key is written once at startup,
// then read by many goroutines at once.
type productStore struct {
	products sync.Map
}

// seedCount is how many products exist when the server starts.
// POST can only update these; any other ID returns 404.
const seedCount = 100000

// Each search checks exactly maxChecked products, so every request does a
// fixed amount of work, and returns at most maxResults of the matches.
const (
	maxChecked = 100
	maxResults = 20
)

// Sample values for generated products. The pools have different lengths
// so brand and category do not always appear together.
var (
	brands     = []string{"Alpha", "Beta", "Gamma", "Delta", "Omega"}
	categories = []string{"Electronics", "Books", "Home", "Toys", "Sports", "Garden", "Beauty", "Grocery"}
)

var store = newProductStore(seedCount)

// newProductStore creates a store pre-filled with n sample products (IDs 1..n),
// named "Product [Brand] [ID]" with categories rotating by modulo.
func newProductStore(n int32) *productStore {
	s := &productStore{}
	for id := int32(1); id <= n; id++ {
		weight := id * 10
		brand := brands[int(id)%len(brands)]
		s.products.Store(id, product{
			ProductID:    id,
			Name:         fmt.Sprintf("Product %s %d", brand, id),
			SKU:          fmt.Sprintf("SKU-%06d", id),
			Manufacturer: brand,
			CategoryID:   id%int32(len(categories)) + 1,
			Weight:       &weight,
			SomeOtherID:  id,
		})
	}
	return s
}

func (s *productStore) get(id int32) (product, bool) {
	v, ok := s.products.Load(id)
	if !ok {
		return product{}, false
	}
	return v.(product), true
}

// update replaces an existing product and reports whether it existed.
// Name is optional in the request body, so an empty one keeps the existing name.
// Load then Store is not atomic, but products are never deleted, so an ID
// that exists at Load still exists at Store.
func (s *productStore) update(p product) bool {
	old, ok := s.get(p.ProductID)
	if !ok {
		return false
	}
	if p.Name == "" {
		p.Name = old.Name
	}
	s.products.Store(p.ProductID, p)
	return true
}

// search checks exactly maxChecked products (or all of them, if there are
// fewer) and returns up to maxResults whose name or category name contains q,
// ignoring case. Range visits products in no fixed order.
func (s *productStore) search(q string) []product {
	q = strings.ToLower(q)
	results := make([]product, 0, maxResults)
	checked := 0
	s.products.Range(func(_, v any) bool {
		p := v.(product)
		checked++
		if len(results) < maxResults &&
			(strings.Contains(strings.ToLower(p.Name), q) ||
				strings.Contains(strings.ToLower(categoryName(p.CategoryID)), q)) {
			results = append(results, p)
		}
		return checked < maxChecked // false stops Range
	})
	return results
}

// categoryName maps a category ID to its name. IDs outside the sample set
// (possible after a POST) have no name and match no query.
func categoryName(id int32) string {
	if id < 1 || int(id) > len(categories) {
		return ""
	}
	return categories[id-1]
}

// RED metrics (Rate, Errors, Duration), served at /metrics for Prometheus.
// route is the route template ("/products/:productId"), not the raw path,
// so 100,000 product IDs share one time series instead of creating 100,000.
var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Requests handled, by method, route, and status code.",
	}, []string{"method", "route", "status"})

	errorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_errors_total",
		Help: "Requests that returned a non-2xx status code.",
	}, []string{"method", "route", "status"})

	// Buckets from 50µs to about 1.6s: the handlers take microseconds, so the
	// default buckets (starting at 5ms) would put every request in the first one.
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time to handle a request, in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.00005, 2, 16),
	}, []string{"method", "route"})
)

// main registers the product routes and starts the server on port 8080.
// Every request runs recordMetrics, logRequest, and Recovery first, then simulateError, then its handler.
// /metrics and /health are registered before Use, so scrapes and ALB health
// checks are not counted in the RED metrics or logged.
func main() {
	router := gin.New() // empty router, no default middleware
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.Use(recordMetrics, logRequest, gin.CustomRecovery(recoverWithError)) // global middleware
	products := router.Group("/products", simulateError)                        // group middleware
	products.GET("/search", searchProducts)                                     // static path wins over /:productId
	products.GET("/:productId", getProduct)
	products.POST("/:productId/details", addProductDetails)
	router.Run(":8080")

}

// searchProducts responds with up to maxResults products whose name or
// category contains the q query parameter, e.g. GET /products/search?q=electro.
func searchProducts(c *gin.Context) {
	q := c.Query("q") // "" when q is missing
	if n := utf8.RuneCountInString(q); n < 1 || n > 100 {
		c.JSON(http.StatusBadRequest, errorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "Query parameter q must be 1 to 100 characters",
		})
		return
	}

	start := time.Now()
	results := store.search(q)
	c.JSON(http.StatusOK, searchResponse{
		Products:   results,
		TotalFound: len(results),
		SearchTime: time.Since(start).String(),
	})
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
// The caller picks the status for an invalid ID: GET returns 404 (api.yaml
// has no 400 for GET), POST returns 400.
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

// recordMetrics counts and times every request. It runs outside Recovery,
// so it still sees the final status when a panic is turned into a 500.
func recordMetrics(c *gin.Context) {
	start := time.Now()
	c.Next()

	route := routeLabel(c)
	code := c.Writer.Status()
	status := strconv.Itoa(code)

	requestsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
	if code < 200 || code >= 300 {
		errorsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
	}
	requestDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
}

// logger writes one JSON object per line to stdout.
var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// logRequest writes one JSON log line per request with the same RED data
// as the metrics, so it can be searched after the fact.
func logRequest(c *gin.Context) {
	start := time.Now()
	c.Next()

	logger.Info("request",
		"method", c.Request.Method,
		"endpoint", routeLabel(c),
		"path", c.Request.URL.Path,
		"status", c.Writer.Status(),
		"duration_ms", float64(time.Since(start).Microseconds())/1000,
	)
}

// routeLabel returns the matched route template, e.g. "/products/:productId".
func routeLabel(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return "unmatched" // no route matched
}

// recoverWithError turns a panic in any handler into a 500 in the Error schema.
func recoverWithError(c *gin.Context, err any) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{
		Error:   "INTERNAL_ERROR",
		Message: "Internal server error",
	})
}
