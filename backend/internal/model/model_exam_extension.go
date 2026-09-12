package model

import "time"

// ExamExtension is an append-only audit record of one exam end-time extension.
// Records are created together with the extension and never updated or deleted.
type ExamExtension struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ExamID           uint      `gorm:"index;not null" json:"exam_id"`
	OperatorID       uint      `gorm:"index;not null" json:"operator_id"`
	OperatorName     string    `gorm:"size:64" json:"operator_name"`
	OldEndTime       time.Time `json:"old_end_time"`
	NewEndTime       time.Time `json:"new_end_time"`
	ExtendMinutes    float64   `gorm:"not null;default:0" json:"extend_minutes"`
	AffectedAttempts int       `gorm:"not null;default:0" json:"affected_attempts"`
	CreatedAt        time.Time `json:"created_at"`
}
