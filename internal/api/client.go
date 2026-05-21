package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/svg153/engram-monitor-tui/internal/model"
)

var broadTerms = []string{"the", "is", "to", "in", "a", "of", "and", "project", "agent"}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Health(ctx context.Context) (model.Health, error) {
	var health model.Health
	err := c.getJSON(ctx, "/health", nil, &health)
	return health, err
}

func (c *Client) Stats(ctx context.Context) (model.Stats, error) {
	var stats model.Stats
	err := c.getJSON(ctx, "/stats", nil, &stats)
	return stats, err
}

func (c *Client) Search(ctx context.Context, params model.SearchParams) ([]model.Observation, error) {
	query := url.Values{}
	query.Set("q", params.Q)
	if params.Type != "" {
		query.Set("type", params.Type)
	}
	if params.Project != "" {
		query.Set("project", params.Project)
	}
	if params.Scope != "" {
		query.Set("scope", params.Scope)
	}
	if params.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	var observations []model.Observation
	if err := c.getJSON(ctx, "/search", query, &observations); err != nil {
		return nil, err
	}
	return observations, nil
}

func (c *Client) AllObservations(ctx context.Context) ([]model.Observation, error) {
	type result struct {
		items []model.Observation
		err   error
	}
	ch := make(chan result, len(broadTerms))
	for _, term := range broadTerms {
		go func(query string) {
			items, err := c.Search(ctx, model.SearchParams{Q: query, Limit: 1000})
			ch <- result{items: items, err: err}
		}(term)
	}
	seen := make(map[int64]struct{})
	var all []model.Observation
	var errs []string
	for range broadTerms {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-ch:
			if res.err != nil {
				errs = append(errs, res.err.Error())
				continue
			}
			for _, item := range res.items {
				if _, ok := seen[item.ID]; ok {
					continue
				}
				seen[item.ID] = struct{}{}
				all = append(all, item)
			}
		}
	}
	if len(all) == 0 && len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return all, nil
}

func (c *Client) RecentSessions(ctx context.Context, limit int) ([]model.SessionSummary, error) {
	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", limit))
	var sessions []model.SessionSummary
	if err := c.getJSON(ctx, "/sessions/recent", query, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (c *Client) RecentPrompts(ctx context.Context, limit int) ([]model.Prompt, error) {
	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", limit))
	var prompts []model.Prompt
	if err := c.getJSON(ctx, "/prompts/recent", query, &prompts); err != nil {
		return nil, err
	}
	return prompts, nil
}

func (c *Client) Timeline(ctx context.Context, observationID int64, before, after int) (model.TimelineResult, error) {
	query := url.Values{}
	query.Set("observation_id", fmt.Sprintf("%d", observationID))
	query.Set("before", fmt.Sprintf("%d", before))
	query.Set("after", fmt.Sprintf("%d", after))
	var result model.TimelineResult
	err := c.getJSON(ctx, "/timeline", query, &result)
	return result, err
}

func (c *Client) UpdateObservation(ctx context.Context, id int64, payload model.ObservationUpdate) (model.Observation, error) {
	var observation model.Observation
	err := c.sendJSON(ctx, http.MethodPatch, fmt.Sprintf("/observations/%d", id), payload, &observation)
	return observation, err
}

func (c *Client) DeletePrompt(ctx context.Context, id int64) error {
	return c.sendJSON(ctx, http.MethodDelete, fmt.Sprintf("/prompts/%d", id), nil, nil)
}

func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.sendJSON(ctx, http.MethodDelete, "/sessions/"+url.PathEscape(id), nil, nil)
}

func (c *Client) Export(ctx context.Context, project string) ([]byte, error) {
	query := url.Values{}
	if strings.TrimSpace(project) != "" {
		query.Set("project", project)
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/export", query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, decodeHTTPError(resp)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) Import(ctx context.Context, data []byte) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/import", nil, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeHTTPError(resp)
	}
	return nil
}

func (c *Client) ImportFile(ctx context.Context, path string) (model.ExportData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.ExportData{}, err
	}
	var data model.ExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		return model.ExportData{}, err
	}
	if err := c.Import(ctx, raw); err != nil {
		return model.ExportData{}, err
	}
	return data, nil
}

func (c *Client) MergeProjects(ctx context.Context, from, to string) error {
	payload := map[string]string{"from": from, "to": to}
	return c.sendJSON(ctx, http.MethodPost, "/projects/migrate", payload, nil)
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, dst any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeHTTPError(resp)
	}
	if dst == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (c *Client) sendJSON(ctx context.Context, method, path string, payload any, dst any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := c.newRequest(ctx, method, path, nil, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeHTTPError(resp)
	}
	if dst == nil || resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func decodeHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error) != "" {
		return fmt.Errorf("%s", payload.Error)
	}
	if strings.TrimSpace(string(body)) != "" {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("http %d", resp.StatusCode)
}
