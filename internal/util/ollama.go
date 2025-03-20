package util

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)


func CreateOllamaGeneration(prompt OllamaGenerateRequest) (OllamaGenerateResponse, error) {

	data, err := json.Marshal(prompt)
	if err != nil {
		return OllamaGenerateResponse{}, err
	}
	// os.WriteFile("req.json", data, 0644)

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/api/generate", ConfigFile.OLLAMA_URL), bytes.NewBuffer(data))

	if err != nil {
		fmt.Println(err)
		return OllamaGenerateResponse{}, err
	}
	req.Header.Add("Content-Type", "application/json")

	switch ConfigFile.OLLAMA_AUTH_TYPE {
	case "basic":
		username, err := GetOllamaUsername()
		if err != nil {
			fmt.Println(err)
			return OllamaGenerateResponse{}, err
		}

		password, err := GetOllamaPassword()
		if err != nil {
			fmt.Println(err)
			return OllamaGenerateResponse{}, err
		}

		token := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		req.Header.Add("Authorization", fmt.Sprintf("Basic %s", token))
	}

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
