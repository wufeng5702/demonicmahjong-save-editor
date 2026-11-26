package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wufeng5702/demonicmahjong-save-editor/internal/editor"
)

func TestOnlineCatalogKeepsLastValidCache(t *testing.T) {
	valid := []byte(`{"data":{"lingyong":[{"id":1,"name":"娃娃","src":"shared"}],"relics":[{"id":3,"name":"莲台","desc":"你的灵俑槽+1","src":"shared"}],"pailing":[{"id":10027,"name":"鼠灵"}],"baopai":[{"id":1,"name":"红宝牌"}]}}`)
	response := valid
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(response) }))
	defer server.Close()
	cache := t.TempDir()
	client := resourceClient()
	cat, source, err := onlineCatalog(client, server.URL, cache)
	if err != nil || cat.SlotDelta(3) != 1 || source == "" {
		t.Fatalf("online load failed: %v", err)
	}
	response = []byte(`{"data":{"lingyong":[],"relics":[],"pailing":[],"baopai":[]}}`)
	cat, source, err = onlineCatalog(client, server.URL, cache)
	if err != nil || cat.SlotDelta(3) != 1 || source[:len("离线缓存")] != "离线缓存" {
		t.Fatalf("invalid update replaced cache: %s %v", source, err)
	}
	raw, _ := os.ReadFile(filepath.Join(cache, "data.json"))
	if string(raw) != string(valid) {
		t.Fatal("valid cache overwritten")
	}
	server.Close()
	if _, _, err = onlineCatalog(client, server.URL, cache); err != nil {
		t.Fatal("offline cache unavailable")
	}
}

func TestCatalogExcludesBossSkillsAndEnumPlaceholders(t *testing.T) {
	raw := []byte(`{"data":{"lingyong":[{"id":1,"name":"娃娃"},{"id":10001,"name":"BOSS"},{"id":20039,"name":"技能"},{"id":98,"name":"雏凤","tags":["衍生"]}],"relics":[{"id":3,"name":"莲台"},{"id":999,"name":"未实装","src":"enum"}],"pailing":[{"id":10027,"name":"鼠灵"}],"baopai":[{"id":1,"name":"红宝牌"}]}}`)
	cat, err := editor.ParseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Lingyong) != 2 || !cat.Lingyong[98].Derived || len(cat.Relics) != 1 || len(cat.Pailing) != 1 || len(cat.Baopai) != 1 {
		t.Fatal("catalog eligibility failed")
	}
}

func TestReadOnlyServiceRefusesWrites(t *testing.T) {
	s := &Service{readOnly: true}
	if _, err := s.Save(editor.Request{}); err == nil {
		t.Fatal("readonly save accepted")
	}
	if _, err := s.Restore("", "", ""); err == nil {
		t.Fatal("readonly restore accepted")
	}
}
