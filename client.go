package netnod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://primarydnsapi.netnod.se"

type apiRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

type apiRRset struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	TTL        *int        `json:"ttl"`
	Records    []apiRecord `json:"records,omitempty"`
	ChangeType string      `json:"changetype,omitempty"`
}

type apiZone struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	NotifiedSerial int        `json:"notified_serial"`
	RRsets         []apiRRset `json:"rrsets,omitempty"`
}

type apiZoneList struct {
	Data   []apiZone `json:"data"`
	Offset int       `json:"offset"`
	Limit  int       `json:"limit"`
	Total  int       `json:"total"`
}

type apiError struct {
	Error string `json:"error"`
}

func (p *Provider) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	baseURL := defaultBaseURL
	if p.BaseURL != "" {
		baseURL = p.BaseURL
	}

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	client := http.DefaultClient
	if p.HTTPClient != nil {
		client = p.HTTPClient
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Token "+p.APIToken)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("performing request: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()

			wait := backoff
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}

			slog.Info("rate limited, retrying", "method", method, "path", path, "wait", wait)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("rate limited, context cancelled while waiting to retry: %w", ctx.Err())
			case <-time.After(wait):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		if resp.StatusCode >= 400 {
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			var apiErr apiError
			if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
				return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Error)
			}
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}

		return resp, nil
	}
}

func (p *Provider) getZone(ctx context.Context, zoneID string) (apiZone, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, "/api/v1/zones/"+url.PathEscape(zoneID), nil)
	if err != nil {
		return apiZone{}, err
	}
	defer resp.Body.Close()

	var zone apiZone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		return apiZone{}, fmt.Errorf("decoding zone response: %w", err)
	}
	return zone, nil
}

func (p *Provider) patchZone(ctx context.Context, zoneID string, rrsets []apiRRset) error {
	body := struct {
		RRsets []apiRRset `json:"rrsets"`
	}{RRsets: rrsets}

	resp, err := p.doRequest(ctx, http.MethodPatch, "/api/v1/zones/"+url.PathEscape(zoneID), body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (p *Provider) listAllZones(ctx context.Context) ([]apiZone, error) {
	var all []apiZone
	limit := 100
	offset := 0

	for {
		path := "/api/v1/zones?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
		resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var list apiZoneList
		err = json.NewDecoder(resp.Body).Decode(&list)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding zone list response: %w", err)
		}

		all = append(all, list.Data...)

		if offset+limit >= list.Total {
			break
		}
		offset += limit
	}

	return all, nil
}
