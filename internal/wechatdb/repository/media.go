package repository

import (
	"context"

	"github.com/TE0dollary/chatlog-bot/internal/model"
)

func (r *Repository) GetMedia(ctx context.Context, _type string, key string) (*model.Media, error) {
	return r.ds.GetMedia(ctx, _type, key)
}
