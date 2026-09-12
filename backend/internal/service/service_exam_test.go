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
	records     []model.ExamExtension
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

// ExtendExamEndTime applies the end-time change, the deadline shift and the
// audit record as one operation. When extendErr is set it fails without
// touching stored state, mirroring a rolled-back transaction.
func (f *fakeExamRepo) ExtendExamEndTime(_ context.Context, examID uint, newEnd time.Time, delta time.Duration, record *model.ExamExtension) (int64, error) {
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
	record.ExamID = examID
	record.AffectedAttempts = int(f.affected)
	record.ID = uint(len(f.records) + 1)
	record.CreatedAt = time.Now()
	f.records = append(f.records, *record)
	return f.affected, nil
}

// ListExamExtensions returns stored records newest first, like the real repo.
func (f *fakeExamRepo) ListExamExtensions(_ context.Context, _ uint) ([]model.ExamExtension, error) {
	result := make([]model.ExamExtension, 0, len(f.records))
	for i := len(f.records) - 1; i >= 0; i-- {
		result = append(result, f.records[i])
	}
	return result, nil
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

// fakeUserRepo is an in-memory UserRepo for operator name snapshots.
type fakeUserRepo struct {
	users map[uint]model.User
}

func (fakeUserRepo) CreateUser(_ context.Context, _ *model.User) error { return nil }
func (fakeUserRepo) FindUserByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (f fakeUserRepo) FindUserByID(_ context.Context, id uint) (*model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}
func (fakeUserRepo) ListUsers(_ context.Context, _, _ string, _, _ int) ([]model.User, int64, error) {
	return nil, 0, nil
}
func (fakeUserRepo) UpdateUserStatus(_ context.Context, _ uint, _ string) error { return nil }

var extendTestUsers = map[uint]model.User{
	42: {ID: 42, Username: "teacher1", Name: "张老师"},
	1:  {ID: 1, Username: "admin", Name: "管理员"},
}

func newExamService(repo ExamRepo) *ExamService {
	return NewExamService(repo, fakeQuestionRepo{}, fakeUserRepo{users: extendTestUsers}, slog.Default())
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
			svc := newExamService(examRepo)

			resp, err := svc.Extend(context.Background(), tt.role, tt.userID, 7, dto.ExamExtendRequest{EndTime: tt.endTime})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Extend() error = %v, want %v", err, tt.wantErr)
				}
				if examRepo.extendCalls != 0 {
					t.Fatalf("ExtendExamEndTime called %d times, want 0", examRepo.extendCalls)
				}
				if len(examRepo.records) != 0 {
					t.Fatalf("extension records = %d, want 0 after rejected extend", len(examRepo.records))
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
			if len(examRepo.records) != 1 {
				t.Fatalf("extension records = %d, want 1", len(examRepo.records))
			}
			rec := examRepo.records[0]
			if rec.ExamID != 7 {
				t.Fatalf("record ExamID = %d, want 7", rec.ExamID)
			}
			if rec.OperatorID != tt.userID {
				t.Fatalf("record OperatorID = %d, want %d", rec.OperatorID, tt.userID)
			}
			if wantName := extendTestUsers[tt.userID].Name; rec.OperatorName != wantName {
				t.Fatalf("record OperatorName = %q, want %q", rec.OperatorName, wantName)
			}
			if !rec.OldEndTime.Equal(oldEnd) || !rec.NewEndTime.Equal(tt.endTime) {
				t.Fatalf("record times = %v -> %v, want %v -> %v", rec.OldEndTime, rec.NewEndTime, oldEnd, tt.endTime)
			}
			if rec.ExtendMinutes != wantMinutes {
				t.Fatalf("record ExtendMinutes = %v, want %v", rec.ExtendMinutes, wantMinutes)
			}
			if rec.AffectedAttempts != 3 {
				t.Fatalf("record AffectedAttempts = %d, want 3", rec.AffectedAttempts)
			}
		})
	}
}

// TestExamServiceExtendRollback verifies that a persistence failure during
// extend leaves the exam end time, the attempt deadlines and the record log
// untouched: the whole operation is saved atomically or not at all.
func TestExamServiceExtendRollback(t *testing.T) {
	now := time.Now()
	oldEnd := now.Add(2 * time.Hour)
	exam := &model.Exam{ID: 7, Title: "期中考试", Status: constants.ExamPublished, CreatedBy: 42, EndTime: &oldEnd}
	persistErr := errors.New("shift in-progress deadlines: connection lost")
	examRepo := &fakeExamRepo{exam: exam, extendErr: persistErr}
	svc := newExamService(examRepo)

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
	if len(examRepo.records) != 0 {
		t.Fatalf("extension records = %d, want 0 after rolled-back extend", len(examRepo.records))
	}
}

func TestExamServiceListExtensions(t *testing.T) {
	now := time.Now()
	oldEnd := now.Add(2 * time.Hour)
	ctx := context.Background()

	// extendTwice returns a repo whose exam 7 (created by 42) was extended
	// twice: first by teacher 42 to now+3h, then by admin 1 to now+4h.
	extendTwice := func(t *testing.T) *fakeExamRepo {
		t.Helper()
		end := oldEnd
		examRepo := &fakeExamRepo{
			exam:     &model.Exam{ID: 7, Title: "期中考试", Status: constants.ExamPublished, CreatedBy: 42, EndTime: &end},
			affected: 2,
		}
		svc := newExamService(examRepo)
		if _, err := svc.Extend(ctx, constants.RoleTeacher, 42, 7, dto.ExamExtendRequest{EndTime: now.Add(3 * time.Hour)}); err != nil {
			t.Fatalf("first extend: %v", err)
		}
		if _, err := svc.Extend(ctx, constants.RoleAdmin, 1, 7, dto.ExamExtendRequest{EndTime: now.Add(4 * time.Hour)}); err != nil {
			t.Fatalf("second extend: %v", err)
		}
		return examRepo
	}

	t.Run("teacher lists own exam records newest first", func(t *testing.T) {
		examRepo := extendTwice(t)
		svc := newExamService(examRepo)
		records, err := svc.ListExtensions(ctx, constants.RoleTeacher, 42, 7)
		if err != nil {
			t.Fatalf("ListExtensions() unexpected error = %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("records = %d, want 2", len(records))
		}
		if !records[0].NewEndTime.Equal(now.Add(4*time.Hour)) || !records[1].NewEndTime.Equal(now.Add(3*time.Hour)) {
			t.Fatalf("records not newest first: %v then %v", records[0].NewEndTime, records[1].NewEndTime)
		}
		if records[0].OperatorID != 1 || records[0].OperatorName != "管理员" {
			t.Fatalf("newest record operator = %d/%q, want 1/管理员", records[0].OperatorID, records[0].OperatorName)
		}
		if records[1].OperatorID != 42 || records[1].OperatorName != "张老师" {
			t.Fatalf("oldest record operator = %d/%q, want 42/张老师", records[1].OperatorID, records[1].OperatorName)
		}
		if records[0].AffectedAttempts != 2 || records[0].ExtendMinutes != 60 {
			t.Fatalf("newest record = %d attempts/%v minutes, want 2/60", records[0].AffectedAttempts, records[0].ExtendMinutes)
		}
	})

	t.Run("admin lists any exam records", func(t *testing.T) {
		examRepo := extendTwice(t)
		svc := newExamService(examRepo)
		records, err := svc.ListExtensions(ctx, constants.RoleAdmin, 1, 7)
		if err != nil {
			t.Fatalf("ListExtensions() unexpected error = %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("records = %d, want 2", len(records))
		}
	})

	t.Run("teacher cannot list others exam records", func(t *testing.T) {
		examRepo := extendTwice(t)
		svc := newExamService(examRepo)
		if _, err := svc.ListExtensions(ctx, constants.RoleTeacher, 99, 7); !errors.Is(err, ErrForbidden) {
			t.Fatalf("ListExtensions() error = %v, want %v", err, ErrForbidden)
		}
	})

	t.Run("no records returns empty list", func(t *testing.T) {
		end := oldEnd
		examRepo := &fakeExamRepo{exam: &model.Exam{ID: 7, Status: constants.ExamPublished, CreatedBy: 42, EndTime: &end}}
		svc := newExamService(examRepo)
		records, err := svc.ListExtensions(ctx, constants.RoleTeacher, 42, 7)
		if err != nil {
			t.Fatalf("ListExtensions() unexpected error = %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("records = %d, want 0", len(records))
		}
	})

	t.Run("exam not found", func(t *testing.T) {
		examRepo := &fakeExamRepo{}
		svc := newExamService(examRepo)
		if _, err := svc.ListExtensions(ctx, constants.RoleAdmin, 1, 7); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ListExtensions() error = %v, want %v", err, ErrNotFound)
		}
	})
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
