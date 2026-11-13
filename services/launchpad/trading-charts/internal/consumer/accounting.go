package consumer

import (
	"context"
	"errors"
	"regexp"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

type ReprocessRequest struct{ PublicationID, PreviousDigest, CorrectedDigest, DecoderVersion, OperatorID string }

func NewReprocessingAccountingConsumer(ctx context.Context, config SourceVerifiedAccountingConfig, request ReprocessRequest) (*AccountingConsumer, error) {
	if !config.SeriesV2 || config.Exact == nil || !regexp.MustCompile(`^reprocess_[a-zA-Z0-9_-]{1,100}$`).MatchString(config.Generation) || request.DecoderVersion == "" || request.OperatorID == "" {
		return nil, errors.New("charts: isolated audited reprocess generation required")
	}
	consumer, err := NewSourceVerifiedAccountingConsumer(config)
	if err != nil {
		return nil, err
	}
	if err := config.Exact.AuthorizeReprocess(ctx, config.ConsumerID, config.Generation, request.PublicationID, request.PreviousDigest, request.CorrectedDigest, request.DecoderVersion, request.OperatorID); err != nil {
		return nil, err
	}
	consumer.reprocess = &request
	return consumer, nil
}

func (c *AccountingConsumer) RefreshCompleteness(ctx context.Context, publication accounting.PublicationV1) error {
	if c == nil || c.completeness == nil {
		return ErrAccountingAuthority
	}
	return c.completeness.refresh(ctx, publication)
}
