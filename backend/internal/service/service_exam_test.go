package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
	"github.com/gbexam/online-exam/internal/repository"
)

// fakeExamRepo is an in-memory ExamRepo for exam service tests.
type fakeExamRepo struct {
	exam        *model.Exam
	affected    int64
	extendErr   error
	extendCalls int
	updateCalls int
	extendEnd   time.Time
	extendDelta time.Duration
}

func (f *fakeExamRepo) CreateExam(_ context.Context, _ *model.Exam) error { return nil }

func (f *fakeExamRepo) FindExamByID(_ context.Context, id uint) (*model.Exam, error) {
	if f.exam == nil || f.exam.ID != id {
		return nil, repository.ErrNotFound
	}
	cp := *f.exam
	return &cp, nil
}

func (f *fakeExamRepo) UpdateExam(_ context.Context, exam *model.Exam) error {
	f.updateCalls++
	cp := *exam
	f.exam = &cp
	return nil
}

func (f *fakeExamRepo) DeleteExam(_ context.Context, _ uint) error { return nil }

func (f *fakeExamRepo) ListExams(_ context.Context, _ repository.ExamFilter, _, _ int) ([]model.Exam, int64, error) {
	return nil, 0, nil
}

func (f *fakeExamRepo) ReplaceExamQuestions(_ context.Context, _ uint, _ []model.ExamQuestion) error {
	return nil
}

func (f *fakeExamRepo) ListExamQuestions(_ context.Context, _ uint) ([]model.ExamQuestion, error) {
	return nil, nil
}

func (f *fakeExamRepo) CountExamQuestions(_ context.Context, _ uint) (int64, error) { return 0, nil }

// ExtendExamEndTime applies the end-time change and the deadline shift as one
// operation. When extendErr is set it fails without touching stored state,
// mirroring a rolled-back transaction.
func (f *fakeExamRepo) ExtendExamEndTime(_ context.Context, examID uint, newEnd time.Time, delta time.Duration) (int64, error) {
	f.extendCalls++
	if f.extendErr != nil {
		return 0, f.extendErr
	}
	if f.exam == nil || f.exam.ID != examID {
		return 0, repository.ErrNotFound
	}
	f.extendEnd = newEnd
	f.extendDelta = delta
	end := newEnd
	f.exam.EndTime = &end
	return f.affected, nil
}

// fakeQuestionRepo satisfies QuestionRepo for tests that never touch questions.
type fakeQuestionRepo struct{}

func (fakeQuestionRepo) CreateQuestion(_ context.Context, _ *model.Question) error { return nil }
func (fakeQuestionRepo) CreateQuestionsBatch(_ context.Context, _ []model.Question) error {
	return nil
}
func (fakeQuestionRepo) FindQuestionByID(_ context.Context, _ uint) (*model.Question, error) {
	return nil, repository.ErrNotFound
}
func (fakeQuestionRepo) UpdateQuestion(_ context.Context, _ *model.Question) error { return nil }
func (fakeQuestionRepo) DeleteQuestion(_ context.Context, _ uint) error             { return nil }
func (fakeQuestionRepo) ListQuestions(_ context.Context, _ repository.QuestionFilter, _, _ int) ([]model.Question, int64, error) {
	return nil, 0, nil
}
func (fakeQuestionRepo) ListQuestionsByTypeDifficulty(_ context.Context, _, _ string) ([]model.Question, error) {
	return nil, nil
}
func (fakeQuestionRepo) FindQuestionsByIDs(_ context.Context, _ []uint) (map[uint]model.Question, error) {
	return nil, nil
}

func TestExamServiceExtend(t *testing.T) {
	now := time.Now()
	oldEnd := now.Add(2 * time.Hour)
	newEnd := now.Add(3 * time.Hour)

	examWith := func(status string, end *time.Time) *model.Exam {
		return &model.Exam{ID: 7, Title: "期中考试", Status: status, CreatedBy: 42, EndTime: end}
	}
	published := func() *model.Exam {
		end := oldEnd
		return examWith(constants.ExamPublished, &end)
	}

	tests := []struct {
		name    string
		exam    *model.Exam
		role    string
		userID  uint
		endTime time.Time
		wantErr error
	}{
		{name: "teacher extends own exam", exam: published(), role: constants.RoleTeacher, userID: 42, endTime: newEnd},
		{name: "admin extends any exam", exam: published(), role: constants.RoleAdmin, userID: 1, endTime: newEnd},
		{name: "teacher cannot extend others exam", exam: published(), role: constants.RoleTeacher, userID: 99, endTime: newEnd, wantErr: ErrForbidden},
		{name: "closed exam rejected", exam: examWith(constants.ExamClosed, &oldEnd), role: constants.RoleAdmin, userID: 1, endTime: newEnd, wantErr: ErrValidation},
		{name: "draft exam rejected", exam: examWith(constants.ExamDraft, &oldEnd), role: constants.RoleAdmin, userID: 1, endTime: newEnd, wantErr: ErrValidation},
		{name: "missing end time rejected", exam: examWith(constants.ExamPublished, nil), role: constants.RoleAdmin, userID: 1, endTime: newEnd, wantErr: ErrValidation},
		{name: "moving end time backwards rejected", exam: published(), role: constants.RoleAdmin, userID: 1, endTime: oldEnd.Add(-time.Hour), wantErr: ErrValidation},
		{name: "same end time rejected", exam: published(), role: constants.RoleAdmin, userID: 1, endTime: oldEnd, wantErr: ErrValidation},
		{
			name:    "new end time must be after now",
			exam:    examWith(constants.ExamPublished, ptrTime(now.Add(-2*time.Hour))),
			role:    constants.RoleAdmin,
			userID:  1,
			endTime: now.Add(-time.Hour),
			wantErr: ErrValidation,
		},
		{name: "exam not found", exam: nil, role: constants.RoleAdmin, userID: 1, endTime: newEnd, wantErr: ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			examRepo := &fakeExamRepo{exam: tt.exam, affected: 3}
			svc := NewExamService(examRepo, fakeQuestionRepo{}, slog.Default())

			resp, err := svc.Extend(context.Background(), tt.role, tt.userID, 7, dto.ExamExtendRequest{EndTime: tt.endTime})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Extend() error = %v, want %v", err, tt.wantErr)
				}
				if examRepo.extendCalls != 0 {
					t.Fatalf("ExtendExamEndTime called %d times, want 0", examRepo.extendCalls)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extend() unexpected error = %v", err)
			}
			if !resp.OldEndTime.Equal(oldEnd) {
				t.Fatalf("OldEndTime = %v, want %v", resp.OldEndTime, oldEnd)
			}
			if !resp.NewEndTime.Equal(tt.endTime) {
				t.Fatalf("NewEndTime = %v, want %v", resp.NewEndTime, tt.endTime)
			}
			wantMinutes := round2(tt.endTime.Sub(oldEnd).Minutes())
			if resp.ExtendMinutes != wantMinutes {
				t.Fatalf("ExtendMinutes = %v, want %v", resp.ExtendMinutes, wantMinutes)
			}
			if resp.AffectedAttempts != 3 {
				t.Fatalf("AffectedAttempts = %d, want 3", resp.AffectedAttempts)
			}
			if examRepo.extendCalls != 1 {
				t.Fatalf("ExtendExamEndTime called %d times, want exactly 1 atomic save", examRepo.extendCalls)
			}
			if examRepo.updateCalls != 0 {
				t.Fatalf("UpdateExam called %d times, want 0 (extend must be a single atomic save)", examRepo.updateCalls)
			}
			if !examRepo.extendEnd.Equal(tt.endTime) {
				t.Fatalf("extend end = %v, want %v", examRepo.extendEnd, tt.endTime)
			}
			if wantDelta := tt.endTime.Sub(oldEnd); examRepo.extendDelta != wantDelta {
				t.Fatalf("extend delta = %v, want %v", examRepo.extendDelta, wantDelta)
			}
			if examRepo.exam.EndTime == nil || !examRepo.exam.EndTime.Equal(tt.endTime) {
				t.Fatalf("exam end time not updated: %v", examRepo.exam.EndTime)
			}
		})
	}
}

// TestExamServiceExtendRollback verifies that a persistence failure during
// extend leaves both the exam end time and the attempt deadlines untouched:
// the whole operation is saved atomically or not at all.
func TestExamServiceExtendRollback(t *testing.T) {
	now := time.Now()
	oldEnd := now.Add(2 * time.Hour)
	exam := &model.Exam{ID: 7, Title: "期中考试", Status: constants.ExamPublished, CreatedBy: 42, EndTime: &oldEnd}
	persistErr := errors.New("shift in-progress deadlines: connection lost")
	examRepo := &fakeExamRepo{exam: exam, extendErr: persistErr}
	svc := NewExamService(examRepo, fakeQuestionRepo{}, slog.Default())

	newEnd := now.Add(3 * time.Hour)
	resp, err := svc.Extend(context.Background(), constants.RoleAdmin, 1, 7, dto.ExamExtendRequest{EndTime: newEnd})
	if err == nil {
		t.Fatal("Extend() expected error, got nil")
	}
	if !errors.Is(err, persistErr) {
		t.Fatalf("Extend() error = %v, want wrapped %v", err, persistErr)
	}
	if resp != nil {
		t.Fatalf("Extend() response = %+v, want nil", resp)
	}
	if examRepo.extendCalls != 1 {
		t.Fatalf("ExtendExamEndTime called %d times, want 1", examRepo.extendCalls)
	}
	if examRepo.updateCalls != 0 {
		t.Fatalf("UpdateExam called %d times, want 0 (no partial save before the failure)", examRepo.updateCalls)
	}
	if examRepo.exam.EndTime == nil || !examRepo.exam.EndTime.Equal(oldEnd) {
		t.Fatalf("exam end time changed after failed extend: %v, want original %v", examRepo.exam.EndTime, oldEnd)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
