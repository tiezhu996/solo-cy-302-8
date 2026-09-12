package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/gbexam/online-exam/internal/model"
)

// ExtendExamEndTime atomically moves an exam's end time to newEnd, shifts the
// personal deadline of every in-progress attempt by delta, and appends the
// extension audit record. All three writes commit in one transaction: if any
// fails, the transaction rolls back and both times (and the record log) keep
// their original values. Returns the affected attempt count.
func (r *Repository) ExtendExamEndTime(ctx context.Context, examID uint, newEnd time.Time, delta time.Duration, record *model.ExamExtension) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Exam{}).Where("id = ?", examID).Update("end_time", newEnd)
		if res.Error != nil {
			return fmt.Errorf("update exam end time: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		res = tx.Model(&model.ExamAttempt{}).
			Where("exam_id = ? AND status = ?", examID, "in_progress").
			Update("deadline", gorm.Expr("DATE_ADD(deadline, INTERVAL ? MICROSECOND)", delta.Microseconds()))
		if res.Error != nil {
			return fmt.Errorf("shift in-progress deadlines: %w", res.Error)
		}
		affected = res.RowsAffected
		record.ExamID = examID
		record.AffectedAttempts = int(affected)
		if err := tx.Create(record).Error; err != nil {
			return fmt.Errorf("create exam extension record: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// ListExamExtensions returns the extension records of an exam, newest first.
// Records are append-only: no update or delete paths exist for them.
func (r *Repository) ListExamExtensions(ctx context.Context, examID uint) ([]model.ExamExtension, error) {
	var records []model.ExamExtension
	if err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Order("id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list exam extensions: %w", err)
	}
	return records, nil
}
