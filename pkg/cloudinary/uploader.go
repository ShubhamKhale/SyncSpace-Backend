package cloudinary

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	cld "github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Uploader wraps the Cloudinary SDK for avatar uploads.
type Uploader struct {
	client *cld.Cloudinary
}

// NewUploader creates an Uploader authenticated with the given credentials.
func NewUploader(cloudName, apiKey, apiSecret string) (*Uploader, error) {
	client, err := cld.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init: %w", err)
	}
	client.Config.API.Timeout = 15 // seconds
	return &Uploader{client: client}, nil
}

// allowedExts lists accepted image extensions (lowercase).
var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// UploadAvatar uploads a user avatar image to Cloudinary and returns its secure URL.
// publicID is set to "syncspace/avatars/<userID>" so re-uploads replace the same asset.
func (u *Uploader) UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userID string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		return "", fmt.Errorf("unsupported file type %q: only jpg, jpeg, png allowed", ext)
	}
	if header.Size > 1<<20 {
		return "", fmt.Errorf("file too large: max 1 MB")
	}

	overwrite := true
	result, err := u.client.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID:     "syncspace/avatars/" + userID,
		Overwrite:    &overwrite,
		ResourceType: "image",
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload: %w", err)
	}

	return result.SecureURL, nil
}
