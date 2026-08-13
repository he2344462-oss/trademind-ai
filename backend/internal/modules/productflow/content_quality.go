package productflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) CreateContentQualityReview(ctx context.Context, tenantID int64, listingID uuid.UUID, actor *uuid.UUID, body ContentQualityReviewBody) (*ListingContentQualityReview, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, listingID)
	if err != nil {
		return nil, err
	}
	if listing.CurrentContentVersionID == nil {
		return nil, ErrNotFound
	}
	status := strings.ToLower(strings.TrimSpace(body.Status))
	if status != "approved" && status != "needs_edit" && status != "rejected" {
		return nil, fmt.Errorf("%w: invalid quality status", ErrValidation)
	}
	for _, score := range []int{body.TitleQuality, body.DescriptionQuality, body.FactAccuracy} {
		if score < 0 || score > 100 {
			return nil, fmt.Errorf("%w: quality scores must be 0-100", ErrValidation)
		}
	}
	row := ListingContentQualityReview{TenantID: tenantID, ListingDraftID: listingID, ContentVersionID: *listing.CurrentContentVersionID, Status: status, TitleQuality: body.TitleQuality, DescriptionQuality: body.DescriptionQuality, FactAccuracy: body.FactAccuracy, WasEdited: body.WasEdited, EditReason: safeFileText(body.EditReason, 2000), CreatedBy: actor}
	if err = s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
