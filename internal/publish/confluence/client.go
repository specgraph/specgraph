// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PageInfo represents a Confluence page response.
type PageInfo struct {
	ID      string
	Title   string
	Version int
	WebURL  string
}

// CommentInfo represents a Confluence comment.
type CommentInfo struct {
	ID               string
	Body             string
	Author           string
	CreatedAt        string
	InlineProperties map[string]any // for inline comments: textSelection, etc.
	ParentID         string
}

// Client communicates with the Confluence REST API v2.
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient creates a Confluence API client.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s", cfg.CloudID)
	}
	return &Client{cfg: cfg, http: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.UserEmail, c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("confluence API %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// CreatePage creates a new Confluence page.
func (c *Client) CreatePage(ctx context.Context, title, parentID string, adfBody []byte) (*PageInfo, error) {
	payload := map[string]any{
		"spaceId":  c.cfg.SpaceKey,
		"title":    title,
		"parentId": parentID,
		"status":   "current",
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(adfBody),
		},
	}
	respBody, err := c.do(ctx, http.MethodPost, "/wiki/api/v2/pages", payload)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// UpdatePage updates an existing Confluence page.
func (c *Client) UpdatePage(ctx context.Context, pageID, title string, version int, adfBody []byte) (*PageInfo, error) {
	payload := map[string]any{
		"id":     pageID,
		"title":  title,
		"status": "current",
		"version": map[string]any{
			"number": version + 1,
		},
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(adfBody),
		},
	}
	respBody, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), payload)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// GetPage retrieves a Confluence page by ID.
func (c *Client) GetPage(ctx context.Context, pageID string) (*PageInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// DeletePage deletes a Confluence page.
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), nil)
	return err
}

// GetInlineComments retrieves inline comments for a page.
func (c *Client) GetInlineComments(ctx context.Context, pageID string) ([]CommentInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s/inline-comments", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parseCommentsResponse(respBody)
}

// GetFooterComments retrieves footer comments for a page.
func (c *Client) GetFooterComments(ctx context.Context, pageID string) ([]CommentInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s/footer-comments", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parseCommentsResponse(respBody)
}

func parsePageResponse(data []byte) (*PageInfo, error) {
	var resp struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse page response: %w", err)
	}
	return &PageInfo{
		ID:      resp.ID,
		Title:   resp.Title,
		Version: resp.Version.Number,
		WebURL:  resp.Links.WebUI,
	}, nil
}

func parseCommentsResponse(data []byte) ([]CommentInfo, error) {
	var resp struct {
		Results []struct {
			ID   string `json:"id"`
			Body struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
			Version struct {
				AuthorID string `json:"authorId"`
			} `json:"version"`
			Properties struct {
				InlineMarker map[string]any `json:"inline-marker"`
			} `json:"properties"`
			CreatedAt string `json:"createdAt"`
			ParentID  string `json:"parentId"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}
	comments := make([]CommentInfo, len(resp.Results))
	for i, r := range resp.Results {
		comments[i] = CommentInfo{
			ID:               r.ID,
			Body:             r.Body.Storage.Value,
			Author:           r.Version.AuthorID,
			CreatedAt:        r.CreatedAt,
			InlineProperties: r.Properties.InlineMarker,
			ParentID:         r.ParentID,
		}
	}
	return comments, nil
}
