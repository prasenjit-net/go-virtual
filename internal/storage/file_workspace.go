package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/prasenjit/go-virtual/internal/models"
)

func workspaceBackupPath(base string) string { return base + ".designer-backup" }

// recoverWorkspaceSwap completes or rolls back an interrupted directory swap.
// If the live directory is absent, the old snapshot wins; if both are present,
// the staged directory had already become live and is kept.
func recoverWorkspaceSwap(base string) error {
	backup := workspaceBackupPath(base)
	if _, err := os.Stat(backup); err == nil {
		if _, liveErr := os.Stat(base); os.IsNotExist(liveErr) {
			if err := os.Rename(backup, base); err != nil {
				return fmt.Errorf("restore interrupted workspace save: %w", err)
			}
		} else if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("clean completed workspace save: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	parent, name := filepath.Dir(base), filepath.Base(base)
	stages, err := filepath.Glob(filepath.Join(parent, name+".designer-stage-*"))
	if err != nil {
		return err
	}
	for _, stage := range stages {
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
	}
	return nil
}

func copyWorkspaceDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace storage contains unsupported symlink %q", rel)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func (f *FileStorage) ApplySpecWorkspace(workspace *models.SpecWorkspace) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	base, err := filepath.Abs(f.basePath)
	if err != nil {
		return err
	}
	if err := recoverWorkspaceSwap(base); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(base), filepath.Base(base)+".designer-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := copyWorkspaceDirectory(base, stage); err != nil {
		return fmt.Errorf("stage workspace files: %w", err)
	}
	staged, err := NewFileStorage(stage)
	if err != nil {
		return fmt.Errorf("load staged workspace: %w", err)
	}
	if err := applyWorkspaceRecords(staged, workspace); err != nil {
		_ = staged.Close()
		return fmt.Errorf("apply staged workspace: %w", err)
	}
	backup := workspaceBackupPath(base)
	if err := os.Rename(base, backup); err != nil {
		_ = staged.Close()
		return fmt.Errorf("prepare workspace swap: %w", err)
	}
	if err := os.Rename(stage, base); err != nil {
		if restoreErr := os.Rename(backup, base); restoreErr != nil {
			_ = staged.Close()
			return fmt.Errorf("publish workspace: %v; restore prior workspace: %w", err, restoreErr)
		}
		_ = staged.Close()
		return fmt.Errorf("publish workspace: %w", err)
	}

	// Keep the live MemoryStorage pointer stable for readers. Its map set is
	// swapped under its own lock after the complete directory is in place.
	current := f.memory
	current.mu.Lock()
	current.specs = staged.memory.specs
	current.operations = staged.memory.operations
	current.responseConfigs = staged.memory.responseConfigs
	current.scriptBindings = staged.memory.scriptBindings
	current.collectionMappings = staged.memory.collectionMappings
	current.validationRules = staged.memory.validationRules
	current.mu.Unlock()
	_ = staged.Close()
	// Backup cleanup is recoverable on startup and cannot turn an already
	// committed snapshot into a failed Save response.
	_ = os.RemoveAll(backup)
	return nil
}
