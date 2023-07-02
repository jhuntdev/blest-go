package blest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type BlestRequestHandler func(requests [][]interface{}, context map[string]interface{}) [2]interface{}

type BlestRequestSender func(route string, parameters interface{}, selector []interface{}) (interface{}, error)

type blestRequestObject struct {
	ID         string
	Route      string
	Parameters interface{}
	Selector   []interface{}
}

func CreateHTTPServer(requestHandler BlestRequestHandler, options interface{}) *http.Server {
	port := 8080

	if options != nil {
		portOption, err := options.(map[string]interface{})["port"].(int)
		if !err {
			port = portOption
		}
	}

	server := &http.Server{
		Addr: fmt.Sprintf("%s%d", ":", port),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var data [][]interface{}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		context := map[string]interface{}{
			"headers": r.Header,
		}

		response := requestHandler(data, context)
		result, ok1 := response[0].([][4]interface{})
		reqErr, ok2 := response[1].(map[string]interface{})
		if ok2 && reqErr != nil {
			log.Println(reqErr["message"])
			statusCode, ok3 := reqErr["code"].(int)
			if !ok3 {
				statusCode = 500
			}
			w.WriteHeader(statusCode)
			fmt.Fprint(w, reqErr["error"])
			return
		} else if !ok1 {
			log.Println(reqErr)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "Request handler returned an improperly formatted response")
			return
		} else if result != nil {
			responseJSON, err := json.Marshal(result)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, string(responseJSON))
			return
		} else {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	log.Println("Server listening on port 8080")

	return server
}

func httpPostRequest(url string, data interface{}) [][]interface{} {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Failed to marshal JSON data: %s", err)
		return nil
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to create request: %s", err)
		return nil
	}

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("POST request failed: %s", err)
		return nil
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Failed to read response body: %s", err)
		return nil
	}

	var result [][]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Printf("Failed to unmarshal JSON: %s", err)
		return nil
	}

	return result
}

type eventEmitter struct {
	events map[string][]chan interface{}
}

func (e *eventEmitter) on(event string, ch chan interface{}) {
	if e.events == nil {
		e.events = make(map[string][]chan interface{})
	}
	e.events[event] = append(e.events[event], ch)
}

func (e *eventEmitter) once(event string, ch chan interface{}) {
	chOnce := make(chan interface{}, 1)
	e.on(event, chOnce)
	go func() {
		defer close(chOnce)
		for val := range chOnce {
			ch <- val
			break
		}
		e.off(event, chOnce)
	}()
}

func (e *eventEmitter) emit(event string, args ...interface{}) {
	channels := e.events[event]
	if channels == nil {
		return
	}
	for _, ch := range channels {
		go func(ch chan interface{}) {
			ch <- args
		}(ch)
	}
}

func (e *eventEmitter) off(event string, ch chan interface{}) {
	if channels := e.events[event]; channels != nil {
		for i, c := range channels {
			if c == ch {
				e.events[event] = append(channels[:i], channels[i+1:]...)
				break
			}
		}
	}
}

func CreateHttpClient(url string, options map[string]interface{}) func(string, ...interface{}) (map[string]interface{}, error) {

	maxBatchSize := 100
	queue := [][]interface{}{}
	timeout := new(time.Timer)
	emitter := &eventEmitter{}

	timeout = nil

	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	process := func() {
		newQueue := queue[:min(len(queue), maxBatchSize)]
		queue = queue[len(newQueue):]
		timeout.Stop()
		if len(queue) == 0 {
			timeout = nil
		} else {
			timeout.Reset(1 * time.Millisecond)
		}

		data := httpPostRequest(url, newQueue)

		for _, r := range data {
			emitter.emit(r[0].(string), r[2], r[3])
		}
	}

	request := func(route string, args ...interface{}) (map[string]interface{}, error) {
		if route == "" {
			return nil, errors.New("Route is required")
		}

		var parameters map[string]interface{}
		if len(args) > 0 {
			p, ok := args[0].(map[string]interface{})
			if !ok && p != nil {
				return nil, errors.New("Parameters should be a map")
			}
			parameters = p
		}

		var selector []interface{}
		if len(args) > 1 {
			s, ok := args[1].([]interface{})
			if !ok && s != nil {
				return nil, errors.New("Selector should be a slice")
			}
			selector = s
		}

		id := uuid.New().String()
		ch := make(chan interface{}, 1)
		emitter.once(id, ch)
		queue = append(queue, []interface{}{id, route, parameters, selector})
		if timeout == nil {
			timeout = time.AfterFunc(1*time.Millisecond, process)
		}
		select {
		case val := <-ch:
			if err, ok := val.(error); ok {
				return nil, err
			}

			myVal, ok := val.([]interface{})
			if !ok || len(myVal) != 2 {
				return nil, errors.New("Invalid response format")
			}

			errVal, ok := myVal[1].(map[string]interface{})
			if !ok && errVal != nil {
				return nil, errors.New("Invalid error format")
			}
			if errVal != nil {
				errMsg, _ := errVal["message"].(string)
				return nil, errors.New(errMsg)
			}

			result, ok := myVal[0].(map[string]interface{})
			if !ok && result != nil {
				return nil, errors.New("Invalid result format")
			}

			return result, nil
		case <-time.After(5 * time.Second):
			return nil, errors.New("Request timed out")
		}
	}

	return request
}

func CreateRequestHandler(routes map[string]interface{}, options map[string]interface{}) BlestRequestHandler {
	if options != nil {
		fmt.Println("The \"options\" argument is not yet used, but may be used in the future")
	}

	routeRegex := regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_\\-/]*[a-zA-Z0-9_\\-]$")

	handler := func(requests [][]interface{}, context map[string]interface{}) [2]interface{} {
		if requests == nil || len(requests) == 0 {
			return handleError(400, "Request body should be a JSON array")
		}

		uniqueIds := make(map[string]bool)
		results := make([][4]interface{}, len(requests))

		for i, request := range requests {
			requestLen := len(request)

			if requestLen < 2 {
				return handleError(400, "Request item should be an array")
			}

			id, ok := request[0].(string)
			if !ok || id == "" {
				return handleError(400, "Request item should have an ID")
			}

			route, ok := request[1].(string)
			if !ok || route == "" {
				return handleError(400, "Request item should have a route")
			}

			if !routeRegex.MatchString(route) {
				routeLength := len(route)
				if routeLength < 2 {
					return handleError(400, "Request item route should be at least two characters long")
				} else if route[routeLength-1] == '/' {
					return handleError(400, "Request item route should not end in a forward slash")
				} else if !regexp.MustCompile("[a-zA-Z]").MatchString(route[:1]) {
					return handleError(400, "Request item route should start with a letter")
				} else {
					return handleError(400, "Request item route should contain only letters, numbers, dashes, underscores, and forward slashes")
				}
			}

			parameters := make(map[string]interface{})
			if requestLen > 2 {
				parameters, _ = request[2].(map[string]interface{})
			}

			var selector []interface{}
			if requestLen > 3 {
				selector, _ = request[3].([]interface{})
			}

			if _, exists := uniqueIds[id]; exists {
				return handleError(400, "Request items should have unique IDs")
			}

			uniqueIds[id] = true

			routeHandler, exists := routes[route]
			if !exists {
				routeHandler = routeNotFound
			}

			requestObject := blestRequestObject{
				ID:         id,
				Route:      route,
				Parameters: parameters,
				Selector:   selector,
			}

			result, err := routeReducer(routeHandler, requestObject, context)
			if err != nil {
				return handleError(500, err.Error())
			}

			results[i] = result
		}

		return handleResult(results)
	}

	return handler
}

func handleResult(result [][4]interface{}) [2]interface{} {
	return [2]interface{}{result, nil}
}

func handleError(code int, message string) [2]interface{} {
	return [2]interface{}{nil, map[string]interface{}{"code": code, "message": message}}
}

func routeNotFound() {
	panic(errors.New("Route not found"))
}

func routeReducer(handler interface{}, request blestRequestObject, context map[string]interface{}) ([4]interface{}, error) {
	var safeContext map[string]interface{}

	if context != nil {
		safeContext = deepCopyMap(context)
	} else {
		safeContext = make(map[string]interface{})
	}

	var result interface{}
	var err error

	switch h := handler.(type) {
	case func(interface{}, map[string]interface{}) (interface{}, error):
		result, err = h(request.Parameters, safeContext)
	case func(interface{}, *map[string]interface{}) (interface{}, error):
		result, err = h(request.Parameters, &safeContext)
	case []func(interface{}, map[string]interface{}) (interface{}, error):
		for i, f := range h {
			tempResult, tempErr := f(request.Parameters, safeContext)
			if i == len(h)-1 {
				result = tempResult
				err = tempErr
			} else {
				if tempResult != nil {
					err = errors.New("Middleware should not return anything but may mutate context")
					break
				}
			}
		}
	case []func(interface{}, *map[string]interface{}) (interface{}, error):
		for i, f := range h {
			tempResult, tempErr := f(request.Parameters, &safeContext)
			if i == len(h)-1 {
				result = tempResult
				err = tempErr
			} else {
				if tempResult != nil {
					err = errors.New("Middleware should not return anything but may mutate context")
					break
				}
			}
		}
	default:
		err = errors.New("Route not found")
	}

	if err != nil {
		return [4]interface{}{request.ID, request.Route, nil, map[string]interface{}{"message": err.Error()}}, nil
	}

	if result != nil {
		switch r := result.(type) {
		case map[string]interface{}:
			if request.Selector != nil {
				result = filterObject(r, request.Selector)
			}
		default:
			return [4]interface{}{request.ID, request.Route, nil, map[string]interface{}{"message": "The result, if any, should be a JSON object"}}, nil
		}
	}

	return [4]interface{}{request.ID, request.Route, result, nil}, nil
}

func filterObject(obj map[string]interface{}, arr []interface{}) map[string]interface{} {
	if arr == nil {
		return obj
	}

	filteredObj := make(map[string]interface{})

	for _, key := range arr {
		switch k := key.(type) {
		case string:
			if value, ok := obj[k]; ok {
				filteredObj[k] = value
			}
		case []interface{}:
			if nestedObj, ok := obj[k[0].(string)]; ok {
				switch nested := nestedObj.(type) {
				case []interface{}:
					filteredArr := make([]interface{}, 0)
					for _, nestedItem := range nested {
						filteredNestedObj := filterObject(nestedItem.(map[string]interface{}), k[1].([]interface{}))
						if len(filteredNestedObj) > 0 {
							filteredArr = append(filteredArr, filteredNestedObj)
						}
					}
					if len(filteredArr) > 0 {
						filteredObj[k[0].(string)] = filteredArr
					}
				case map[string]interface{}:
					filteredNestedObj := filterObject(nested, k[1].([]interface{}))
					if len(filteredNestedObj) > 0 {
						filteredObj[k[0].(string)] = filteredNestedObj
					}
				}
			}
		}
	}

	return filteredObj
}

func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{}, len(original))

	for key, value := range original {
		copy[key] = deepCopy(value)
	}

	return copy
}

func deepCopy(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return deepCopyMap(v)
	case []interface{}:
		return deepCopySlice(v)
	default:
		return value
	}
}

func deepCopySlice(slice []interface{}) []interface{} {
	copy := make([]interface{}, len(slice))
	for i, v := range slice {
		copy[i] = deepCopy(v)
	}
	return copy
}
