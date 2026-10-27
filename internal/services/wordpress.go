package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type WordPressService struct {
	baseURL    string
	authHeader string
	client     *http.Client
}

type WordPressRenderedField struct {
	Raw      string `json:"raw,omitempty"`
	Rendered string `json:"rendered,omitempty"`
}

func (w *WordPressRenderedField) UnmarshalJSON(data []byte) error {

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		w.Rendered = str
		return nil
	}

	var obj struct {
		Raw      string `json:"raw,omitempty"`
		Rendered string `json:"rendered,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		w.Raw = obj.Raw
		w.Rendered = obj.Rendered
		return nil
	}

	return fmt.Errorf("cannot unmarshal WordPressRenderedField from %s", string(data))
}

func (w WordPressRenderedField) MarshalJSON() ([]byte, error) {

	return json.Marshal(w.Rendered)
}

func (w WordPressRenderedField) String() string {
	return w.Rendered
}

type WordPressPost struct {
	ID            int                    `json:"id,omitempty"`
	Title         WordPressRenderedField `json:"title,omitempty"`
	Excerpt       WordPressRenderedField `json:"excerpt,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Type          string                 `json:"type,omitempty"`
	Author        int                    `json:"author,omitempty"`
	Categories    []int                  `json:"project_category,omitempty"`
	Tags          []int                  `json:"tags,omitempty"`
	FeaturedMedia int                    `json:"featured_media,omitempty"`
	Slug          string                 `json:"slug,omitempty"`
	Date          string                 `json:"date,omitempty"`
	Modified      string                 `json:"modified,omitempty"`
	Meta          map[string]interface{} `json:"meta,omitempty"`
}

type WordPressMedia struct {
	ID          int                    `json:"id,omitempty"`
	Title       WordPressRenderedField `json:"title,omitempty"`
	SourceURL   string                 `json:"source_url,omitempty"`
	URL         string                 `json:"url,omitempty"`
	AltText     string                 `json:"alt_text,omitempty"`
	Caption     WordPressRenderedField `json:"caption,omitempty"`
	Description WordPressRenderedField `json:"description,omitempty"`
	MediaType   string                 `json:"media_type,omitempty"`
	MimeType    string                 `json:"mime_type,omitempty"`
	Date        string                 `json:"date,omitempty"`
	Modified    string                 `json:"modified,omitempty"`
}

type WordPressCategory struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Count       int    `json:"count,omitempty"`
	Parent      int    `json:"parent"`
}

type WordPressTag struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Count       int    `json:"count,omitempty"`
}

type WordPressMenu struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

func NewWordPressService(config config.WordPressConfig) *WordPressService {

	// Prefer the application password; fall back to the plain password
	// (config.Load accepts either one).
	password := config.ApplicationPassword
	if password == "" {
		password = config.Password
	}
	authString := fmt.Sprintf("%s:%s", config.Username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	baseURL := strings.TrimRight(config.BaseURL, "/")
	authHeader := "Basic " + encodedAuth

	// Never log authHeader: it is the base64-encoded username:password.
	debug.Printf("WordPress Service Initialized - Base URL: %s, Username: %s", baseURL, config.Username)

	return &WordPressService{
		baseURL:    baseURL,
		authHeader: authHeader,
		client:     &http.Client{Timeout: httpTimeout},
	}
}

// httpTimeout bounds every WordPress request (including media uploads and
// image downloads) so a stalled connection cannot hang a run.
const httpTimeout = 120 * time.Second

// maxLoggedBodyBytes caps response bodies copied into logs and errors.
const maxLoggedBodyBytes = 2048

// truncateForLog shortens b to maxLoggedBodyBytes, noting how much was cut.
func truncateForLog(b []byte) string {
	if len(b) <= maxLoggedBodyBytes {
		return string(b)
	}
	return fmt.Sprintf("%s... (truncated, %d bytes total)", b[:maxLoggedBodyBytes], len(b))
}

// Download fetches url with the service's HTTP client (timeout applies).
// No Authorization header is sent: the URL may point at any host.
func (s *WordPressService) Download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return data, nil
}

// cleanCategories removes any 0 values from the Categories array
func cleanCategories(categories []int) []int {
	if categories == nil {
		return nil
	}

	var cleaned []int
	for _, cat := range categories {
		if cat != 0 {
			cleaned = append(cleaned, cat)
		}
	}
	return cleaned
}

func (s *WordPressService) CreatePost(ctx context.Context, post *WordPressPost) (*WordPressPost, error) {
	l := logger.Get()
	op := l.StartOperation("wordpress_create_post")
	op.WithContext("post_title", post.Title.String())
	op.WithContext("post_slug", post.Slug)
	op.WithContext("post_status", post.Status)

	// Clean the project_category array by removing any 0 values
	post.Categories = cleanCategories(post.Categories)
	if len(post.Categories) == 0 {
		post.Categories = nil // Set to nil if empty to use omitempty
	}

	jsonData, err := json.Marshal(post)
	if err != nil {
		op.Fail("Failed to marshal post", err)
		return nil, fmt.Errorf("failed to marshal post: %v", err)
	}

	resp, err := s.makeRequest(ctx, "POST", "/wp/v2/project", jsonData)
	if err != nil {
		op.Fail("WordPress API request failed", err)
		return nil, err
	}
	defer resp.Body.Close()

	var createdPost WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&createdPost); err != nil {
		op.Fail("Failed to decode response", err)
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	op.WithWordPress(createdPost.ID, 0, createdPost.Slug)
	op.WithContext("status_code", resp.StatusCode)
	op.Complete(fmt.Sprintf("Created WordPress post: %s (ID: %d)", createdPost.Title.String(), createdPost.ID))
	return &createdPost, nil
}

func (s *WordPressService) UpdatePost(ctx context.Context, postID int, post *WordPressPost) (*WordPressPost, error) {
	l := logger.Get()
	op := l.StartOperation("wordpress_update_post")
	op.WithWordPress(postID, 0, post.Slug)
	op.WithContext("post_title", post.Title.String())
	op.WithContext("post_status", post.Status)

	// Clean the project_category array by removing any 0 values
	post.Categories = cleanCategories(post.Categories)
	if len(post.Categories) == 0 {
		post.Categories = nil // Set to nil if empty to use omitempty
	}

	jsonData, err := json.Marshal(post)
	if err != nil {
		op.Fail("Failed to marshal post", err)
		return nil, fmt.Errorf("failed to marshal post: %v", err)
	}

	resp, err := s.makeRequest(ctx, "PUT", fmt.Sprintf("/wp/v2/project/%d", postID), jsonData)
	if err != nil {
		op.Fail("WordPress API request failed", err)
		return nil, err
	}
	defer resp.Body.Close()

	var updatedPost WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&updatedPost); err != nil {
		op.Fail("Failed to decode response", err)
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	op.WithWordPress(updatedPost.ID, 0, updatedPost.Slug)
	op.WithContext("status_code", resp.StatusCode)
	op.Complete(fmt.Sprintf("Updated WordPress post: %s (ID: %d)", updatedPost.Title.String(), updatedPost.ID))
	return &updatedPost, nil
}

func (s *WordPressService) UploadMediaFromFile(ctx context.Context, filePath, title, altText string) (*WordPressMedia, error) {
	// Log HTTP request with multipart payload info
	l := logger.Get()
	op := l.StartOperation("wordpress_upload_media_from_file")
	op.WithContext("http_method", "POST")
	op.WithContext("http_url", s.baseURL+"/wp-json/wp/v2/media")
	op.WithContext("http_endpoint", "/wp/v2/media")
	op.WithContext("http_request_payload_type", "multipart/form-data")
	op.WithContext("http_request_payload_file_path", filePath)
	op.WithContext("http_request_payload_title", title)
	op.WithContext("http_request_payload_alt_text", altText)

	file, err := os.Open(filePath)
	if err != nil {
		op.Fail("Failed to open file", err)
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		op.Fail("Failed to get file info", err)
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	op.WithContext("http_request_payload_file_name", fileInfo.Name())
	op.WithContext("http_request_payload_file_size", fileInfo.Size())

	body, contentType, err := buildMultipart(file, fileInfo.Name(), title, altText)
	if err != nil {
		op.Fail("Failed to build multipart body", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/wp-json/wp/v2/media", body)
	if err != nil {
		op.Fail("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", s.authHeader)

	resp, err := s.client.Do(req)
	if err != nil {
		op.WithContext("http_status_code", 0)
		op.Fail("HTTP request failed", err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	op.WithContext("http_status_code", resp.StatusCode)

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := truncateForLog(body)
		op.WithContext("http_response_body", bodyStr)
		op.Fail(fmt.Sprintf("Upload failed with status %d", resp.StatusCode), fmt.Errorf("%s", bodyStr))
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, bodyStr)
	}

	var media WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		op.Fail("Failed to decode response", err)
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	op.WithWordPress(0, media.ID, "")
	op.WithContext("http_response_media_id", media.ID)
	op.Complete(fmt.Sprintf("Uploaded media from file: %s (ID: %d)", media.Title.String(), media.ID))
	return &media, nil
}

// buildMultipart builds the POST /wp/v2/media form body: the file content
// under "file" plus optional "title" and "alt_text" fields. It returns the
// body and its Content-Type (with boundary).
func buildMultipart(file io.Reader, fileName, title, altText string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("failed to copy file data: %w", err)
	}
	if title != "" {
		if err := writer.WriteField("title", title); err != nil {
			return nil, "", fmt.Errorf("failed to write title field: %w", err)
		}
	}
	if altText != "" {
		if err := writer.WriteField("alt_text", altText); err != nil {
			return nil, "", fmt.Errorf("failed to write alt_text field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body: %w", err)
	}
	return &buf, writer.FormDataContentType(), nil
}

func (s *WordPressService) SearchCategories(ctx context.Context, year string) ([]*WordPressCategory, error) {

	query := url.Values{}
	query.Set("search", year)
	query.Set("per_page", "100")

	endpoint := "/wp/v2/project_category?" + query.Encode()

	resp, err := s.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var categories []*WordPressCategory
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return categories, nil
}

func (s *WordPressService) GetMedia(ctx context.Context, mediaID int) (*WordPressMedia, error) {
	resp, err := s.makeRequest(ctx, "GET", fmt.Sprintf("/wp/v2/media/%d", mediaID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var media WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &media, nil
}

// SearchMedia returns media items matching the search string (searches title and filename).
func (s *WordPressService) SearchMedia(ctx context.Context, search string) ([]*WordPressMedia, error) {
	if search == "" {
		return nil, nil
	}
	query := url.Values{}
	query.Set("search", search)
	query.Set("per_page", "20")
	endpoint := "/wp/v2/media?" + query.Encode()
	resp, err := s.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var items []*WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return items, nil
}

// TestConnection checks authentication against /wp/v2/users/me and returns
// the authenticated user's name (slug or display name).
func (s *WordPressService) TestConnection(ctx context.Context) (string, error) {
	resp, err := s.makeRequest(ctx, "GET", "/wp/v2/users/me", nil)
	if err != nil {
		return "", fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	var user struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("failed to decode user response: %w", err)
	}
	if user.Slug != "" {
		return user.Slug, nil
	}
	return user.Name, nil
}

func (s *WordPressService) GetNavMenus(ctx context.Context) ([]*WordPressMenu, error) {

	// As a last resort, try menu locations and synthesize names
	type menuLocation struct {
		Id          int    `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	endpoint := "/wp/v2/menus?per_page=20"

	resp, err := s.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return []*WordPressMenu{}, err
	}
	defer resp.Body.Close()

	// Read the response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return []*WordPressMenu{}, err
	}

	// Try to decode as array of menuLocation first
	var menuArray []menuLocation
	if err := json.Unmarshal(bodyBytes, &menuArray); err == nil {
		var result []*WordPressMenu
		for _, menu := range menuArray {
			result = append(result, &WordPressMenu{ID: menu.Id, Name: menu.Name, Slug: menu.Slug})
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// Try to decode as map of locations. Locations carry no menu term ID, so
	// ID is left 0 (unknown) rather than inventing one.
	var locations map[string]menuLocation
	if err := json.Unmarshal(bodyBytes, &locations); err == nil {
		var result []*WordPressMenu
		for slug, loc := range locations {
			result = append(result, &WordPressMenu{Name: loc.Name, Slug: slug})
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	return []*WordPressMenu{}, nil
}

func (s *WordPressService) makeRequest(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	url := s.baseURL + "/wp-json" + endpoint

	// Log HTTP request with payload
	l := logger.Get()
	op := l.StartOperation("wordpress_http_request")
	op.WithContext("http_method", method)
	op.WithContext("http_url", url)
	op.WithContext("http_endpoint", endpoint)

	if body != nil {
		// Payloads contain full Divi shortcode bodies; keep them out of the log
		// unless -debug is on.
		op.WithContext("http_request_payload_size", len(body))
		if debug.IsEnabled() {
			op.WithContext("http_request_payload", string(body))
		}
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		op.Fail("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", s.authHeader)

	debug.Printf("WordPress API Request - Method: %s, URL: %s", method, url)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		op.WithContext("http_status_code", 0)
		op.Fail("HTTP request failed", err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	// Log response status
	op.WithContext("http_status_code", resp.StatusCode)

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := truncateForLog(bodyBytes)
		op.WithContext("http_response_body", bodyStr)
		op.Fail(fmt.Sprintf("WordPress API Error - Status: %d", resp.StatusCode), fmt.Errorf("%s", bodyStr))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, bodyStr)
	}

	// Complete the operation successfully
	op.Complete(fmt.Sprintf("HTTP %s request to %s completed", method, endpoint))

	return resp, nil
}

func (s *WordPressService) GetCategoryIDsByNames(ctx context.Context, year string, categoryNames []string) ([]int, error) {
	l := logger.Get()
	op := l.StartOperation("wordpress_get_category_ids")
	op.WithContext("year", year)
	op.WithContext("category_names", categoryNames)

	var categoryIDs []int
	foundCount := 0
	notFoundCount := 0

	// One request per film instead of one per category name.
	categories, err := s.SearchCategories(ctx, year)
	if err != nil {
		op.Fail("Failed to search categories", err)
		return nil, fmt.Errorf("search categories for %q: %w", year, err)
	}

	for _, categoryName := range categoryNames {

		if strings.TrimSpace(categoryName) == "" {
			continue
		}

		foundCategory := matchCategory(categories, categoryName)
		if foundCategory != nil {
			categoryIDs = append(categoryIDs, foundCategory.ID)
			categoryIDs = append(categoryIDs, foundCategory.Parent)
			foundCount++
		} else {
			notFoundCount++
		}
	}

	op.WithContext("found_categories", foundCount)
	op.WithContext("not_found_categories", notFoundCount)
	op.WithContext("total_category_ids", len(categoryIDs))
	op.Complete(fmt.Sprintf("Found %d categories, %d not found", foundCount, notFoundCount))

	return categoryIDs, nil
}

// matchCategory returns the first category whose name contains categoryName,
// trying the dashed form first ("Competencia Nacional" -> "competencia-nacional")
// and then the plain form. Matching is case-insensitive. Returns nil when
// nothing matches.
func matchCategory(categories []*WordPressCategory, categoryName string) *WordPressCategory {
	name := strings.ToLower(strings.TrimSpace(categoryName))
	if name == "" {
		return nil
	}
	for _, candidate := range []string{strings.ReplaceAll(name, " ", "-"), name} {
		for _, category := range categories {
			if category != nil && strings.Contains(strings.ToLower(category.Name), candidate) {
				return category
			}
		}
	}
	return nil
}

func ParseCategoryString(categoryString string) []string {
	if categoryString == "" {
		return []string{}
	}

	parts := strings.Split(categoryString, ",")
	var categories []string

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			categories = append(categories, trimmed)
		}
	}

	return categories
}
