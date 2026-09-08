package workshop

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SpaiR/imgui-go"
	"github.com/skratchdot/open-golang/open"
	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/app/window"
	"sdmm/internal/env"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/ship"
)

// RemovalConfirmation keeps the irreversible action separate from navigation.
// Plans are read-only until Confirm succeeds; Cancel simply closes the dialog.
type RemovalConfirmation struct {
	Kind     string
	Entry    string
	Plan     *ship.RemovalPlan
	Error    string
	Warning  string
	Check    func() error
	Removed  func(*ship.RemovalPlan, string)
	prepared <-chan removalResult
	applied  <-chan removalResult
}

type removalResult struct {
	plan   *ship.RemovalPlan
	backup string
	err    error
}

// Prepare keeps project scans off the UI thread. Callers capture the current
// catalog before opening the modal; only the UI thread updates editor state.
func (r *RemovalConfirmation) Prepare(build func() (*ship.RemovalPlan, error)) {
	result := make(chan removalResult, 1)
	r.prepared = result
	go func() {
		plan, err := build()
		result <- removalResult{plan: plan, err: err}
	}()
}

func (r *RemovalConfirmation) Poll() {
	select {
	case result := <-r.prepared:
		r.prepared = nil
		r.Plan = result.plan
		if result.err != nil {
			r.Error = result.err.Error()
		}
	default:
	}
}

func (r *RemovalConfirmation) Pending() bool { return r.prepared != nil || r.applied != nil }

func (r *RemovalConfirmation) Name() string       { return "Remove " + r.Kind + "?" }
func (*RemovalConfirmation) HasCloseButton() bool { return false }

func (r *RemovalConfirmation) check() error {
	if r.Plan == nil {
		return fmt.Errorf("review the removal first")
	}
	if r.Check != nil {
		if err := r.Check(); err != nil {
			return err
		}
	}
	return nil
}

func applyRemoval(plan *ship.RemovalPlan) (string, error) {
	profile, err := env.ProfileDir()
	if err != nil {
		return "", err
	}
	return plan.Apply(filepath.Join(profile, "removed-project-files"))
}

func (r *RemovalConfirmation) Confirm() error {
	if err := r.check(); err != nil {
		return err
	}
	backup, err := applyRemoval(r.Plan)
	if err != nil {
		return err
	}
	if r.Removed != nil {
		r.Removed(r.Plan, backup)
	}
	return nil
}

func (r *RemovalConfirmation) Process() {
	r.Poll()
	select {
	case result := <-r.applied:
		r.applied = nil
		if result.err != nil {
			r.Error = result.err.Error()
		} else {
			if r.Removed != nil {
				r.Removed(r.Plan, result.backup)
			}
			imgui.CloseCurrentPopup()
			return
		}
	default:
	}
	PushStyle()
	defer PopStyle()
	width := min(float32(560)*window.PointSize(), imgui.MainViewport().Size().X-70*window.PointSize())
	imgui.Dummy(imgui.Vec2{X: width})
	imgui.PushTextWrapPosV(width)
	defer imgui.PopTextWrapPos()
	Title(r.Entry)
	if r.prepared != nil {
		Muted("Checking registrations and shared files...")
	}
	if r.applied != nil {
		Muted("Saving a recovery copy and removing files...")
	}
	if r.Plan != nil {
		deleted, updated, maps := 0, 0, 0
		for _, change := range r.Plan.Changes {
			if change.Delete {
				deleted++
				if strings.EqualFold(filepath.Ext(change.Path), ".dmm") {
					maps++
				}
			} else {
				updated++
			}
		}
		if len(r.Plan.Changes) == 0 {
			imgui.TextWrapped("Discard this unsaved entry? No project files will be removed.")
		} else {
			imgui.TextWrapped(fmt.Sprintf("Remove this %s and its registrations from the project?", r.Kind))
			imgui.TextWrapped(fmt.Sprintf("Maps: %d  |  Files removed: %d  |  Files updated: %d", maps, deleted, updated))
			Muted("A recovery ZIP will be saved before removal. This cannot be undone with Ctrl+Z.")
		}
		if r.Warning != "" {
			imgui.TextWrapped(r.Warning)
		}
		if imgui.CollapsingHeader("Files affected") {
			imgui.BeginChildV("removal-files", imgui.Vec2{X: width, Y: 180 * window.PointSize()}, true, 0)
			for _, change := range r.Plan.Changes {
				action := "Update "
				if change.Delete {
					action = "Remove "
				}
				rel, _ := filepath.Rel(r.Plan.Root, change.Path)
				imgui.TextWrapped(action + filepath.ToSlash(rel))
			}
			imgui.EndChild()
		}
		if len(r.Plan.Kept) > 0 && imgui.CollapsingHeader("Shared files and types kept") {
			imgui.BeginChildV("removal-kept", imgui.Vec2{X: width, Y: 130 * window.PointSize()}, true, 0)
			for _, path := range r.Plan.Kept {
				if rel, err := filepath.Rel(r.Plan.Root, path); err == nil {
					path = filepath.ToSlash(rel)
				}
				imgui.TextWrapped(path)
			}
			imgui.EndChild()
		}
	}
	if r.Error != "" {
		imgui.TextWrapped(r.Error)
	}
	Gap()
	imgui.BeginDisabledV(r.Plan == nil || r.Pending())
	if DangerButton("Remove " + r.Kind) {
		if err := r.check(); err != nil {
			r.Error = err.Error()
		} else {
			r.Error = ""
			result := make(chan removalResult, 1)
			r.applied = result
			plan := r.Plan
			go func() {
				backup, err := applyRemoval(plan)
				result <- removalResult{backup: backup, err: err}
			}()
		}
	}
	imgui.EndDisabled()
	imgui.BeginDisabledV(r.applied != nil)
	if Button("Cancel", false) {
		imgui.CloseCurrentPopup()
	}
	imgui.EndDisabled()
}

func DangerButton(label string) bool {
	imgui.PushStyleColor(imgui.StyleColorButton, style.RGB(0x482c36))
	imgui.PushStyleColor(imgui.StyleColorButtonHovered, style.RGB(0x69404c))
	imgui.PushStyleColor(imgui.StyleColorButtonActive, style.RGB(0x814d5b))
	imgui.PushStyleColor(imgui.StyleColorText, style.Text)
	clicked := Button(label, false)
	imgui.PopStyleColorV(4)
	return clicked
}

func OpenRemoval(r *RemovalConfirmation) { dialog.Open(r) }
func OpenRemovalRecovery(path string) {
	if path != "" {
		_ = open.Run(filepath.Dir(path))
	}
}
