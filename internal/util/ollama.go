package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

func CreateOllamaGenaration(prompt OllamaGenerateRequest) (OllamaGenerateResponse, error) {

	data, err :=json.Marshal(prompt)
	if err !=nil {
		return OllamaGenerateResponse{}, err
	}
	os.WriteFile("req.json", data, 0644)

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/api/generate",ConfigFile.OLLAMA_URL), bytes.NewBuffer(data))

	if err != nil {
		fmt.Println(err)
		return OllamaGenerateResponse{}, err
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return OllamaGenerateResponse{}, err
	}

	var bodyData string
	if resp != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		bodyData = buf.String()
	}
	if resp.StatusCode != 200 {
		fmt.Println(bodyData)
		return OllamaGenerateResponse{}, errors.New(bodyData)
	}
	
	var r OllamaGenerateResponse
	json.Unmarshal([]byte(bodyData), &r)
	return r, nil
}
