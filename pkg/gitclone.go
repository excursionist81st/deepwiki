package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

type GitCloner struct {
	RepoDir string
}

type FileFilter struct {
	ExcludeDirs  []string
	ExcludeExts  []string
	ExcludeFiles []string
	IncludeExts  []string
	IncludeFiles []string
	MaxFileSize  int64
}

func NewFileFilter() *FileFilter {
	return &FileFilter{
		ExcludeDirs: []string{
			".git", "vendor", "node_modules", "__pycache__",
			".venv", "venv", "dist", "build", "bin", "target",
		},
		ExcludeExts: []string{
			".pyc", ".so", ".dll", ".exe", ".bin",
			".png", ".jpg", ".gif", ".pdf", ".zip",
			".mod", ".sum", ".lock",
		},
		ExcludeFiles: []string{
			"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
			"requirements.txt", "Pipfile.lock", "poetry.lock",
			"pom.xml", "build.gradle", "gradle.lockfile",
			"Cargo.toml", "Cargo.lock",
			"Gemfile", "Gemfile.lock",
			"composer.json", "composer.lock",
			"kitex_info.yaml",
		},
		MaxFileSize: 1 * 1024 * 1024,
	}
}

func (g *GitCloner) Clone(repoURL, repoName string, filter *FileFilter) (string, error) {
	targetPath := filepath.Join(g.RepoDir, repoName)

	if _, err := os.Stat(targetPath); err == nil {
		if err := os.RemoveAll(targetPath); err != nil {
			return "", fmt.Errorf("删除旧仓库目录失败: %v", err)
		}
	}

	_, err := git.PlainClone(targetPath, false, &git.CloneOptions{
		URL:      repoURL,
		Depth:    1,
		Progress: os.Stdout,
	})

	if err != nil {
		return "", fmt.Errorf("克隆仓库失败: %v", err)
	}

	if filter != nil {
		err = g.cleanFiles(targetPath, filter)
		if err != nil {
			return "", fmt.Errorf("清理文件失败: %v", err)
		}
	}

	return targetPath, nil
}

func (g *GitCloner) cleanFiles(targetPath string, filter *FileFilter) error {
	return filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == targetPath {
			return nil
		}

		relPath, _ := filepath.Rel(targetPath, path)

		shouldSkip := false
		if info.IsDir() {
			shouldSkip = g.shouldSkipDir(relPath, filter)
		} else {
			shouldSkip = filter.ShouldSkip(relPath, info)
		}

		if shouldSkip {
			if info.IsDir() {
				os.RemoveAll(path)
				return filepath.SkipDir
			} else {
				os.Remove(path)
			}
		}

		return nil
	})
}

func (g *GitCloner) shouldSkipDir(relPath string, filter *FileFilter) bool {
	for _, dir := range filter.ExcludeDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func (f *FileFilter) ShouldSkip(path string, info os.FileInfo) bool {
	filename := filepath.Base(path)

	for _, name := range f.IncludeFiles {
		if filename == name {
			return false
		}
	}

	for _, dir := range f.ExcludeDirs {
		if strings.Contains(path, string(os.PathSeparator)+dir+string(os.PathSeparator)) {
			return true
		}
	}

	ext := filepath.Ext(path)
	for _, e := range f.ExcludeExts {
		if ext == e {
			return true
		}
	}

	for _, name := range f.ExcludeFiles {
		if filename == name {
			return true
		}
	}

	if len(f.IncludeExts) > 0 {
		included := false
		for _, e := range f.IncludeExts {
			if ext == e {
				included = true
				break
			}
		}
		if !included {
			return true
		}
	}

	if info.Size() > f.MaxFileSize {
		return true
	}

	return false
}
