package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
	"excentrico-tools-go/internal/logger"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	Content       WordPressRenderedField `json:"content,omitempty"`
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
	Parent 			int    `json:"parent"`
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

	authString := fmt.Sprintf("%s:%s", config.Username, config.ApplicationPassword)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	baseURL := strings.TrimRight(config.BaseURL, "/")
	authHeader := "Basic " + encodedAuth

	debug.Printf("WordPress Service Initialized - Base URL: %s, Username: %s, Auth Header: %s", baseURL, config.Username, authHeader)

	return &WordPressService{
		baseURL:    baseURL,
		authHeader: authHeader,
		client:     &http.Client{},
	}
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

func (s *WordPressService) CreatePost(post *WordPressPost) (*WordPressPost, error) {
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

	resp, err := s.makeRequest("POST", "/wp/v2/project", jsonData)
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

func (s *WordPressService) UpdatePost(postID int, post *WordPressPost) (*WordPressPost, error) {
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

	resp, err := s.makeRequest("PUT", fmt.Sprintf("/wp/v2/project/%d", postID), jsonData)
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

func (s *WordPressService) GetPost(postID int) (*WordPressPost, error) {
	resp, err := s.makeRequest("GET", fmt.Sprintf("/wp/v2/project/%d", postID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var post WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &post, nil
}

func (s *WordPressService) GetPosts(params map[string]string) ([]*WordPressPost, error) {
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}

	endpoint := "/wp/v2/project"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	resp, err := s.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posts []*WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return posts, nil
}

func (s *WordPressService) DeletePost(postID int) error {
	resp, err := s.makeRequest("DELETE", fmt.Sprintf("/wp/v2/project/%d", postID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("Deleted WordPress post ID: %d", postID)
	return nil
}

func (s *WordPressService) UploadMedia(file *multipart.FileHeader, title, altText string) (*WordPressMedia, error) {
	// Log HTTP request with multipart payload info
	l := logger.Get()
	op := l.StartOperation("wordpress_upload_media")
	op.WithContext("http_method", "POST")
	op.WithContext("http_url", s.baseURL+"/wp-json/wp/v2/media")
	op.WithContext("http_endpoint", "/wp/v2/media")
	op.WithContext("http_request_payload_type", "multipart/form-data")
	op.WithContext("http_request_payload_file_name", file.Filename)
	op.WithContext("http_request_payload_file_size", file.Size)
	op.WithContext("http_request_payload_title", title)
	op.WithContext("http_request_payload_alt_text", altText)

	fileReader, err := file.Open()
	if err != nil {
		op.Fail("Failed to open file", err)
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer fileReader.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", file.Filename)
	if err != nil {
		op.Fail("Failed to create form file", err)
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = io.Copy(part, fileReader)
	if err != nil {
		op.Fail("Failed to copy file data", err)
		return nil, fmt.Errorf("failed to copy file data: %v", err)
	}

	if title != "" {
		writer.WriteField("title", title)
	}

	if altText != "" {
		writer.WriteField("alt_text", altText)
	}

	writer.Close()

	req, err := http.NewRequest("POST", s.baseURL+"/wp-json/wp/v2/media", &buf)
	if err != nil {
		op.Fail("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
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
		op.WithContext("http_response_body", string(body))
		op.Fail(fmt.Sprintf("Upload failed with status %d", resp.StatusCode), fmt.Errorf("%s", string(body)))
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var media WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		op.Fail("Failed to decode response", err)
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	op.WithWordPress(0, media.ID, "")
	op.WithContext("http_response_media_id", media.ID)
	op.Complete(fmt.Sprintf("Uploaded media: %s (ID: %d)", media.Title.String(), media.ID))
	return &media, nil
}

func (s *WordPressService) UploadMediaFromFile(filePath, title, altText string) (*WordPressMedia, error) {
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

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", fileInfo.Name())
	if err != nil {
		op.Fail("Failed to create form file", err)
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		op.Fail("Failed to copy file data", err)
		return nil, fmt.Errorf("failed to copy file data: %v", err)
	}

	if title != "" {
		writer.WriteField("title", title)
	}

	if altText != "" {
		writer.WriteField("alt_text", altText)
	}

	writer.Close()

	req, err := http.NewRequest("POST", s.baseURL+"/wp-json/wp/v2/media", &buf)
	if err != nil {
		op.Fail("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
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
		op.WithContext("http_response_body", string(body))
		op.Fail(fmt.Sprintf("Upload failed with status %d", resp.StatusCode), fmt.Errorf("%s", string(body)))
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
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

func (s *WordPressService) GetCategories() ([]*WordPressCategory, error) {
	resp, err := s.makeRequest("GET", "/wp/v2/project-category", nil)
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

func (s *WordPressService) SearchCategories(year string) ([]*WordPressCategory, error) {

	query := url.Values{}
	query.Set("search", year)
	query.Set("per_page", "100")

	endpoint := "/wp/v2/project_category?" + query.Encode()

	resp, err := s.makeRequest("GET", endpoint, nil)
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

func (s *WordPressService) SearchCategoriesWithParams(params map[string]string) ([]*WordPressCategory, error) {
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}

	endpoint := "/wp/v2/categories"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	resp, err := s.makeRequest("GET", endpoint, nil)
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

func (s *WordPressService) GetTags() ([]*WordPressTag, error) {
	resp, err := s.makeRequest("GET", "/wp/v2/tags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tags []*WordPressTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return tags, nil
}

func (s *WordPressService) SearchTags(searchText string) ([]*WordPressTag, error) {

	query := url.Values{}
	query.Set("search", searchText)

	endpoint := "/wp/v2/tags?" + query.Encode()

	resp, err := s.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tags []*WordPressTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return tags, nil
}

func (s *WordPressService) GetMedia(mediaID int) (*WordPressMedia, error) {
	resp, err := s.makeRequest("GET", fmt.Sprintf("/wp/v2/media/%d", mediaID), nil)
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

func (s *WordPressService) TestConnection() error {
	log.Printf("Testing WordPress API connection to: %s", s.baseURL)

	resp, err := s.makeRequest("GET", "/wp/v2/users/me", nil)
	if err != nil {
		return fmt.Errorf("connection test failed: %v", err)
	}
	defer resp.Body.Close()

	var user map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("failed to decode user response: %v", err)
	}

	if username, ok := user["username"].(string); ok {
		log.Printf("WordPress API connection successful - authenticated as: %s", username)
	} else {
		log.Printf("WordPress API connection successful - user info: %v", user)
	}

	return nil
}

func (s *WordPressService) GetNavMenus() ([]*WordPressMenu, error) {

	// As a last resort, try menu locations and synthesize names
	type menuLocation struct {
		Id          int    `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	endpoint := "/wp/v2/menus?per_page=20"

	resp, err := s.makeRequest("GET", endpoint, nil)
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

	// Try to decode as map of locations
	var locations map[string]menuLocation
	if err := json.Unmarshal(bodyBytes, &locations); err == nil {
		var result []*WordPressMenu
		i := 1
		for slug, loc := range locations {
			result = append(result, &WordPressMenu{ID: i, Name: loc.Name, Slug: slug})
			i++
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	return []*WordPressMenu{}, nil
}

func (s *WordPressService) makeRequest(method, endpoint string, body []byte) (*http.Response, error) {
	url := s.baseURL + "/wp-json" + endpoint

	// Log HTTP request with payload
	l := logger.Get()
	op := l.StartOperation("wordpress_http_request")
	op.WithContext("http_method", method)
	op.WithContext("http_url", url)
	op.WithContext("http_endpoint", endpoint)
	
	if body != nil {
		// Log the request payload
		op.WithContext("http_request_payload", string(body))
		op.WithContext("http_request_payload_size", len(body))
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		op.Fail("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", s.authHeader)

	debug.Printf("WordPress API Request - Method: %s, URL: %s, Auth Header: %s", method, url, s.authHeader)

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
		op.WithContext("http_response_body", string(bodyBytes))
		op.Fail(fmt.Sprintf("WordPress API Error - Status: %d", resp.StatusCode), fmt.Errorf("%s", string(bodyBytes)))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Complete the operation successfully
	op.Complete(fmt.Sprintf("HTTP %s request to %s completed", method, endpoint))

	return resp, nil
}

func (s *WordPressService) GetCategoryIDsByNames(year string,categoryNames []string) ([]int, error) {
	l := logger.Get()
	op := l.StartOperation("wordpress_get_category_ids")
	op.WithContext("year", year)
	op.WithContext("category_names", categoryNames)
	
	var categoryIDs []int
	foundCount := 0
	notFoundCount := 0

	for _, categoryName := range categoryNames {

		if strings.TrimSpace(categoryName) == "" {
			continue
		}

		categories, err := s.SearchCategories(year)
		if err != nil {
			op.Warn(&logger.WideEvent{
				Message: fmt.Sprintf("Failed to search for category '%s'", categoryName),
			})
			notFoundCount++
			continue
		}

		var foundCategory *WordPressCategory
		for _, category := range categories {
			if strings.Contains(strings.ToLower(category.Name), strings.ReplaceAll(strings.ToLower(categoryName), "" , "-")) {
				foundCategory = category
				break
			}
		}

		if foundCategory == nil {
			for _, category := range categories {
				if strings.Contains(strings.ToLower(category.Name), strings.ToLower(strings.TrimSpace(categoryName))) {
					foundCategory = category
					break
				}
			}
		}

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
