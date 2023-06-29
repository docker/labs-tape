package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/otiai10/copy"
)

type Loader interface {
	Load() error
	Paths() []string
	RelPaths() (string, []string)
	ContainsRelPath(string) bool
	Cleanup() error
}

type RecursiveManifestDirectoryLoader struct {
	fromPath string
	tempDir  string
	paths    []string
	relPaths map[string]string
}

func NewRecursiveManifestDirectoryLoader(path string) Loader {
	return &RecursiveManifestDirectoryLoader{fromPath: path}
}

func (l *RecursiveManifestDirectoryLoader) Load() error {
	tempDir, err := mkdirTemp()
	if err != nil {
		return err
	}
	l.tempDir = tempDir

	if filepath.IsAbs(l.fromPath) {
		relPath, err := filepath.Rel("", l.fromPath)
		if err != nil {
			return err
		}
		l.fromPath = relPath
	}

	files, err := getFiles(l.fromPath)
	if err != nil {
		return err
	}
	l.relPaths = make(map[string]string, len(files))
	for _, f := range files {
		relPath, err := filepath.Rel(l.fromPath, f)
		if err != nil {
			return err
		}
		l.relPaths[relPath] = f
	}

	// if useKustomize(l.fromPath) || useKustomize(files...) {
	// 	// it's possible that kustomization references files out of the tempDir
	// 	// this will do `kustomize build` on l.fromPath and write the result into l.tempDir
	// 	// TODO: this assumes all files are to be handled by kustomize,
	// 	// it would be sensible to enable using different handlers for different subdirs
	// 	return fmt.Errorf("TODO: implement kustomize build")
	// }

	copyOptions := copy.Options{
		Skip: func(fi fs.FileInfo, src, _ string) (bool, error) {
			if fi.IsDir() {
				return false, nil
			}
			return ignoreFile(src), nil
		},
	}

	if err := copy.Copy(l.fromPath, l.tempDir, copyOptions); err != nil {
		return err
	}

	files, err = getFiles(l.tempDir)
	if err != nil {
		return err
	}
	l.paths = files
	return nil
}

func (l *RecursiveManifestDirectoryLoader) Paths() []string {
	return l.paths
}

func (l *RecursiveManifestDirectoryLoader) RelPaths() (string, []string) {
	relPaths := make([]string, 0, len(l.relPaths))
	for p := range l.relPaths {
		relPaths = append(relPaths, p)
	}
	return l.tempDir, relPaths
}

func (l *RecursiveManifestDirectoryLoader) ContainsRelPath(p string) bool {
	_, ok := l.relPaths[p]
	return ok
}

func (l *RecursiveManifestDirectoryLoader) Cleanup() error {
	if l.tempDir == "" {
		return nil
	}
	return os.RemoveAll(l.tempDir)
}

func mkdirTemp() (string, error) {
	tempDir, err := os.MkdirTemp("", "bpt-manifest-loader-*")
	if err != nil {
		return "", err
	}
	tempDir, err = filepath.EvalSymlinks(tempDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: %w", err)
	}
	return tempDir, nil
}

// based on ExpandPathsToFileVisitors (https://github.com/kubernetes/cli-runtime/blob/022795328092ecd88b713a2bab868e3994eb0b87/pkg/resource/visitor.go#L478)
func getFiles(path string) ([]string, error) {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("the path %q does not exist: %w", path, err)
	}
	if err != nil {
		return nil, fmt.Errorf("the path %q cannot be accessed: %v", path, err)
	}

	if !fi.IsDir() {
		return []string{path}, nil
	}

	files := []string{}
	doWalk := func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if e.IsDir() || ignoreFile(p) {
			return nil
		}

		files = append(files, p)
		return nil
	}

	if err := filepath.WalkDir(path, doWalk); err != nil {
		return nil, err
	}
	return files, nil
}

func ignoreFile(path string) bool {
	switch filepath.Ext(path) {
	case ".json", ".yaml", ".yml":
		return false
	default:
		return true
	}
}

func useKustomize(paths ...string) bool {
	for i := range paths {
		switch filepath.Base(paths[i]) {
		case "kustomization.yaml", "kustomization.yml", "Kustomization":
			return true
		}
	}
	return false
}
