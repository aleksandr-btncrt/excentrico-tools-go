package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"excentrico-tools-go/internal/config"
	"excentrico-tools-go/internal/debug"
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

func (s *WordPressService) CreatePost(post *WordPressPost) (*WordPressPost, error) {
	jsonData, err := json.Marshal(post)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal post: %v", err)
	}

	resp, err := s.makeRequest("POST", "/wp/v2/project", jsonData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var createdPost WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&createdPost); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Created WordPress post: %s (ID: %d)", createdPost.Title.String(), createdPost.ID)
	return &createdPost, nil
}

func (s *WordPressService) UpdatePost(postID int, post *WordPressPost) (*WordPressPost, error) {
	jsonData, err := json.Marshal(post)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal post: %v", err)
	}

	resp, err := s.makeRequest("PUT", fmt.Sprintf("/wp/v2/project/%d", postID), jsonData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var updatedPost WordPressPost
	if err := json.NewDecoder(resp.Body).Decode(&updatedPost); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Updated WordPress post: %s (ID: %d)", updatedPost.Title.String(), updatedPost.ID)
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
	fileReader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer fileReader.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", file.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = io.Copy(part, fileReader)
	if err != nil {
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
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", s.authHeader)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var media WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Uploaded media: %s (ID: %d)", media.Title.String(), media.ID)
	return &media, nil
}

func (s *WordPressService) UploadMediaFromFile(filePath, title, altText string) (*WordPressMedia, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", fileInfo.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
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
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", s.authHeader)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var media WordPressMedia
	if err := json.NewDecoder(resp.Body).Decode(&media); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Uploaded media from file: %s (ID: %d)", media.Title.String(), media.ID)
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

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", s.authHeader)

	debug.Printf("WordPress API Request - Method: %s, URL: %s, Auth Header: %s", method, url, s.authHeader)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("WordPress API Error - Status: %d, URL: %s, Response: %s", resp.StatusCode, req.URL.String(), string(bodyBytes))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

func (s *WordPressService) GetCategoryIDsByNames(year string,categoryNames []string) ([]int, error) {
	var categoryIDs []int

	for _, categoryName := range categoryNames {

		if strings.TrimSpace(categoryName) == "" {
			continue
		}

		categories, err := s.SearchCategories(year)
		if err != nil {
			log.Printf("Warning: Failed to search for category '%s': %v", categoryName, err)
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
			log.Printf("Found category '%s' with ID %d", foundCategory.Name, foundCategory.ID)
		} else {
			log.Printf("Warning: Category '%s' not found in WordPress", categoryName)
		}
	}

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
