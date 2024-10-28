package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jhundev/blest-go"
)

// Create some middleware (optional)
func authMiddleware(body interface{}, context *map[string]interface{}) {
	headers, ok := context.(map[string]interface{})["headers"].(map[string]interface{})
	if !ok || headers["auth"] != "myToken" {
		return nil, errors.New("Unauthorized")
	}
	(*context)["user"] = map[string]interface{}{
		// user info for example
	}
}

// Create a route controller
func greetController(body interface{}, context *map[string]interface{}) (interface{}, error) {
	name, ok := body["name"].(string)
	if !ok {
		return nil, errors.New("name not found or has an invalid type")
	}
	greeting := fmt.Sprintf("Hi, %v!", name)
	return map[string]interface{}{
		"greeting": greeting,
	}, nil
}

func main() {
	// Set up a router
	router := map[string]interface{}{
		"greet": []func(interface{}, *map[string]interface{}) (interface{}, error){
			authMiddleware,
			greetController,
		},
	}

	// Create a request handler
	requestHandler := blest.CreateRequestHandler(router, nil)

	// Create a Gin POST requst handler
	handlePostRequest := func(c *gin.Context) {
		var req gin.Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid JSON payload",
			})
			return
		}

		// Use the request handler
		response := requestHandler(req.Data, nil)
		result, ok1 := response[0].([][4]interface{})
		reqErr, ok2 := response[1].(map[string]interface{})
		if ok2 && reqErr != nil {
			log.Println(reqErr["message"])
			statusCode, ok3 := reqErr["code"].(int)
			if !ok3 {
				statusCode = 500
			}
			c.String(statusCode, reqErr["message"])
			return
		} else if !ok1 {
			c.String(http.StatusInternalServerError, "Request handler returned an improperly formatted response")
			return
		} else {
			json, err := json.Marshal(result)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.JSON(http.StatusOK, string(json))
			}
		}
	}

	// Create a new Gin router
	app := gin.Default()

	// Enable CORS middleware
	app.Use(corsMiddleware())

	// Define the route handlers
	app.OPTIONS("/", handleOptionsRequest)
	app.POST("/", handlePostRequest)

	// Start the server
	app.Run(":8080")
}

// CORS middleware to enable CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		c.Next()
	}
}

// Handler for OPTIONS requests on "/"
func handleOptionsRequest(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}
