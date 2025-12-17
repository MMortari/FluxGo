package fluxgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
)

type ParsedResponse map[string]interface{}
type Headers map[string]string

func RunTestRequest(http *Http, method string, route string, reqBody interface{}, headers *Headers) (int, ParsedResponse) {
	status, body := RunTestRequestRaw(http, method, route, reqBody, headers)

	return status, ParseResponse(string(body))
}
func RunTestRequestRaw(http *Http, method string, route string, reqBody interface{}, headers *Headers) (int, []byte) {
	var requestBody io.Reader = nil

	if reqBody != nil {
		requestByte, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Println(err)
		}
		requestBody = bytes.NewReader(requestByte)
	}

	req := httptest.NewRequest(method, route, requestBody)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Api-Version", "1")
	if headers != nil {
		for key, value := range *headers {
			req.Header.Add(key, value)
		}
	}

	resp, _ := http.app.Test(req, 10000)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	if err := resp.Body.Close(); err != nil {
		fmt.Println(err)
	}

	return resp.StatusCode, responseBody
}

func ParseResponse(data string) ParsedResponse {
	var responseBody ParsedResponse

	if err := json.Unmarshal([]byte(data), &responseBody); err != nil {
		return nil
	}

	return responseBody
}

func ConvertToList(data interface{}) []interface{} {
	return data.([]interface{})
}
func ConvertToMap(data interface{}) map[string]interface{} {
	return data.(map[string]interface{})
}
