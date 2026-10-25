package repositories

import (
	"gorm.io/gorm"
)

type Repository[T any] struct{ DB *gorm.DB }

func New[T any](db *gorm.DB) Repository[T] { return Repository[T]{DB: db} }

func (r Repository[T]) Create(item *T) error { return r.DB.Create(item).Error }

func (r Repository[T]) Get(id uint, ownerID uint, admin bool) (T, error) {
	var item T
	query := r.DB.First(&item, id)
	if !admin && ownerID != 0 {
		query = r.DB.Where("owner_id = ?", ownerID).First(&item, id)
	}
	return item, query.Error
}

func (r Repository[T]) List(ownerID uint, admin bool, offset, limit int, filters map[string]any, order string) ([]T, int64, error) {
	var items []T
	var total int64
	query := r.DB.Model(new(T))
	if !admin && ownerID != 0 {
		query = query.Where("owner_id = ?", ownerID)
	}
	for field, value := range filters {
		query = query.Where(field+" = ?", value)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order(order).Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository[T]) Update(item *T, id uint, ownerID uint, admin bool) error {
	query := r.DB.Model(new(T)).Where("id = ?", id)
	if !admin && ownerID != 0 {
		query = query.Where("owner_id = ?", ownerID)
	}
	result := query.Updates(item)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r Repository[T]) Delete(id uint, ownerID uint, admin bool) error {
	query := r.DB.Delete(new(T), id)
	if !admin && ownerID != 0 {
		query = r.DB.Where("owner_id = ?", ownerID).Delete(new(T), id)
	}
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
