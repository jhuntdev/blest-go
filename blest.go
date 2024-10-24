package blest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RequestHandler func(requests [][]interface{}, context map[string]interface{}) ([][4]interface{}, map[string]interface{})

type requestObject struct {
	ID         string
	Route      string
	Body 	   interface{}
	Headers    interface{}
}

type eventEmitter struct {
	events map[string][]chan interface{}
}

type Router struct {
	Options       map[string]interface{}
	Introspection bool
	Middleware    []interface{}
	Afterware     []interface{}
	Timeout       int
	Routes        map[string]Route
}

type Route struct {
	Handler     []interface{}
	Description string
	Schema      interface{}
	Visible     bool
	Validate    bool
	Timeout     int
}

type HttpClient struct {
	Url          string
	Options      map[string]interface{}
	HttpHeaders  map[string]string
	MaxBatchSize int
	Queue        [][]interface{}
	Timeout      *time.Timer
	Emitter      *EventEmitter
}

type BlestError struct {
	Message    string
	StatusCode int
	Code       string
}

var routeRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_\-/]*[a-zA-Z0-9]$`)
var systemRouteRegex = regexp.MustCompile(`^_[a-zA-Z][a-zA-Z0-9_\-/]*[a-zA-Z0-9]$`)

func validateRoute(route string, system boolean) string {
	if route == "" {
		return "Route is required"
	} else if system && !systemRouteRegex.MatchString(route) {
		routeLength := len(route)
		if routeLength < 3 {
			return "System route should be at least three characters long"
		} else if route[0] != '_' {
			return "System route should start with an underscore"
		} else if !isLetterOrNumber(route[routeLength-1]) {
			return "System route should end with a letter or a number"
		} else {
			return "System route should contain only letters, numbers, dashes, underscores, and forward slashes"
		}
	} else if !system && !routeRegex.MatchString(route) {
		routeLength := len(route)
		if routeLength < 2 {
			return "Route should be at least two characters long"
		} else if !isLetter(route[0]) {
			return "Route should start with a letter"
		} else if !isLetterOrNumber(route[routeLength-1]) {
			return "Route should end with a letter or a number"
		} else {
			return "Route should contain only letters, numbers, dashes, underscores, and forward slashes"
		}
	} else if strings.Contains(route, "/") {
		subRoutes := strings.Split(route, "/")
		for _, subRoute := range subRoutes {
			if len(subRoute) < 2 {
				return "Sub-routes should be at least two characters long"
			} else if !isLetter(subRoute[0]) {
				return "Sub-routes should start with a letter"
			} else if !isLetterOrNumber(subRoute[len(subRoute)-1]) {
				return "Sub-routes should end with a letter or a number"
			}
		}
	}
	return ""
}

func isLetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func isLetterOrNumber(char byte) bool {
	return isLetter(char) || (char >= '0' && char <= '9')
}

func NewBlestError(message string, args ...interface{}) error {
	var status int
	var code string
	if len(args) > 1 {
		s, sOk := args[0].(int)
		if sOk && s > 0 {
			status = s
		} else {
			status = 500
		}
	}
	if len(args) > 2 {
		c, cOk := args[1].(string)
		if cOk && c != "" {
			code = c
		}
	}
	return &BlestError{
		Message:    message,
		StatusCode: status,
		Code:       code,
	}
}

func (be *BlestError) Error() string {
	return fmt.Sprint(be.Message)
}

func NewRouter(args ...interface{}) *Router {
	var options map[string]interface{}
	if len(args) > 0 {
		o, oOk := args[0].(map[string]interface{})
		if oOk {
			options = o
		}
	}
	var introspection bool
	if options["introspection"] != nil {
		i, ok := options["introspection"].(bool)
		if ok {
			introspection = i
		}
	}
	var timeout int
	if options["timeout"] != nil {
		t, ok := options["timeout"].(int)
		if !ok {
			panic("Timeout should be an integer")
		}
		if t < 0 {
			panic("Timeout should be a positive integer")
		}
		timeout = t
	}
	router := &Router{
		Options:       options,
		Introspection: introspection,
		Timeout:       timeout,
		Routes:        make(map[string]Route),
	}
	return router
}

func (r *Router) Use(handlers ...interface{}) {
	for _, handler := range handlers {
		if !isFunction(handler) {
			panic("All arguments should be functions")
		}
		argCount := reflect.ValueOf(handler).Type().NumIn()
		switch argCount {
		case 0, 1, 2:
			r.Middleware = append(r.Middleware, handler)
		case 3:
			r.Afterware = append(r.Afterware, handler)
		default:
			panic("Middleware should have at most three arguments")
		}
	}
}

func isFunction(fn interface{}) bool {
	fnType := reflect.TypeOf(fn)
	return fnType.Kind() == reflect.Func
}

func isSlice(v interface{}) bool {
	t := reflect.TypeOf(v)
	return t.Kind() == reflect.Slice
}

func (r *Router) Route(route string, args ...interface{}) {
	lastArg := args[len(args)-1]
	var options map[string]interface{}
	handlers := args

	if !isFunction(lastArg) {
		if opts, ok := lastArg.(map[string]interface{}); ok {
			options = opts
		} else {
			panic("Options should be a map")
		}
		handlers = args[:len(args)-1]
	}

	routeError := validateRoute(route)
	if routeError != "" {
		panic(routeError)
	} else if _, exists := r.Routes[route]; exists {
		panic("Route already exists")
	} else if len(handlers) == 0 {
		panic("At least one handler is required")
	} else if options != nil && reflect.TypeOf(options).Kind() != reflect.Map {
		panic("Last argument must be a configuration map or a handler function")
	} else {
		for i := 0; i < len(handlers); i++ {
			if !isFunction(handlers[i]) {
				panic(fmt.Sprintf("Handlers must be functions: %d", i))
			}
		}
	}

	r.Routes[route] = Route{
		Handler:     append(append([]interface{}{}, r.Middleware...), handlers...),
		Description: "",
		Schema:      nil,
		Visible:     r.Introspection,
		Validate:    false,
		Timeout:     r.Timeout,
	}

	if options != nil {
		r.Describe(route, options)
	}
}

func (r *Router) Describe(route string, config map[string]interface{}) error {
	routeInfo, exists := r.Routes[route]
	if !exists {
		return errors.New("Route does not exist")
	}

	if config == nil || reflect.TypeOf(config).Kind() != reflect.Map {
		return errors.New("Configuration should be a map")
	}

	if description, ok := config["description"].(string); ok {
		routeInfo.Description = description
	}

	if schema, ok := config["schema"].(map[string]interface{}); ok {
		routeInfo.Schema = schema
	}

	if visible, ok := config["visible"].(bool); ok {
		routeInfo.Visible = visible
	}

	if validate, ok := config["validate"].(bool); ok {
		routeInfo.Validate = validate
	}

	if timeout, ok := config["timeout"].(float64); ok {
		if timeout <= 0 || timeout != float64(int(timeout)) {
			return errors.New("Timeout should be a positive integer")
		}
		routeInfo.Timeout = int(timeout)
	}
	r.Routes[route] = routeInfo
	return nil
}

func (r *Router) Merge(router *Router) error {
	if router == nil {
		return errors.New("Router is required")
	}

	newRoutes := make([]string, 0, len(router.Routes))
	for route := range router.Routes {
		newRoutes = append(newRoutes, route)
	}

	existingRoutes := make([]string, 0, len(r.Routes))
	for route := range r.Routes {
		existingRoutes = append(existingRoutes, route)
	}

	if len(newRoutes) == 0 {
		return errors.New("No routes to merge")
	}

	for _, route := range newRoutes {
		if contains(existingRoutes, route) {
			return errors.New("Cannot merge duplicate routes: " + route)
		} else {
			var timeout int
			if router.Routes[route].Timeout > 0 {
				timeout = router.Routes[route].Timeout
			} else {
				timeout = r.Timeout
			}
			r.Routes[route] = Route{
				Handler:     append(append(append([]interface{}{}, r.Middleware...), router.Routes[route].Handler...), r.Afterware...),
				Description: router.Routes[route].Description,
				Schema:      router.Routes[route].Schema,
				Visible:     router.Routes[route].Visible,
				Validate:    router.Routes[route].Validate,
				Timeout:     timeout,
			}
		}
	}

	return nil
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func (r *Router) Namespace(prefix string, router *Router) error {
	if router == nil {
		return errors.New("Router is required")
	}

	prefixError := validateRoute(prefix)
	if prefixError != "" {
		return errors.New(prefixError)
	}

	newRoutes := make([]string, 0, len(router.Routes))
	for route := range router.Routes {
		newRoutes = append(newRoutes, route)
	}

	existingRoutes := make([]string, 0, len(r.Routes))
	for route := range r.Routes {
		existingRoutes = append(existingRoutes, route)
	}

	if len(newRoutes) == 0 {
		return errors.New("No routes to namespace")
	}

	for _, route := range newRoutes {
		nsRoute := prefix + "/" + route
		if contains(existingRoutes, route) {
			return errors.New("Cannot merge duplicate routes: " + nsRoute)
		} else {
			var timeout int
			if router.Routes[route].Timeout > 0 {
				timeout = router.Routes[route].Timeout
			} else {
				timeout = r.Timeout
			}
			r.Routes[nsRoute] = Route{
				Handler:     append(append(append([]interface{}{}, r.Middleware...), router.Routes[route].Handler...), r.Afterware...),
				Description: router.Routes[route].Description,
				Schema:      router.Routes[route].Schema,
				Visible:     router.Routes[route].Visible,
				Validate:    router.Routes[route].Validate,
				Timeout:     timeout,
			}
		}
	}

	return nil
}

func (r *Router) Handle(requests [][]interface{}, context map[string]interface{}) ([][4]interface{}, map[string]interface{}) {
	return handleRequest(r.Routes, requests, context)
}

func Default(options map[string]interface{}) *Router {
	return NewRouter(options)
}

func (r *Router) Run() {
	server := NewHttpServer(r.Handle, r.Options)
	log.Fatal(server.ListenAndServe())
}

func constructHttpHeaders(options map[string]interface{}) map[string]string {
	httpHeaders := map[string]string{
		"access-control-allow-origin":       "",
		"content-security-policy":           "default-src 'self';base-uri 'self';font-src 'self' https: data:;form-action 'self';frame-ancestors 'self';img-src 'self' data:;object-src 'none';script-src 'self';script-src-attr 'none';style-src 'self' https: 'unsafe-inline';upgrade-insecure-requests",
		"cross-origin-opener-policy":        "same-origin",
		"cross-origin-resource-policy":      "same-origin",
		"origin-agent-cluster":              "?1",
		"referrer-policy":                   "no-referrer",
		"strict-transport-security":         "max-age=15552000; includeSubDomains",
		"x-content-type-options":            "nosniff",
		"x-dns-prefetch-control":            "off",
		"x-download-options":                "noopen",
		"x-frame-options":                   "SAMEORIGIN",
		"x-permitted-cross-domain-policies": "none",
		"x-xss-protection":                  "0",
	}

	accessControlAllowOrigin, acaoOk := options["accessControlAllowOrigin"].(string)
	cors, corsOk := options["cors"].(bool)
	if acaoOk && accessControlAllowOrigin != "" {
		httpHeaders["access-control-allow-origin"] = accessControlAllowOrigin
	} else if corsOk && cors {
		httpHeaders["access-control-allow-origin"] = "*"
	}
	contentSecurityPolicy, cspOk := options["contentSecurityPolicy"].(string)
	if cspOk && contentSecurityPolicy != "" {
		httpHeaders["content-security-policy"] = contentSecurityPolicy
	}
	crossOriginOpenerPolicy, coopOk := options["crossOriginOpenerPolicy"].(string)
	if coopOk && crossOriginOpenerPolicy != "" {
		httpHeaders["cross-origin-opener-policy"] = crossOriginOpenerPolicy
	}
	crossOriginResourcePolicy, corpOk := options["crossOriginResourcePolicy"].(string)
	if corpOk && crossOriginResourcePolicy != "" {
		httpHeaders["cross-origin-resource-policy"] = crossOriginResourcePolicy
	}
	originAgentCluster, oacOk := options["originAgentCluster"].(string)
	if oacOk && originAgentCluster != "" {
		httpHeaders["origin-agent-cluster"] = originAgentCluster
	}
	referrerPolicy, rpOk := options["referrerPolicy"].(string)
	if rpOk && referrerPolicy != "" {
		httpHeaders["referrer-policy"] = referrerPolicy
	}
	strictTransportSecurity, stsOk := options["strictTransportSecurity"].(string)
	if stsOk && strictTransportSecurity != "" {
		httpHeaders["strict-transport-security"] = strictTransportSecurity
	}
	xContentTypeOptions, xctoOk := options["xContentTypeOptions"].(string)
	if xctoOk && xContentTypeOptions != "" {
		httpHeaders["x-content-type-options"] = xContentTypeOptions
	}
	xDnsPrefetchControl, xdpcOk := options["xDnsPrefetchControl"].(string)
	if xdpcOk && xDnsPrefetchControl != "" {
		httpHeaders["x-dns-prefetch-options"] = xDnsPrefetchControl
	}
	xDownloadOptions, xdoOk := options["xDownloadOptions"].(string)
	if xdoOk && xDownloadOptions != "" {
		httpHeaders["x-download-options"] = xDownloadOptions
	}
	xFrameOptions, xfoOk := options["xFrameOptions"].(string)
	if xfoOk && xFrameOptions != "" {
		httpHeaders["x-frame-options"] = xFrameOptions
	}
	xPermittedCrossDomainPolicies, xpcdpOk := options["xPermittedCrossDomainPolicies"].(string)
	if xpcdpOk && xPermittedCrossDomainPolicies != "" {
		httpHeaders["x-permitted-cross-domain-policies"] = xPermittedCrossDomainPolicies
	}
	xXssProtection, xxpOk := options["xXssProtection"].(string)
	if xxpOk && xXssProtection != "" {
		httpHeaders["x-xss-protection"] = xXssProtection
	}
	return httpHeaders
}

func NewHttpServer(requestHandler RequestHandler, args ...interface{}) *http.Server {

	var options map[string]interface{}

	opts, ok := args[0].(map[string]interface{})
	if ok {
		options = opts
	} else {
		fmt.Println("Value is not of type Person")
	}

	port, portOk := options["port"].(int)
	if !portOk || port == 0 {
		port = 8080
	}

	url, urlOk := options["url"].(string)
	if !urlOk || url == "" {
		url = "/"
	}

	httpHeaders := constructHttpHeaders(options)

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

		w.Header().Set("access-control-allow-origin", httpHeaders["access-control-allow-origin"])
		w.Header().Set("content-security-policy", httpHeaders["content-security-policy"])
		w.Header().Set("cross-origin-opener-policy", httpHeaders["cross-origin-opener-policy"])
		w.Header().Set("cross-origin-resource-policy", httpHeaders["cross-origin-resource-policy"])
		w.Header().Set("origin-agent-cluster", httpHeaders["origin-agent-cluster"])
		w.Header().Set("referrer-policy", httpHeaders["referrer-policy"])
		w.Header().Set("strict-transport-security", httpHeaders["strict-transport-security"])
		w.Header().Set("x-content-type-options", httpHeaders["x-content-type-options"])
		w.Header().Set("x-dns-prefetch-control", httpHeaders["x-dns-prefetch-control"])
		w.Header().Set("x-download-options", httpHeaders["x-download-options"])
		w.Header().Set("x-frame-options", httpHeaders["x-frame-options"])
		w.Header().Set("x-permitted-cross-domain-policies", httpHeaders["x-permitted-cross-domain-policies"])
		w.Header().Set("x-xss-protection", httpHeaders["x-xss-protection"])

		var data [][]interface{}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		context := map[string]interface{}{
			"headers": r.Header,
		}

		result, reqErr := requestHandler(data, context)
		// result, ok1 := response[0].([][4]interface{})
		// reqErr, ok2 := response[1].(map[string]interface{})
		if reqErr != nil {
			log.Println(reqErr["message"])
			statusCode, ok := reqErr["code"].(int)
			if !ok {
				statusCode = 500
			}
			w.WriteHeader(statusCode)
			fmt.Fprint(w, reqErr["error"])
			return
			// } else {
			// 	log.Println(reqErr)
			// 	w.WriteHeader(http.StatusInternalServerError)
			// 	fmt.Fprint(w, "Request handler returned an improperly formatted response")
			// 	return
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

	return server
}

func httpPostRequest(url string, data interface{}, headers map[string]string) [][]interface{} {
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

	if len(headers) > 0 {
		for key, value := range headers {
			request.Header.Set(key, value)
		}
	}

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

func NewHttpClient(url string, args ...interface{}) *HttpClient {
	var options map[string]interface{}
	var httpHeaders map[string]string
	if len(args) > 0 {
		o, oOk := args[0].(map[string]interface{})
		if oOk && o != nil {
			options = o
			h, hOk := o["httpHeaders"].(map[string]string)
			if hOk && h != nil {
				httpHeaders = h
			}
		}
	}
	maxBatchSize := 100
	queue := [][]interface{}{}
	timeout := new(time.Timer)
	emitter := &eventEmitter{}
	timeout = nil
	client := &HttpClient{
		Url:          url,
		Options:      options,
		HttpHeaders:  httpHeaders,
		MaxBatchSize: maxBatchSize,
		Queue:        queue,
		Timeout:      timeout,
		Emitter:      emitter,
	}
	return client
}

func (c *HttpClient) Process() {
	newQueue := c.Queue[:min(len(c.Queue), c.MaxBatchSize)]
	c.Queue = c.Queue[len(newQueue):]
	c.Timeout.Stop()
	if len(c.Queue) == 0 {
		c.Timeout = nil
	} else {
		c.Timeout.Reset(1 * time.Millisecond)
	}
	data := httpPostRequest(c.Url, newQueue, c.HttpHeaders)
	for _, r := range data {
		c.Emitter.emit(r[0].(string), r[2], r[3])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *HttpClient) Request(route string, args ...interface{}) (map[string]interface{}, error) {
	if route == "" {
		return nil, errors.New("Route is required")
	}

	var body map[string]interface{}
	if len(args) > 0 {
		b, ok := args[0].(map[string]interface{})
		if !ok && b != nil {
			return nil, errors.New("Body should be a map")
		}
		body = b
	}

	var headers map[string]interface{}
	if len(args) > 0 {
		h, ok := args[1].(map[string]interface{})
		if !ok && h != nil {
			return nil, errors.New("Headers should be a map")
		}
		headers = h
	}

	id := uuid.New().String()
	ch := make(chan interface{}, 1)
	c.Emitter.once(id, ch)
	c.Queue = append(c.Queue, []interface{}{id, route, body, headers})
	if c.Timeout == nil {
		c.Timeout = time.AfterFunc(1*time.Millisecond, c.Process)
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

func handleRequest(routes map[string]Route, requests [][]interface{}, context map[string]interface{}) ([][4]interface{}, map[string]interface{}) {
	if routes == nil {
		panic("Routes are required")
	} else if len(requests) == 0 {
		return handleError(400, "Request body should be a JSON array")
	}

	uniqueIds := make(map[string]bool)
	var results [][4]interface{}

	for _, request := range requests {
		if !isSlice(request) {
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

		var body map[string]interface{}
		if len(request) > 2 {
			b, ok := request[2].(map[string]interface{})
			if ok {
				body = b
			}
		}
		
		var headers map[string]interface{}
		if len(request) > 3 {
			h, ok := request[3].(map[string]interface{})
			if ok {
				headers = h
			}
		}

		if _, exists := uniqueIds[id]; exists {
			return handleError(400, "Request items should have unique IDs")
		}
		uniqueIds[id] = true

		var timeout int
		var routeHandler []interface{}
		thisRoute, exists := routes[route]
		if exists {
			routeHandler = thisRoute.Handler
			if thisRoute.Timeout > 0 {
				timeout = thisRoute.Timeout
			}
		} else {
			routeHandler = []interface{}{routeNotFound}
		}

		requestObject := requestObject{
			ID:         id,
			Route:      route,
			Body:		body,
			Headers: 	headers
		}

		requestContext := map[string]interface{}{}
		for key, value := range context {
			requestContext[key] = value
		}
		requestContext.id = id
		requestContext.route = route
		requestContext.headers = headers
		requestContext.time = time.Now().UnixNano() / int64(time.Millisecond)
		
		resultChan := routeReducer(routeHandler, requestObject, requestContext, timeout)
		// if err != nil {
		// 	return handleError(500, err.Error())
		// }

		for result := range resultChan {
			results = append(results, result)
		}
	}

	return handleResult(results)
}

func handleResult(result [][4]interface{}) ([][4]interface{}, map[string]interface{}) {
	return result, nil
}

func handleError(code int, message string) ([][4]interface{}, map[string]interface{}) {
	return nil, map[string]interface{}{"code": code, "message": message}
}

func routeNotFound() (map[string]interface{}, error) {
	return nil, NewBlestError("Not Found", 404, "NOT_FOUND")
}

func routeReducer(handler []interface{}, request requestObject, context map[string]interface{}, timeout int) <-chan [4]interface{} {
	resultChan := make(chan [4]interface{})

	go func() {
		defer close(resultChan)

		var timer *time.Timer
		var timedOut bool
		id, route, body := request.ID, request.Route, request.Body

		if timeout > 0 {
			timer = time.AfterFunc(time.Duration(timeout)*time.Millisecond, func() {
				timedOut = true
				fmt.Printf("The route \"%s\" timed out after %d milliseconds\n", route, timeout)
				resultChan <- [4]interface{}{id, route, nil, map[string]interface{}{"message": "Internal Server Error", "statusCode": 500}}
			})
		}

		safeContext := deepCopy(context).(map[string]interface{})
		var result interface{}
		var err error

		for _, f := range handler {
			argCount := reflect.ValueOf(f).Type().NumIn()
			if (timedOut || err != nil) && argCount <= 2 {
				continue
			}
			if err == nil && argCount > 2 {
				continue
			}
			var tempResult interface{}
			var tempErr error
			switch h := f.(type) {
			// Middleware
			case func():
				h()
			case func(interface{}):
				h(body)
			case func(interface{}, interface{}):
				h(body, safeContext)
			case func(map[string]interface{}):
				h(body.(map[string]interface{}))
			case func(map[string]interface{}, map[string]interface{}):
				h(body.(map[string]interface{}), safeContext)
			case func(map[string]interface{}, *map[string]interface{}):
				h(body.(map[string]interface{}), &safeContext)
			// Controllers
			case func() (interface{}, error):
				tempResult, tempErr = h()
			case func(interface{}) (interface{}, error):
				tempResult, tempErr = h(body)
			case func(interface{}, interface{}) (interface{}, error):
				tempResult, tempErr = h(body, safeContext)
			case func(map[string]interface{}) (interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}))
			case func(map[string]interface{}, map[string]interface{}) (interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}), safeContext)
			case func(map[string]interface{}, *map[string]interface{}) (interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}), &safeContext)
			case func() (map[string]interface{}, error):
				tempResult, tempErr = h()
			case func(map[string]interface{}) (map[string]interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}))
			case func(map[string]interface{}, map[string]interface{}) (map[string]interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}), safeContext)
			case func(map[string]interface{}, *map[string]interface{}) (map[string]interface{}, error):
				tempResult, tempErr = h(body.(map[string]interface{}), &safeContext)
			// Afterware
			case func(interface{}, interface{}, error):
				h(body, safeContext, err)
			case func(map[string]interface{}, map[string]interface{}, error):
				h(body.(map[string]interface{}), safeContext, err)
			case func(map[string]interface{}, *map[string]interface{}, error):
				h(body.(map[string]interface{}), &safeContext, err)
			default:
				err = errors.New("Unsupported route handler function definition")
			}
			if tempErr != nil {
				err = tempErr
			} else if tempResult != nil {
				if result == nil {
					result = tempResult
				} else {
					err = errors.New("Middleware should not return anything but may mutate context")
					break
				}
			}
		}

		if timedOut {
			return
		}

		if timer != nil {
			timer.Stop()
		}

		if err != nil {
			var statusCode int
			if blestErr, ok := err.(*BlestError); ok {
				statusCode = blestErr.StatusCode
			} else {
				statusCode = 500
			}
			resultChan <- [4]interface{}{id, route, nil, map[string]interface{}{"message": err.Error(), "statusCode": statusCode}}
		} else if result != nil {
			switch r := result.(type) {
			case map[string]interface{}:
				if selector != nil {
					result = filterObject(r, selector)
				}
			default:
				resultChan <- [4]interface{}{id, route, nil, map[string]interface{}{"message": "The result, if any, should be a JSON object", "statusCode": 500}}
			}
			resultChan <- [4]interface{}{id, route, result, nil}
		} else {
			resultChan <- [4]interface{}{id, route, nil, nil}
		}
	}()

	return resultChan
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
