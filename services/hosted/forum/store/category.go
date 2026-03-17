package store

import (
	"context"

	"github.com/forumline/forumline/services/hosted/forum/model"
)

// ListCategories returns all categories ordered by sort_order.
func (s *Store) ListCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.Q.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	categories := make([]model.Category, 0, len(rows))
	for _, r := range rows {
		categories = append(categories, model.Category{
			ID:          uuidStr(r.ID),
			Name:        r.Name,
			Slug:        r.Slug,
			Description: pgtextPtr(r.Description),
			SortOrder:   int(r.SortOrder),
			CreatedAt:   tsStr(r.CreatedAt),
		})
	}
	return categories, nil
}

// GetCategoryBySlug returns a category by its slug.
func (s *Store) GetCategoryBySlug(ctx context.Context, slug string) (*model.Category, error) {
	row, err := s.Q.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	c := model.Category{
		ID:          uuidStr(row.ID),
		Name:        row.Name,
		Slug:        row.Slug,
		Description: pgtextPtr(row.Description),
		SortOrder:   int(row.SortOrder),
		CreatedAt:   tsStr(row.CreatedAt),
	}
	return &c, nil
}
