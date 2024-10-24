# BLEST Go

The Go reference implementation of BLEST (Batch-able, Lightweight, Encrypted State Transfer), an improved communication protocol for web APIs which leverages JSON, supports request batching by default, and provides a modern alternative to REST. It includes an example for Gin.

To learn more about BLEST, please visit the website: https://blest.jhunt.dev

For a front-end implementation in React, please visit https://github.com/jhuntdev/blest-react

## Features

- Built on JSON - Reduce parsing time and overhead
- Request Batching - Save bandwidth and reduce load times
- Compact Payloads - Save even more bandwidth
- Single Endpoint - Reduce complexity and improve data privacy
- Fully Encrypted - Improve data privacy

## Installation

Install BLEST Go from Go Packages.

```bash
go get github.com/jhuntdev/blest-go
```

## Usage

The `Default` struct of this library has an interface similar to Gin. It also provides a `Router` struct with a `Handle` method for use in an existing NodeJS application and an `HttpClient` struct with a `Request` method for making BLEST HTTP requests.

```go
package main

import "github.com/jhuntdev/blest-go"

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
	r := blest.Default()
	r.Use(authMiddleware)
	r.Route("greet", greetController)
	r.Run() // listen and serve on 0.0.0.0:8080
}
```

### Router

```go
package main

import {
	"github.com/jhuntdev/blest-go"
	"github.com/gin-gonic/gin"
}

// Create some middleware (optional)
func authMiddleware(body interface{}, context *map[string]interface{}) {
	headers, ok := context.(map[string]interface{})["headers"].(map[string]interface{})
	if !ok || headers["auth"] !== "myToken" {
		return nil, errors.New("Unauthorized")
	}
	(*context)["user"] = map[string]interface{}{
		"name": name,
	}
}

// Create a route controller
func greetController(body interface{}, context *map[string]interface{}) (interface{}, error) {
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

// Create your router
router := blest.Router()
router.Use(authMiddleware)
router.Route("greet", greetController)

func main() {
	r := gin.Default()
	r.POST("/", func(c *gin.Context) {
		var req gin.Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid JSON payload",
			})
			return
		}
		// Use the router
		response, e := router.Handle(req.Data, nil)
		if e !== nil {
			c.JSON(500, e)
		} else {
			c.JSON(200, response)
		}
	})
	r.Run() // listen and serve on 0.0.0.0:8080
}
```

### HttpClient

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
	// Set headers (optional)
	httpHeaders := map[string]string{
		"Authorization": "Bearer token",
	}

	// Create a client
	client := blest.NewHttpClient("http://localhost:8080", map[string]interface{}{"httpHeaders": httpHeaders})
	
	// Send a request
	result, err := client.Request("greet", map[string]interface{}{ "name": "Steve" }, map[string]interface{}{"auth": "myToken"})
	if err != nil {
		// Do something in case of error
	} else {
		// Do something with the result
	}
}
```

## License

This project is licensed under the [MIT License](LICENSE).