# BLEST Go

The Go reference implementation of BLEST (Batch-able, Lightweight, Encrypted State Transfer), an improved communication protocol for web APIs which leverages JSON, supports request batching and selective returns, and provides a modern alternative to REST. It includes an example for Gin.

To learn more about BLEST, please refer to the white paper: https://jhunt.dev/BLEST%20White%20Paper.pdf

For a front-end implementation in React, please visit https://github.com/jhunt/blest-react

## Features

- Built on JSON - Reduce parsing time and overhead
- Request Batching - Save bandwidth and reduce load times
- Compact Payloads - Save more bandwidth
- Selective Returns - Save even more bandwidth
- Single Endpoint - Reduce complexity and improve data privacy
- Fully Encrypted - Improve data privacy

## Installation

Install BLEST Go from Go Packages.

```bash
go get github.com/jhuntdev/blest-go
```

## Usage

Use the `CreateRequestHandler` function to create a request handler suitable for use in an existing Python application. Use the `CreateHttpServer` function to create a standalone HTTP server for your request handler. Use the `CreateHttpClient` function to create a BLEST HTTP client.

### CreateRequestHandler

The following example uses Gin.
<!-- , but you can find examples with other frameworks [here](examples). -->

```go
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
func authMiddleware(params interface{}, context *map[string]interface{}) (interface{}, error) {
	name, ok := params.(map[string]interface{})["name"].(string)
	if !ok {
		name = "Tarzan"
	}
	(*context)["user"] = map[string]interface{}{
		"name": name,
	}
	return nil, nil
}

// Create a route controller
func greetController(params interface{}, context *map[string]interface{}) (interface{}, error) {
	user, ok := (*context)["user"].(map[string]interface{})
	if !ok {
		return nil, errors.New("user not found or has an invalid type")
	}
	name, ok := user["name"].(string)
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
```

### CreateHttpServer

```go
package main

import (
	"errors"
	"fmt"

	"github.com/jhuntdev/blest-go"
)

func main() {

  // Create some middleware (optional)
  authMiddleware := func(params interface{}, context *map[string]interface{}) (interface{}, error) {
    name, ok := params.(map[string]interface{})["name"].(string)
    if !ok {
      name = "Tarzan"
    }
    (*context)["user"] = map[string]interface{}{
      "name": name,
    }
    return nil, nil
  }

  // Create a route controller
  greetController := func(params interface{}, context *map[string]interface{}) (interface{}, error) {
    user, ok := (*context)["user"].(map[string]interface{})
    if !ok {
      return nil, errors.New("user not found or has an invalid type")
    }
    name, ok := user["name"].(string)
    if !ok {
      return nil, errors.New("name not found or has an invalid type")
    }
    greeting := fmt.Sprintf("Hi, %v!", name)
    return map[string]interface{}{
      "greeting": greeting,
    }, nil
  }

  // Set up a router
  router := map[string]interface{}{
		"greet": []func(interface{}, *map[string]interface{}) (interface{}, error){
			authMiddleware,
      greetController
    }
	}

  // Create a request handler
	requestHandler := blest.CreateRequestHandler(router, nil)

  // Ceate the server
	server := blest.CreateHTTPServer(requestHandler, map[string]interface{}{
    "port": 8080
  })

  // Listen for requests
	log.Fatal(server.ListenAndServe())

}
```

### createHttpClient

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/jhuntdev/blest-go"
)

func main() {

  // Create a client
  request := blest.CreateHttpClient("http://localhost:8080")
  
  // Send a request
  result, err := request("greet", map[string]interface{}{ "name": "Steve" }, []interface{}{ "greeting" })
  if err != nil {
    // Do something in case of error
  }
  // Do something with the result

}
```

## License

This project is licensed under the [MIT License](LICENSE).