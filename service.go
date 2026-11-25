package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"demonicmahjong-save-editor/internal/editor"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows/registry"
)

type Service struct {
	app      *application.App
	catalog  *editor.Catalog
	dataPath string
	icons    string
	saveDir  string
	readOnly bool
	client   *http.Client
	cache    string
	source   string
	mu       sync.Mutex
}

type Bootstrap struct {
	DataPath  string                   `json:"dataPath"`
	Saves     []string                 `json:"saves"`
	Catalog   map[string][]editor.Item `json:"catalog"`
	Fields    []editor.Field           `json:"fields"`
	Running   bool                     `json:"running"`
	ReadOnly  bool                     `json:"readOnly"`
	SlotRules map[int]int              `json:"slotRules"`
	Source    string                   `json:"source"`
	Version   string                   `json:"dataVersion"`
}

func (s *Service) Bootstrap() (*Bootstrap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	running, err := editor.GameRunning()
	if err != nil {
		return nil, err
	}
	rules := map[int]int{}
	for id := range s.catalog.Relics {
		if delta := s.catalog.SlotDelta(id); delta != 0 {
			rules[id] = delta
		}
	}
	return &Bootstrap{s.dataPath, s.detectSaves(), map[string][]editor.Item{
		"lingyong": editor.SortedItems(s.catalog.Lingyong), "relics": editor.SortedItems(s.catalog.Relics),
	}, editor.Fields, running, s.readOnly, rules, s.source, s.catalog.Version}, nil
}

func (s *Service) Load(path string) (*editor.Snapshot, error) { return editor.Load(path) }

func (s *Service) ChooseSave() (string, error) {
	if s.app == nil {
		return "", fmt.Errorf("浏览器预览模式请直接输入文件路径")
	}
	return s.app.Dialog.OpenFile().SetTitle("选择《我在地府打麻将》存档").AddFilter("JSON 存档", "*.json").PromptForSingleSelection()
}

func (s *Service) Preview(request editor.Request) (*editor.Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, raw, err := editor.Read(request.Path)
	if err != nil {
		return nil, err
	}
	_, preview, err := editor.Build(raw, request, s.catalog)
	return preview, err
}

type SaveResult struct {
	Backup string           `json:"backup"`
	Save   *editor.Snapshot `json:"save"`
}

func (s *Service) Save(request editor.Request) (*SaveResult, error) {
	if s.readOnly {
		return nil, fmt.Errorf("当前为只读预览模式，不写入存档")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, raw, err := editor.Read(request.Path)
	if err != nil {
		return nil, err
	}
	updated, preview, err := editor.Build(raw, request, s.catalog)
	if err != nil {
		return nil, err
	}
	if preview.PreviewHash != request.PreviewHash {
		return nil, fmt.Errorf("修改内容与预览不一致，请重新预览")
	}
	backup, err := editor.AtomicSave(path, request.Hash, updated, editor.GameRunning)
	if err != nil {
		return nil, err
	}
	loaded, err := editor.Load(path)
	return &SaveResult{backup, loaded}, err
}

func (s *Service) Backups(path string) ([]string, error) {
	path, _, err := editor.Read(path)
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(path + ".save-editor.*.bak")
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	if files == nil {
		files = []string{}
	}
	return files, err
}

func (s *Service) Restore(path, hash, backup string) (*SaveResult, error) {
	if s.readOnly {
		return nil, fmt.Errorf("只读模式不能恢复备份")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, _, err := editor.Read(path)
	if err != nil {
		return nil, err
	}
	backup, err = filepath.Abs(backup)
	if err != nil || !strings.EqualFold(filepath.Dir(backup), filepath.Dir(path)) ||
		!strings.HasPrefix(strings.ToLower(filepath.Base(backup)), strings.ToLower(filepath.Base(path)+".save-editor.")) || filepath.Ext(backup) != ".bak" {
		return nil, fmt.Errorf("只能恢复当前存档由本修改器创建的备份")
	}
	info, err := os.Stat(backup)
	if err != nil || info.Size() > editor.MaxSize {
		return nil, fmt.Errorf("备份不存在或过大")
	}
	raw, err := os.ReadFile(backup)
	if err != nil {
		return nil, err
	}
	newBackup, err := editor.AtomicSave(path, hash, raw, editor.GameRunning)
	if err != nil {
		return nil, err
	}
	loaded, err := editor.Load(path)
	return &SaveResult{newBackup, loaded}, err
}

func (s *Service) detectSaves() []string {
	if s.saveDir != "" {
		paths, _ := filepath.Glob(filepath.Join(s.saveDir, "Slot*.json"))
		if paths == nil {
			return []string{}
		}
		return paths
	}
	roots := []string{filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"), `E:\Program Files (x86)\Steam`}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err == nil {
		value, _, err := key.GetStringValue("SteamPath")
		key.Close()
		if err == nil {
			roots = append(roots, value)
		}
	}
	seen := map[string]bool{}
	result := []string{}
	pattern := regexp.MustCompile(`^Slot\d+\.json$`)
	for _, root := range roots {
		paths, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "3444020", "remote", "Slot*.json"))
		for _, path := range paths {
			path, _ = filepath.Abs(path)
			if pattern.MatchString(filepath.Base(path)) && !seen[strings.ToLower(path)] {
				result = append(result, path)
				seen[strings.ToLower(path)] = true
			}
		}
	}
	sort.Strings(result)
	return result
}
