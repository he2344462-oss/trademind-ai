package productflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

const maxListingImageBytes int64 = 10 * 1024 * 1024

func assetCacheBase() string { return filepath.Join(os.TempDir(), "trademind-listing-assets") }

func listingAssetRoot(tenantID int64, listingID uuid.UUID) string {
	return filepath.Join(assetCacheBase(), fmt.Sprintf("tenant-%d", tenantID), listingID.String())
}

func pathWithin(root, path string) bool {
	r, err1 := filepath.Abs(filepath.Clean(root))
	p, err2 := filepath.Abs(filepath.Clean(path))
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(r, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func detectImage(data []byte, declared string) (string, string, error) {
	if len(data) == 0 || int64(len(data)) > maxListingImageBytes {
		return "", "", fmt.Errorf("%w: image must be 1-%d bytes", ErrValidation, maxListingImageBytes)
	}
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data[:min(len(data), 512)]), ";")[0]))
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}
	extension, ok := ext[detected]
	if !ok || (declared != "" && declared != "application/octet-stream" && declared != detected) {
		return "", "", fmt.Errorf("%w: MIME and file signature do not match a supported image", ErrValidation)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return "", "", fmt.Errorf("%w: invalid image payload", ErrValidation)
	}
	return detected, extension, nil
}

func (s *Service) UploadListingAsset(ctx context.Context, tenantID int64, listingID uuid.UUID, originalName, declaredType string, src io.Reader) (*ListingAsset, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, listingID)
	if err != nil {
		return nil, err
	}
	var count int64
	if err = s.DB.WithContext(ctx).Model(&ListingAsset{}).Where("tenant_id=? AND listing_draft_id=? AND excluded=false", tenantID, listingID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= maxPackageImages {
		return nil, fmt.Errorf("%w: listing image limit reached", ErrValidation)
	}
	limited := io.LimitReader(src, maxListingImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	mimeType, ext, err := detectImage(data, declaredType)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var existing ListingAsset
	if e := s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=? AND content_hash=? AND source_type=?", tenantID, listingID, hash, "operator_upload").First(&existing).Error; e == nil {
		existing.Cached = pathWithin(listingAssetRoot(tenantID, listingID), existing.CachePath)
		return &existing, nil
	}
	root := listingAssetRoot(tenantID, listingID)
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	name := uuid.NewString() + ext
	path := filepath.Join(root, name)
	if !pathWithin(root, path) {
		return nil, fmt.Errorf("%w: unsafe asset path", ErrValidation)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}
	row := ListingAsset{TenantID: tenantID, CatalogProductID: listing.CatalogProductID, ListingDraftID: &listingID, SourceType: "operator_upload", SourceURL: "upload://" + uuid.NewString(), SourceReference: filepath.Base(originalName), ContentHash: hash, MimeType: mimeType, SizeBytes: int64(len(data)), SortOrder: int(count), IsPrimary: count == 0, CachePath: path, Cached: true}
	if err = s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListingAssetFile(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingAsset, error) {
	var row ListingAsset
	err := s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, id).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.ListingDraftID == nil || !pathWithin(listingAssetRoot(tenantID, *row.ListingDraftID), row.CachePath) {
		return nil, fmt.Errorf("%w: asset is not in controlled cache", ErrValidation)
	}
	data, err := os.ReadFile(row.CachePath)
	if err != nil {
		return nil, ErrNotFound
	}
	if _, _, err = detectImage(data, row.MimeType); err != nil {
		return nil, err
	}
	row.Cached = true
	return &row, nil
}

func (s *Service) DeleteListingAsset(ctx context.Context, tenantID int64, listingID, assetID uuid.UUID) error {
	var row ListingAsset
	if err := s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=? AND id=?", tenantID, listingID, assetID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrNotFound
		}
		return err
	}
	if row.SourceType != "operator_upload" {
		return fmt.Errorf("%w: source images are read-only", ErrValidation)
	}
	if !pathWithin(listingAssetRoot(tenantID, listingID), row.CachePath) {
		return fmt.Errorf("%w: unsafe asset path", ErrValidation)
	}
	if err := s.DB.WithContext(ctx).Delete(&row).Error; err != nil {
		return err
	}
	if err := os.Remove(row.CachePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func trustedAssetBytes(tenantID int64, listingID uuid.UUID, asset ListingAsset) ([]byte, error) {
	if !pathWithin(listingAssetRoot(tenantID, listingID), asset.CachePath) {
		return nil, fmt.Errorf("unsafe cache path")
	}
	data, err := os.ReadFile(asset.CachePath)
	if err != nil {
		return nil, err
	}
	mimeType, _, err := detectImage(data, asset.MimeType)
	if err != nil {
		return nil, err
	}
	if mimeType != asset.MimeType {
		return nil, fmt.Errorf("image MIME changed")
	}
	return data, nil
}
