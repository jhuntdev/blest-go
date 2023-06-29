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

Install BLEST Go from Go Modules.

```bash
go get github.com/jhuntdev/blest-go
```

## Usage

Use the `CreateRequestHandler` function to create a request handler suitable for use in an existing Python application. Use the `CreateHttpServer` function to create a standalone HTTP server for your request handler.
<!-- Use the `createHttpClient` function to create a BLEST HTTP client. -->

### createRequestHandler

The following example uses Gin, but you can find examples with other frameworks [here](examples).

```go
package main

import (
  "blest"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	requestHandler := blest.createRequestHandler(router, nil)

  // Use the request handler
	response := requestHandler(requests, nil)
	result, ok1 := response[0].([][4]interface{})
	reqErr, ok2 := response[1].(map[string]interface{})
	if ok2 && reqErr != nil {
		log.Println(reqErr["message"])
		statusCode, ok3 := reqErr["code"].(int)
		if !ok3 {
			statusCode = 500
		}
		log.Println(statusCode)
		log.Println(reqErr["message"])
		return
	} else if !ok1 {
		log.Println("Request handler returned an improperly formatted response")
		return
	} else {
		json, err := json.Marshal(result)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			fmt.Println("Result:", string(json))
		}
	}

	server := CreateHTTPServer(handler, nil)

	log.Fatal(server.ListenAndServe())
}
```

### createHttpServer

```go
package main

import (
  "blest"
	"errors"
  "fmt"
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
	requestHandler := blest.createRequestHandler(router, nil)

  // Ceate the server
	server := blest.createHTTPServer(requestHandler, map[string]interface{}{
    "port": 8080
  })

  // Listen for requests
	log.Fatal(server.ListenAndServe())

}
```

<!-- ### createHttpClient

```go
package main

import (
  "blest"
	"encoding/json"
	"errors"
	"fmt"
	"log"
)

func main() {

# Create a client
request = create_http_client('http://localhost:8080')

async def main():
  # Send a request
  try:
    result = await request('greet', { 'name': 'Steve' }, ['greeting'])
    # Do something with the result
  except Exception as error:
    # Do something in case of error
``` -->

## License

This project is licensed under the [MIT License](LICENSE).