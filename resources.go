package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/wufeng5702/demonicmahjong-save-editor/internal/editor"
)

const website = "https://demonicmahjong.wufeng.site/wiki"

func resourceClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("拒绝跨站资源重定向")
		}
		return nil
	}}
}

func fetchResource(client *http.Client, address string, limit int64) ([]byte, error) {
	response, err := client.Get(address)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("资源返回 HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("资源读取失败或超过大小限制")
	}
	return raw, nil
}

func writeCache(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func onlineCatalog(client *http.Client, base, cache string) (*editor.Catalog, string, error) {
	path := filepath.Join(cache, "data.json")
	raw, networkErr := fetchResource(client, base+"/data.json", editor.MaxSize)
	if networkErr == nil {
		catalog, err := editor.ParseCatalog(raw)
		if err == nil {
			if err := writeCache(path, raw); err != nil {
				return catalog, "在线（缓存写入失败）", nil
			}
			return catalog, "在线 · " + time.Now().Format("2006-01-02 15:04"), nil
		}
		networkErr = err
	}
	if cached, err := os.ReadFile(path); err == nil {
		if catalog, err := editor.ParseCatalog(cached); err == nil {
			source := "离线缓存"
			if info, err := os.Stat(path); err == nil {
				source += " · " + info.ModTime().Format("2006-01-02 15:04")
			}
			return catalog, source, nil
		}
	}
	return nil, "", fmt.Errorf("无法取得在线图鉴且无有效缓存：%w。首次使用需联网，或用 --data 指定本地 data.json", networkErr)
}

func (s *Service) RefreshCatalog() (*Bootstrap, error) {
	if s.dataPath != website+"/data.json" {
		return s.Bootstrap()
	}
	cat, source, err := onlineCatalog(s.client, website, s.cache)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.catalog, s.source = cat, source
	s.mu.Unlock()
	return s.Bootstrap()
}

func (s *Service) icon(kind, id string) ([]byte, string, error) {
	if s.dataPath == website+"/data.json" {
		s.mu.Lock()
		version := s.catalog.Version
		s.mu.Unlock()
		path := filepath.Join(s.cache, "icons", version, kind, id+".webp")
		if raw, err := os.ReadFile(path); err == nil {
			return raw, "image/webp", nil
		}
		address := website + "/icons/" + url.PathEscape(kind) + "/" + url.PathEscape(id) + ".webp"
		if raw, err := fetchResource(s.client, address, 2<<20); err == nil {
			if len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP" {
				_ = writeCache(path, raw)
				return raw, "image/webp", nil
			}
		}
	}
	if s.icons == "" {
		return nil, "", fmt.Errorf("图标暂不可用")
	}
	raw, err := os.ReadFile(filepath.Join(s.icons, kind, id+".png"))
	return raw, "image/png", err
}
