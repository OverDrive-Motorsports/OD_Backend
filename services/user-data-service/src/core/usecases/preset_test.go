/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset_test.go - Unit tests for the preset use case.
##
*/

package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"overdrive/services/user-data-service/src/core/domain"
)

// fakePresetRepository is an in-memory store keyed by owner, mirroring the real repository's
// ownership rule: a preset owned by another user behaves like a missing one.
type fakePresetRepository struct {
	owners    map[string]string // presetID -> userID
	presets   map[string]domain.Preset
	countErr  error
	err       error
	created   *domain.NewPreset
	createdBy string
}

func newFakeRepo() *fakePresetRepository {
	return &fakePresetRepository{owners: map[string]string{}, presets: map[string]domain.Preset{}}
}

func (f *fakePresetRepository) add(userID string, preset domain.Preset) {
	f.owners[preset.ID] = userID
	f.presets[preset.ID] = preset
}

func (f *fakePresetRepository) ListByUser(_ context.Context, userID string) ([]domain.Preset, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.Preset
	for id, owner := range f.owners {
		if owner == userID {
			out = append(out, f.presets[id])
		}
	}
	return out, nil
}

func (f *fakePresetRepository) GetByID(_ context.Context, userID string, presetID string) (*domain.Preset, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.owners[presetID] != userID {
		return nil, nil
	}
	preset := f.presets[presetID]
	return &preset, nil
}

func (f *fakePresetRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	presets, _ := f.ListByUser(ctx, userID)
	return len(presets), nil
}

func (f *fakePresetRepository) Create(_ context.Context, userID string, input domain.NewPreset) (*domain.Preset, error) {
	f.created = &input
	f.createdBy = userID
	return &domain.Preset{ID: "new", Name: input.Name, IsDefault: input.IsDefault, Data: input.Data}, f.err
}

func (f *fakePresetRepository) Update(ctx context.Context, userID string, presetID string, changes domain.PresetChanges) (*domain.Preset, error) {
	existing, err := f.GetByID(ctx, userID, presetID)
	if err != nil || existing == nil {
		return nil, err
	}
	if changes.Name != nil {
		existing.Name = *changes.Name
	}
	return existing, nil
}

func (f *fakePresetRepository) Delete(_ context.Context, userID string, presetID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.owners[presetID] != userID {
		return false, nil
	}
	delete(f.owners, presetID)
	return true, nil
}

var layout = json.RawMessage(`{"widgets":[]}`)

// TestPresetUseCase_ListAndGet proves List never returns nil and only the caller's presets,
// and Get answers ErrPresetNotFound both for a missing preset and for another user's preset.
func TestPresetUseCase_ListAndGet(t *testing.T) {
	repo := newFakeRepo()
	repo.add("alice", domain.Preset{ID: "p1", Name: "Alice layout"})
	repo.add("bob", domain.Preset{ID: "p2", Name: "Bob layout"})
	uc := NewPresetUseCase(repo)

	got, err := uc.List(context.Background(), "carol")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected an empty non-nil list, got %#v err=%v", got, err)
	}
	got, _ = uc.List(context.Background(), "alice")
	if len(got) != 1 || got[0].ID != "p1" {
		t.Fatalf("expected only alice's preset, got %+v", got)
	}

	if preset, err := uc.Get(context.Background(), "alice", "p1"); err != nil || preset.ID != "p1" {
		t.Fatalf("own preset: got %+v err=%v", preset, err)
	}
	for _, id := range []string{"missing", "p2"} {
		if _, err := uc.Get(context.Background(), "alice", id); !errors.Is(err, domain.ErrPresetNotFound) {
			t.Fatalf("%s: expected ErrPresetNotFound, got %v", id, err)
		}
	}
}

// TestPresetUseCase_Create proves validation runs before any storage call, the first preset of
// a user is forced to default, later ones keep the requested flag, the limit answers
// ErrPresetLimitReached without creating anything, and the user ID is forwarded unchanged.
func TestPresetUseCase_Create(t *testing.T) {
	repo := newFakeRepo()
	if _, err := NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "", Data: layout}); !errors.Is(err, domain.ErrInvalidPreset) || repo.created != nil {
		t.Fatalf("invalid payload: expected ErrInvalidPreset without storage, got %v", err)
	}

	preset, err := NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "First", Data: layout})
	if err != nil || !preset.IsDefault || repo.createdBy != "alice" {
		t.Fatalf("first preset must become default: %+v err=%v by=%q", preset, err, repo.createdBy)
	}

	repo = newFakeRepo()
	repo.add("alice", domain.Preset{ID: "p1"})
	preset, _ = NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "Second", Data: layout})
	if preset.IsDefault {
		t.Fatal("a later preset must keep isDefault=false when not requested")
	}
	preset, _ = NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "Third", IsDefault: true, Data: layout})
	if !preset.IsDefault {
		t.Fatal("a later preset must honour isDefault=true")
	}

	repo = newFakeRepo()
	for index := 0; index < domain.MaxPresetsPerUser; index++ {
		repo.add("alice", domain.Preset{ID: string(rune('a' + index))})
	}
	if _, err := NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "One too many", Data: layout}); !errors.Is(err, domain.ErrPresetLimitReached) || repo.created != nil {
		t.Fatalf("expected ErrPresetLimitReached without storage, got %v", err)
	}

	boom := errors.New("db down")
	repo = newFakeRepo()
	repo.countErr = boom
	if _, err := NewPresetUseCase(repo).Create(context.Background(), "alice", domain.NewPreset{Name: "X", Data: layout}); !errors.Is(err, boom) {
		t.Fatalf("expected the count error, got %v", err)
	}
}

// TestPresetUseCase_UpdateAndDelete proves an empty update is rejected before storage, another
// user's preset is ErrPresetNotFound on update and delete, and the owner can rename and delete.
func TestPresetUseCase_UpdateAndDelete(t *testing.T) {
	repo := newFakeRepo()
	repo.add("alice", domain.Preset{ID: "p1", Name: "Old"})
	uc := NewPresetUseCase(repo)
	name := "New"

	if _, err := uc.Update(context.Background(), "alice", "p1", domain.PresetChanges{}); !errors.Is(err, domain.ErrInvalidPreset) {
		t.Fatalf("empty update: expected ErrInvalidPreset, got %v", err)
	}
	if _, err := uc.Update(context.Background(), "bob", "p1", domain.PresetChanges{Name: &name}); !errors.Is(err, domain.ErrPresetNotFound) {
		t.Fatalf("cross-user update: expected ErrPresetNotFound, got %v", err)
	}
	if preset, err := uc.Update(context.Background(), "alice", "p1", domain.PresetChanges{Name: &name}); err != nil || preset.Name != "New" {
		t.Fatalf("owner update: got %+v err=%v", preset, err)
	}

	if err := uc.Delete(context.Background(), "bob", "p1"); !errors.Is(err, domain.ErrPresetNotFound) {
		t.Fatalf("cross-user delete: expected ErrPresetNotFound, got %v", err)
	}
	if err := uc.Delete(context.Background(), "alice", "p1"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if err := uc.Delete(context.Background(), "alice", "p1"); !errors.Is(err, domain.ErrPresetNotFound) {
		t.Fatalf("second delete: expected ErrPresetNotFound, got %v", err)
	}

	boom := errors.New("db down")
	repo.err = boom
	if err := uc.Delete(context.Background(), "alice", "p1"); !errors.Is(err, boom) {
		t.Fatalf("expected the storage error, got %v", err)
	}
}
