package blest

import (
	"errors"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func dummyController(parameters map[string]interface{}, context *map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"hello": "world"}, nil
}

func TestRouter(t *testing.T) {
	t.Parallel()

	router := NewRouter(map[string]interface{}{"timeout": 1000})
	benchmarks := make([]int64, 0)

	var (
		testID1, testID2, testID3, testID4, testID5                string
		testValue1, testValue2, testValue3, testValue4, testValue5 float64
		result1, result2, result3, result4, result5                [][4]interface{}
		error1, error2, error3, error4, error5                     map[string]interface{}
	)

	router.Route("basicRoute", func(parameters map[string]interface{}, context *map[string]interface{}) (interface{}, error) {
		myContext := make(map[string]interface{})
		for key, value := range *context {
			myContext[key] = value
		}
		return map[string]interface{}{
			"route":      "basicRoute",
			"parameters": parameters,
			"context":    myContext,
		}, nil
	})

	router.Use(func(parameters map[string]interface{}, context *map[string]interface{}) {
		(*context)["test"] = map[string]interface{}{
			"value": parameters["testValue"],
		}
	})

	router.Use(func(parameters map[string]interface{}, context *map[string]interface{}, err error) {
		completeTime := time.Now().UnixNano()
		requestTime := (*context)["requestTime"].(int64)
		difference := completeTime - requestTime
		println((*context)["routeName"].(string))
		benchmarks = append(benchmarks, difference)
	})

	router2 := NewRouter(map[string]interface{}{"timeout": 10})
	router2.Route("mergedRoute", func(parameters map[string]interface{}, context *map[string]interface{}) (interface{}, error) {
		myContext := make(map[string]interface{})
		for key, value := range *context {
			myContext[key] = value
		}
		return map[string]interface{}{
			"route":      "mergedRoute",
			"parameters": parameters,
			"context":    myContext,
		}, nil
	})

	router2.Route("timeoutRoute", func(parameters map[string]interface{}) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		result := make(map[string]interface{})
		result["testValue"] = parameters["testValue"]
		return result, nil
	})

	router.Merge(router2)

	router3 := NewRouter()
	router3.Route("errorRoute", func(parameters map[string]interface{}) (interface{}, error) {
		errorMessage := parameters["testValue"].(float64)
		errorCode := "ERROR_" + strconv.Itoa(int(math.Round(errorMessage*10)))
		return nil, errors.New(errorCode)
	})

	router.Namespace("subRoutes", router3)

	// Basic route
	testID1 = uuid.New().String()
	testValue1 = rand.Float64()
	result1, error1 = router.Handle([][]interface{}{{testID1, "basicRoute", map[string]interface{}{"testValue": testValue1}}}, map[string]interface{}{"testValue": testValue1})

	// Merged route
	testID2 = uuid.New().String()
	testValue2 = rand.Float64()
	result2, error2 = router.Handle([][]interface{}{{testID2, "mergedRoute", map[string]interface{}{"testValue": testValue2}}}, map[string]interface{}{"testValue": testValue2})

	// Error route
	testID3 = uuid.New().String()
	testValue3 = rand.Float64()
	result3, error3 = router.Handle([][]interface{}{{testID3, "subRoutes/errorRoute", map[string]interface{}{"testValue": testValue3}}}, map[string]interface{}{"testValue": testValue3})

	// Missing route
	testID4 = uuid.New().String()
	testValue4 = rand.Float64()
	result4, error4 = router.Handle([][]interface{}{{testID4, "missingRoute", map[string]interface{}{"testValue": testValue4}}}, map[string]interface{}{"testValue": testValue4})

	// Timeout route
	testID5 = uuid.New().String()
	testValue5 = rand.Float64()
	result5, error5 = router.Handle([][]interface{}{{testID5, "timeoutRoute", map[string]interface{}{"testValue": testValue5}}}, map[string]interface{}{"testValue": testValue5})

	// Malformed request
	// result6, error6 = router.Handle([][]interface{}{{testID4}, map[string]interface{}{}, []interface{}{true, 1.25}})

	// result, ok := result5[0][3].(map[string]interface{})
	// if ok {
	// 	for key, value := range result {
	// 		println(key, value)
	// 	}
	// }

	// Assertions
	assert.IsType(t, &Router{}, router)
	assert.Equal(t, 4, len(router.Routes))
	assert.NotNil(t, router.Handle)

	assert.Nil(t, error1)
	assert.Nil(t, error2)
	assert.Nil(t, error3)
	assert.Nil(t, error4)
	assert.Nil(t, error5)

	assert.Equal(t, testID1, result1[0][0].(string))
	assert.Equal(t, "basicRoute", result1[0][1].(string))
	assert.InDelta(t, testValue1, result1[0][2].(map[string]interface{})["parameters"].(map[string]interface{})["testValue"].(float64), 1e-6)

	assert.Equal(t, testID2, result2[0][0].(string))
	assert.Equal(t, "mergedRoute", result2[0][1].(string))
	assert.InDelta(t, testValue2, result2[0][2].(map[string]interface{})["parameters"].(map[string]interface{})["testValue"].(float64), 1e-6)
	assert.InDelta(t, testValue2, result2[0][2].(map[string]interface{})["context"].(map[string]interface{})["test"].(map[string]interface{})["value"].(float64), 1e-6)

	assert.Equal(t, testID3, result3[0][0].(string))
	assert.Equal(t, "subRoutes/errorRoute", result3[0][1].(string))
	assert.Equal(t, 500, result3[0][3].(map[string]interface{})["statusCode"])
	assert.Equal(t, "ERROR_"+strconv.Itoa(int(math.Round(testValue3*10))), result3[0][3].(map[string]interface{})["message"])

	assert.Equal(t, testID4, result4[0][0].(string))
	assert.Equal(t, "missingRoute", result4[0][1].(string))
	assert.Equal(t, 404, result4[0][3].(map[string]interface{})["statusCode"])
	assert.Equal(t, "Not Found", result4[0][3].(map[string]interface{})["message"])

	assert.Equal(t, testID5, result5[0][0].(string))
	assert.Equal(t, "timeoutRoute", result5[0][1].(string))
	assert.Equal(t, 500, result5[0][3].(map[string]interface{})["statusCode"])
	assert.Equal(t, "Internal Server Error", result5[0][3].(map[string]interface{})["message"])

	// assert.NotNil(t, error6)

	assert.Equal(t, 1, len(benchmarks))

	// Additional test for invalid routes
	assert.Panics(t, func() {
		router.Route("a", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("0abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("_abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("-abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc_", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc-", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/0abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/_abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/-abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc//abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/a/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/0abc/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/_abc/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/-abc/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/abc_/abc", dummyController)
	})
	assert.Panics(t, func() {
		router.Route("abc/abc-/abc", dummyController)
	})
}
