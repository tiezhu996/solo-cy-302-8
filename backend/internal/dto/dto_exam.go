package dto

import "time"

// PaperQuestionConfig describes how many questions of a type to draw.
type PaperQuestionConfig struct {
	Type       string  `json:"type" binding:"required,oneof=single multiple true_false fill_blank short_answer"`
	Count      int     `json:"count" binding:"required,min=1"`
	Score      float64 `json:"score" binding:"required,min=0.5"`
	Difficulty string  `json:"difficulty" binding:"omitempty,oneof=easy medium hard"`
}

// ExamCreateRequest is used by teachers to create an auto-generated paper.
type ExamCreateRequest struct {
	Title           string                `json:"title" binding:"required,max=128"`
	Description     string                `json:"description" binding:"max=2000"`
	DurationMinutes int                   `json:"duration_minutes" binding:"required,min=1,max=1440"`
	TotalScore      float64               `json:"total_score" binding:"omitempty,min=0"`
	StartTime       *time.Time            `json:"start_time"`
	EndTime         *time.Time            `json:"end_time"`
	QuestionConfig  []PaperQuestionConfig `json:"question_config" binding:"required,min=1,dive"`
}

// ExamListQuery filters exam list.
type ExamListQuery struct {
	PageQuery
	Status string `form:"status" binding:"omitempty,oneof=draft published closed"`
	Keyword string `form:"keyword"`
}

// ExamExtendRequest postpones a published exam's end time.
type ExamExtendRequest struct {
	EndTime time.Time `json:"end_time" binding:"required"`
}

// ExamExtendResponse describes the applied postponement.
type ExamExtendResponse struct {
	ID               uint      `json:"id"`
	OldEndTime       time.Time `json:"old_end_time"`
	NewEndTime       time.Time `json:"new_end_time"`
	ExtendMinutes    float64   `json:"extend_minutes"`
	AffectedAttempts int       `json:"affected_attempts"`
}

// ExamExtensionResponse is one append-only exam extension audit record.
type ExamExtensionResponse struct {
	ID               uint      `json:"id"`
	ExamID           uint      `json:"exam_id"`
	OperatorID       uint      `json:"operator_id"`
	OperatorName     string    `json:"operator_name"`
	OldEndTime       time.Time `json:"old_end_time"`
	NewEndTime       time.Time `json:"new_end_time"`
	ExtendMinutes    float64   `json:"extend_minutes"`
	AffectedAttempts int       `json:"affected_attempts"`
	CreatedAt        time.Time `json:"created_at"`
}

// ExamResponse is the paper metadata.
type ExamResponse struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	TotalScore      float64    `json:"total_score"`
	DurationMinutes int        `json:"duration_minutes"`
	StartTime       *time.Time `json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	Status          string     `json:"status"`
	QuestionCount   int        `json:"question_count"`
	CreatedBy       uint       `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ExamQuestionResponse is one paper question (teacher/admin view includes answer).
type ExamQuestionResponse struct {
	ID      uint    `json:"id"`
	Score   float64 `json:"score"`
	Question QuestionResponse `json:"question"`
}
