package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"demonicmahjong-save-editor/internal/editor"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:web
var assets embed.FS

func main() {
	data := flag.String("data", "", "图鉴 data.json 路径")
	icons := flag.String("icons", "", "图标目录")
	saveDir := flag.String("save-dir", "", "指定测试或存档目录")
	preview := flag.String("preview", "", "只读浏览器预览地址，例如 127.0.0.1:8787")
	readOnly := flag.Bool("readonly", false, "禁止保存与恢复备份")
	flag.Parse()
	client := resourceClient()
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		fatal(err)
	}
	cache := filepath.Join(cacheRoot, "DemonicMahjongSaveEditor", "cache")
	var cat *editor.Catalog
	source := "本地图鉴"
	if *data == "" {
		cat, source, err = onlineCatalog(client, website, cache)
		*data = website + "/data.json"
	} else {
		cat, err = editor.LoadCatalog(*data)
	}
	if err != nil {
		fatal(err)
	}
	absolute := *data
	if !strings.HasPrefix(absolute, "https://") {
		absolute, _ = filepath.Abs(*data)
	}
	if *icons == "" {
		*icons = filepath.Join(filepath.Dir(filepath.Dir(absolute)), "output", "site", "icons")
		if _, err := os.Stat(*icons); err != nil {
			*icons = filepath.Join(filepath.Dir(absolute), "icons")
		}
	}
	s := &Service{catalog: cat, dataPath: absolute, icons: *icons, saveDir: *saveDir, readOnly: *readOnly || *preview != "", client: client, cache: cache, source: source}
	web, _ := fs.Sub(assets, "web")
	if *preview != "" {
		cwd, _ := os.Getwd()
		live := filepath.Join(cwd, "web")
		if info, err := os.Stat(filepath.Join(live, "index.html")); err == nil && info.Mode().IsRegular() {
			web = os.DirFS(live) // Read-only development preview uses live HTML/CSS/JS.
		}
	}
	static := http.FileServer(http.FS(web))
	iconPattern := regexp.MustCompile(`^/icons/(lingyong|relics)/\d+\.png$`)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if iconPattern.MatchString(r.URL.Path) {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/icons/"), "/")
			raw, mime, err := s.icon(parts[0], strings.TrimSuffix(parts[1], ".png"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", mime)
			w.Write(raw)
			return
		}
		static.ServeHTTP(w, r)
	})
	if *preview != "" {
		if !strings.HasPrefix(*preview, "127.0.0.1:") {
			fmt.Fprintln(os.Stderr, "预览服务器只允许绑定 127.0.0.1")
			os.Exit(1)
		}
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			panic(err)
		}
		token := hex.EncodeToString(secret[:])
		mux := http.NewServeMux()
		mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			if r.Host != *preview || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+*preview) {
				http.Error(w, "invalid origin", 403)
				return
			}
			if r.URL.Path == "/api/Bootstrap" {
				boot, err := s.Bootstrap()
				if err != nil {
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"token": token, "boot": boot})
				return
			}
			if r.Method != "POST" || r.Header.Get("X-Editor-Token") != token {
				http.Error(w, "read-only preview", 403)
				return
			}
			var args []json.RawMessage
			err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&args)
			var result any
			if err == nil {
				switch r.URL.Path {
				case "/api/RefreshCatalog":
					result, err = s.RefreshCatalog()
				case "/api/Load", "/api/Backups":
					var path string
					if len(args) != 1 {
						err = fmt.Errorf("参数数量错误")
						break
					}
					if err = json.Unmarshal(args[0], &path); err == nil {
						if r.URL.Path == "/api/Load" {
							result, err = s.Load(path)
						} else {
							result, err = s.Backups(path)
						}
					}
				case "/api/Preview":
					var req editor.Request
					if len(args) != 1 {
						err = fmt.Errorf("参数数量错误")
						break
					}
					if err = json.Unmarshal(args[0], &req); err == nil {
						result, err = s.Preview(req)
					}
				default:
					err = fmt.Errorf("浏览器预览模式禁止写入，请使用桌面 EXE")
				}
			}
			if err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(result)
		})
		mux.Handle("/", handler)
		fmt.Println("Read-only UI preview: http://" + *preview)
		if err := http.ListenAndServe(*preview, mux); err != nil {
			panic(err)
		}
		return
	}
	app := application.New(application.Options{
		Name: "DemonicMahjong Save Editor", Description: "我在地府打麻将 · 本地存档编辑器",
		Assets: application.AssetOptions{Handler: handler},
	})
	s.app = app
	app.RegisterService(application.NewService(s))
	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "我在地府打麻将 · 存档编辑器", Width: 1440, Height: 820, MinWidth: 1050, MinHeight: 650})
	if err = app.Run(); err != nil {
		fatal(err)
	}
}
