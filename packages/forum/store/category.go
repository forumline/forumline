package store

import (
	"context"


	"github.com/forumline/forumline/forum/oapi"
)

// ListCategories returns all categories ordered by sort_order.
func (s *Store) ListCategories(ctx context.Context) ([]oapi.Category, error) {
	rows, err := s.Q.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	categories := make([]oapi.Category, 0, len(rows))
	for _, r := range rows {
		categories = append(categories, oapi.Category{
			Id:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			Description: pgtextPtr(r.Description),
			SortOrder:   int(r.SortOrder),
			CreatedAt:   tsTime(r.CreatedAt),
		})
	}
	return categories, nil
}

// GetCategoryBySlug returns a category by its slug.
func (s *Store) GetCategoryBySlug(ctx context.Context, slug string) (*oapi.Category, error) {
	row, err := s.Q.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	c := oapi.Category{
		Id:          row.ID,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: pgtextPtr(row.Description),
		SortOrder:   int(row.SortOrder),
		CreatedAt:   tsTime(row.CreatedAt),
	}
	return &c, nil
}
