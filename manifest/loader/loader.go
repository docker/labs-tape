package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/otiai10/copy"
)

type Loader interface {
	Load() error
	Paths() []string
	RelPaths() (string, []string)
	ContainsRelPath(string) bool
	Cleanup() error
	MostRecentlyModified() (string, time.Time)
}

type RecursiveManifestDirectoryLoader struct {
	fromPath string
	tempDir  string
	files    []fileWithModTime
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
		relPath, err := filepath.Rel(l.fromPath, f.path)
		if err != nil {
			return err
		}
		l.relPaths[relPath] = f.path
	}

	// if useKustomize(l.fromPath) || useKustomize(files...) {
	// 	// it's possible that kustomization references files out of the tempDir
	// 	// this will do `kustomize build` on l.fromPath and write the result into l.tempDir
	// 	// TODO: this assumes all files are to be handled by kustomize,
	// 	// it would be sensible to enable using different handlers for different subdirs
	// 	return fmt.Errorf("TODO: implement kustomize build")
	// }

	copyOptions := copy.Options{
		// TODO: documentation for PreserveTimes says there is limited accuracy on Linux,
		// namely it's only to up to 1ms, it's possible that it need a fix, e.g. by rouding
		// stored timestamps to milliseconds on all platforms;
		// the answer really depends on how accurately mtime can be preserved between platforms,
		// it makes sense to consider what git, tar and rsync do in that regard
		PreserveTimes: true,
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
	l.files = files

	return nil
}

func (l *RecursiveManifestDirectoryLoader) MostRecentlyModified() (string, time.Time) {
	return l.files[0].path, l.files[0].time
}

func (l *RecursiveManifestDirectoryLoader) Paths() []string {
	paths := make([]string, 0, len(l.files))
	for _, file := range l.files {
		paths = append(paths, file.path)
	}
	return paths
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

type fileWithModTime struct {
	// collect timestamps to use for setting the artefact creation time
	path string
	time time.Time
}

// based on ExpandPathsToFileVisitors (https://github.com/kubernetes/cli-runtime/blob/022795328092ecd88b713a2bab868e3994eb0b87/pkg/resource/visitor.go#L478)
func getFiles(path string) ([]fileWithModTime, error) {
	files := []fileWithModTime{}

	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("the path %q does not exist: %w", path, err)
	}
	if err != nil {
		return nil, fmt.Errorf("the path %q cannot be accessed: %v", path, err)
	}

	if !fi.IsDir() {
		files = append(files, fileWithModTime{path: path, time: fi.ModTime()})
		return files, nil
	}

	doWalk := func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if e.IsDir() || ignoreFile(p) {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}

		files = append(files, fileWithModTime{path: p, time: info.ModTime()})
		return nil
	}

	if err := filepath.WalkDir(path, doWalk); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in %q", path)
	}
	slices.SortFunc(files, func(a, b fileWithModTime) int {
		if timewise := a.time.Compare(b.time); timewise != 0 {
			return timewise
		}
		return strings.Compare(a.path, b.path)
	})
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
