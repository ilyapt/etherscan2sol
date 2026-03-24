package etherscan

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxResponseSize = 50 * 1024 * 1024 // 50 MB

// Client is an HTTP client for the Etherscan v2 API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Etherscan API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.etherscan.io/v2/api",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetSourceCode fetches the verified source code for a contract.
func (c *Client) GetSourceCode(chainID, address string) (*Result, error) {
	params := url.Values{}
	params.Set("chainid", chainID)
	params.Set("module", "contract")
	params.Set("action", "getsourcecode")
	params.Set("address", address)
	params.Set("apikey", c.apiKey)

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("etherscan request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("etherscan returned HTTP %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding etherscan response: %w", err)
	}

	if apiResp.Status != "1" {
		return nil, fmt.Errorf("etherscan API error: %s", apiResp.Message)
	}

	if len(apiResp.Result) == 0 {
		return nil, fmt.Errorf("etherscan returned no results")
	}

	return &apiResp.Result[0], nil
}
