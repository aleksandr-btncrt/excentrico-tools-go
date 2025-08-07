# Excentrico Tools Go

A comprehensive Go application for film festival management that integrates with Google Sheets, Google Drive, image processing, WordPress API, and generates Divi templates for WordPress.

## Features

- **Google Sheets Integration**: Read, write, and manage film festival spreadsheet data
- **Google Drive Integration**: Automatically download and process film assets from shared folders
- **Image Processing**: Resize, crop, convert, and optimize images for web use
- **WordPress API**: Create, update, and manage film posts with media uploads
- **Divi Template Generation**: Generate complete Divi Builder templates with film data and images

## Prerequisites

- Go 1.21 or higher
- Google Cloud Platform account with APIs enabled
- WordPress site with REST API enabled and Divi theme
- Turso database account (for metadata storage)

## Setup

### 1. Clone the repository

```bash
git clone <repository-url>
cd excentrico-tools-go
```

### 2. Install dependencies

```bash
go mod tidy
```

### 3. Google Cloud Setup

1. Go to the [Google Cloud Console](https: 
2. Create a new project or select an existing one
3. Enable the following APIs:
   - Google Sheets API
   - Google Drive API
4. Create a service account and download the credentials JSON file
5. Place the credentials file in the project root as `credentials.json`

### 4. Configuration Setup

#### Option A: Create default configuration (Recommended)

```bash
# Build the application
go build -o excentrico-tools-go

# Create a default configuration file
./excentrico-tools-go -create-config
```

This will create a `configuration.json` file with default values. Edit this file with your actual settings.

#### Option B: Manual configuration

Copy the example configuration file:

```bash
cp configuration.json.example configuration.json
```

Then edit `configuration.json` with your settings:

```json
{
  "google_credentials_path": "credentials.json",
  "google_sheet_id": "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
  "wordpress_config": {
    "base_url": "https: 
    "username": "your-username",
    "password": "",
    "application_password": "your-application-password"
  },
  "turso_config": {
    "database_url": "libsql: 
    "auth_token": "your-auth-token"
  },
  "image_config": {
    "max_width": 1920,
    "max_height": 1080,
    "quality": 85
  }
}
```

### 5. Run the application

```bash
# Process films for a specific year
./excentrico-tools-go -year 2024

# Enable debug logging
./excentrico-tools-go -debug -year 2024

# Process all films
./excentrico-tools-go
```

## Film Processing Workflow

The application provides a complete workflow for processing film festival submissions:

### 1. Data Import
- Reads film data from Google Sheets
- Validates and parses film information
- Filters films by year or other criteria

### 2. Asset Processing
- Downloads images from Google Drive folders (specified in ENLACES column)
- Optimizes images for web use (creates `_web.jpg` versions)
- Organizes files in structured directories

### 3. WordPress Integration
- Uploads optimized images to WordPress Media Library
- Creates or updates WordPress posts with film information
- Associates media with posts

### 4. Divi Template Generation
- Generates complete Divi Builder templates with film data
- Matches director photos automatically
- Creates structured JSON templates with:
  - Film information and credits
  - Director sections with bios and photos
  - Image galleries
  - Styled sections with festival branding

### 5. Metadata Storage
- Tracks processing status in Turso database
- Stores WordPress post IDs and media mappings
- Maintains file processing history

## Configuration

### Required Files

The application requires three configuration files:

1. **`credentials.json`** - Google API credentials file
   - Download from Google Cloud Console
   - Contains service account credentials for Google Sheets and Drive APIs

2. **`configuration.json`** - Application configuration
   - Contains WordPress settings, Google Sheet ID, database config, and image processing options
   - Can be created automatically with `-create-config` flag

3. **Database Configuration** - Turso database for metadata storage
   - Stores processing status and WordPress metadata
   - Prevents duplicate uploads and processing

### Configuration Options

| Field | Description | Required | Default |
|-------|-------------|----------|---------|
| `google_credentials_path` | Path to Google API credentials file | Yes | `credentials.json` |
| `google_sheet_id` | Default Google Sheet ID to use | No | - |
| `wordpress_config.base_url` | WordPress site URL | Yes | - |
| `wordpress_config.username` | WordPress username | Yes | - |
| `wordpress_config.password` | WordPress password | No* | - |
| `wordpress_config.application_password` | WordPress application password | No* | - |
| `turso_config.database_url` | Turso database URL | Yes | - |
| `turso_config.auth_token` | Turso authentication token | Yes | - |
| `image_config.max_width` | Maximum image width for resizing | No | `1920` |
| `image_config.max_height` | Maximum image height for resizing | No | `1080` |
| `image_config.quality` | JPEG quality for image processing | No | `85` |

*Either `password` or `application_password` is required for WordPress authentication.

## Usage Examples

### Film Processing

```bash
# Process all films from 2024
./excentrico-tools-go -year 2024

# Process all films with debug logging
./excentrico-tools-go -debug

# Create initial configuration
./excentrico-tools-go -create-config
```

### Google Sheets

```go
 
data, err := sheetsService.ReadRange("spreadsheet-id", "Sheet1!A1:D10")
if err != nil {
    log.Fatal(err)
}

 
values := [][]interface{}{
    {"Name", "Email", "Phone"},
    {"John Doe", "john@example.com", "123-456-7890"},
}
err = sheetsService.WriteRange("spreadsheet-id", "Sheet1!A1:C2", values)

 
spreadsheet, err := sheetsService.CreateSpreadsheet("My New Spreadsheet")
```

### Film Data Management

```go
 
filmService := services.NewFilmDataService(sheetsService, sheetID)

 
filmService.LoadHeaders()
films, err := filmService.ReadAllFilms()

 
newFilm := &services.FilmData{
    TituloOriginal: "Example Film",
    Direccion:      "Example Director",
    Pais:           "Spain",
    Ano:            "2024",
     
}
err = filmService.AddFilm(newFilm)

 
for _, film := range films {
    if film.Pais == "Spain" {
         
    }
}
```

### Divi Template Generation

```go
 
diviService := services.NewDiviTemplateService()

 
templateData, shortcodes := diviService.GenerateCompleteTemplate(filmData, imageIds, wordpressService)

 
err := diviService.SaveDiviTemplateToFile(filmData, imageIds, wordpressService, filmDir)
```

### Google Drive

```go
 
file, err := driveService.UploadFile("local-file.jpg", "remote-file.jpg", "folder-id")

 
err = driveService.DownloadFile("file-id", "downloaded-file.jpg")

 
files, err := driveService.ListFiles("folder-id")
```

### Image Processing

```go
 
err := imageService.ResizeImage("input.jpg", "output.jpg")

 
err := imageService.CreateThumbnail("input.jpg", "thumbnail.jpg", 150, 150)

 
err := imageService.ConvertFormat("input.png", "output.jpg")

 
err := imageService.CropImage("input.jpg", "cropped.jpg", 100, 100, 500, 500)
```

### WordPress API

```go
 
post := &WordPressPost{
    Title:   "My New Post",
    Content: "This is the content of my post.",
    Status:  "publish",
}
createdPost, err := wordpressService.CreatePost(post)

 
posts, err := wordpressService.GetPosts(map[string]string{
    "per_page": "10",
    "status":   "publish",
})

 
media, err := wordpressService.UploadMedia(fileHeader, "Image Title", "Alt text")
```

## Divi Template Structure

The generated Divi templates follow a specific JSON structure compatible with Divi Builder:

```json
{
  "context": "et_builder",
  "data": {
    "film_title": "[et_pb_section...divi_shortcode_content...]"
  },
  "presets": {
    "et_pb_row": {
      "presets": {
        "_initial": {
          "name": "Fila Preset 1",
          "version": "4.6.1",
          "settings": {
            "use_custom_gutter": "off",
            "gutter_width": "1",
            "width": "90%",
            "module_alignment": "center"
          }
        }
      },
      "default": "_initial"
    }
  },
  "global_colors": [
    ["gcid-primary-color", {"color": "#31045c", "active": "yes"}],
    ["gcid-secondary-color", {"color": "#24a68e", "active": "yes"}],
    ["gcid-heading-color", {"color": "#e6f543", "active": "yes"}],
    ["gcid-body-color", {"color": "#333333", "active": "yes"}]
  ],
  "images": {
    "https: 
      "encoded": "base64_encoded_image_data",
      "url": "https: 
      "id": 123
    }
  },
  "thumbnails": []
}
```

### Director Image Matching

The system automatically matches director photos with uploaded images using:

- **Image titles** containing director names
- **Filenames** containing director names
- **Alt text** containing director names  
- **Partial name matching** for complex names

Example matching scenarios:
- Director "María García" matches image titled "maria-garcia-director.jpg"
- Director "John Smith" matches alt text "Photo of john smith"
- Director "Alex Johnson" matches filename "alex_headshot.png"

## Project Structure

```
excentrico-tools-go/
├── main.go                 # Application entry point and main processing logic
├── go.mod                  # Go module file
├── go.sum                  # Go module checksums
├── README.md              # This file
├── configuration.json.example  # Example configuration file
├── credentials.json       # Google API credentials (not in repo)
├── configuration.json     # Application configuration (not in repo)
├── films/                 # Generated film directories (created during processing)
│   ├── film_name_1/
│   │   ├── divi_template.json
│   │   ├── original_image.jpg
│   │   ├── original_image_web.jpg
│   │   └── Director/
│   └── film_name_2/
├── internal/
│   ├── config/
│   │   └── config.go      # Configuration management
│   ├── debug/
│   │   └── debug.go       # Debug utilities
│   └── services/
│       ├── google_sheets.go    # Google Sheets service
│       ├── google_drive.go     # Google Drive service  
│       ├── image_service.go    # Image processing service
│       ├── wordpress.go        # WordPress API service
│       ├── film_data_service.go # Film data management service
│       ├── divi_template.go    # Divi template generation service
│       └── turso_service.go    # Database service for metadata storage
└── examples/
    ├── basic_usage.go     # Basic API usage examples
    └── film_data/         # Film data processing examples
```

## Generated Files

For each processed film, the application creates:

- **Film directory**: `films/{sanitized_film_title}/`
- **Original images**: Downloaded from Google Drive
- **Optimized images**: `*_web.jpg` versions for web use
- **Divi template**: `divi_template.json` with complete template data
- **Metadata**: Stored in Turso database for tracking

## Examples

### Basic Usage

Run the basic examples:

```bash
go run examples/basic_usage.go
```

### Film Data Management

Run the film data example:

```bash
cd examples/film_data
go run main.go
```

This example demonstrates:
- Reading film data from Google Sheets
- Adding new films
- Searching and filtering films
- Sheet statistics and analysis

## Building and Deployment

### Build the application

```bash
go build -o excentrico-tools-go
```

Or use the build script:

```bash
./build.sh
```

### Deploy with configuration

When deploying the application, ensure both configuration files are in the same directory as the executable:

```
deployment/
├── excentrico-tools-go
├── configuration.json
└── credentials.json
```

## Security Notes

- Never commit your `credentials.json` or `configuration.json` files to version control
- Use application passwords for WordPress instead of your main password
- Keep your Google API credentials secure and rotate them regularly
- Secure your Turso database credentials
- The application validates that required configuration files exist before starting

## Error Handling

All services include comprehensive error handling and logging. The application will:

- Validate configuration files exist and are properly formatted
- Check that Google credentials file is accessible
- Verify WordPress configuration is complete
- Test database connectivity
- Provide helpful error messages and suggestions for fixing issues
- Continue processing other films if one fails

## Troubleshooting

### Common Issues

1. **"Configuration error: failed to read configuration file"**
   - Run `./excentrico-tools-go -create-config` to create a default configuration
   - Edit the generated `configuration.json` file with your settings

2. **"Google credentials file not found"**
   - Ensure `credentials.json` is in the same directory as the executable
   - Verify the file path in `configuration.json` is correct

3. **"WordPress authentication failed"**
   - Check your WordPress username and password/application password
   - Ensure the WordPress site URL is correct and accessible

4. **"Database connection failed"**
   - Verify your Turso database URL and auth token
   - Check that the database is accessible

5. **"Google Sheet access denied"**
   - Ensure your service account has access to the specified Google Sheet
   - Share the Google Sheet with your service account email address
   - Verify the Sheet ID is correct

6. **"Film data parsing errors"**
   - Ensure the Google Sheet has the correct header structure
   - Check that the first row contains the expected column headers
   - Verify data is properly formatted in the sheet

7. **"Director images not found"**
   - Ensure director photos are uploaded to WordPress
   - Check that image titles or filenames contain director names
   - Verify images are properly associated with the film

8. **"Divi template generation failed"**
   - Check that all required film data fields are present
   - Verify WordPress images are accessible
   - Ensure director names match uploaded image metadata

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 