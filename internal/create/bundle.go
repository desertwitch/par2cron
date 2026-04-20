package create

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
)

func (prog *Service) packAsBundle(ctx context.Context, job *Job, mf *schema.Manifest) error {
	logger := prog.creationLogger(ctx, job, job.par2Path)

	files, err := prog.findBundleableFiles(job)
	if err != nil {
		return fmt.Errorf("failed to find created files: %w", err)
	}

	p, err := prog.par2er.ParseFile(prog.fsys, job.par2Path, true)
	if err != nil {
		return fmt.Errorf("failed to parse index par2: %w", err)
	}

	logger.Debug("Parsed PAR2 index file", "sets", len(p.Sets))
	if len(p.Sets) != 1 || p.Sets[0].MainPacket == nil {
		return errors.New("failed to parse index par2: malformed file")
	}

	recoverySetID := p.Sets[0].MainPacket.SetID
	logger.Debug("Parsed PAR2 main packet", "setID", recoverySetID)

	baseName := strings.TrimSuffix(job.par2Name, schema.Par2Extension)
	bundleName := baseName + schema.BundleExtension + schema.Par2Extension
	bundlePath := filepath.Join(job.workingDir, bundleName)

	manifestData, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifest := bundle.ManifestInput{
		Name:  job.manifestName,
		Bytes: manifestData,
	}

	if err := prog.bundler.Pack(prog.fsys, bundlePath, recoverySetID, manifest, files); err != nil {
		return fmt.Errorf("failed to pack bundle: %w", err)
	}

	for _, file := range files {
		if err := prog.fsys.Remove(file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.creationLogger(ctx, job, file.Path)
			logger.Warn("Failed to cleanup a file after bundling (needs manual deletion)", "error", err)
		}
	}

	for _, path := range []string{job.manifestPath, job.lockPath} {
		if err := prog.fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.creationLogger(ctx, job, path)
			logger.Warn("Failed to cleanup a file after bundling (needs manual deletion)", "error", err)
		}
	}

	job.par2Name = bundleName
	job.par2Path = bundlePath
	job.manifestName = bundleName
	job.manifestPath = bundlePath
	job.lockPath = bundlePath

	return nil
}

func (prog *Service) findBundleableFiles(job *Job) ([]bundle.FileInput, error) {
	entries, err := afero.ReadDir(prog.fsys, job.workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	inputs := []bundle.FileInput{}

	baseName := strings.TrimSuffix(job.par2Name, schema.Par2Extension) + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, baseName) {
			continue
		}
		if !util.IsPar2Index(name) && !util.IsPar2Volume(name) {
			continue
		}
		if strings.Contains(name, schema.BundleExtension) {
			continue
		}

		inputs = append(inputs, bundle.FileInput{
			Name: name,
			Path: filepath.Join(job.workingDir, name),
		})
	}

	return inputs, nil
}
