package editor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fixture() []byte {
	return []byte(`{
  "Version":9,"Coin":801172.2,"Hp":3,"HpMax":3,"AccHpLost":32,
  "InitSwapPaiTotal":6,"SwapPaiTurnTotal":5,"HuPaiSlotsCount":29,
  "LingYongSlotNum":5,"XiaoChouRepositorySlot":4,
  "AllXcp":[1,2],"AllXcpRepository":[],
  "Xcp":{"1":{"id":1,"permanentAddFan":0.0}, "2": {"id":2,"accPermanentMultiIndependent":5.279997,"future":79228162514264337593543950335.0},"99":{"id":99,"mark":7}},
  "RelicList":[{"DisplayId":77,"RoundCount":4,"activationCount":2,"future":{"exact":79228162514264337593543950335.0}}],
  "AccountGameData":{"achievements":{"MaxSumScore":79228162514264337593543950335.0,"MaxHuScore":161461040910412030.0}},
  "HadXiaoChouList":[1,98],"UnlockedLingYongList":[9,17],
  "HadRelicList":[77],"UnlockedRelicList":["QingLingGuan"],
  "MapHistoryPath":"keep me","AllFakeXcp":[20039]
}
`)
}

func catalogFixture() *Catalog {
	return &Catalog{Lingyong: map[int]Item{1: {ID: 1, Name: "万福娃娃"}, 2: {ID: 2, Name: "气筒娃娃"}, 14: {ID: 14, Name: "三头犬"}, 98: {ID: 98, Name: "雏凤", Derived: true}},
		Relics: map[int]Item{77: {ID: 77, Name: "怪异供台", Desc: "你的灵俑槽+1（上限为8）。"}, 3: {ID: 3, Name: "莲台", Desc: "你的灵俑槽+1（上限为8）。"}, 188: {ID: 188, Name: "外婆的祝福", Tags: []string{"供台"}, Desc: "拾取时获得一个供台。"}}}
}

func req(raw []byte) Request { return Request{Hash: Hash(raw)} }

func TestNoOpPreservesEveryByte(t *testing.T) {
	for _, raw := range [][]byte{fixture(), append([]byte{239, 187, 191}, fixture()...)} {
		updated, preview, err := Build(raw, req(raw), catalogFixture())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, updated) || len(preview.Changes) != 0 {
			t.Fatal("no-op changed bytes")
		}
	}
}

func TestNumericsPreserveHugeDecimalsAndAllOtherBytes(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Fields = map[string]string{"Coin": "1000000.25", "Hp": "2"}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Replace(raw, []byte(`"Coin":801172.2`), []byte(`"Coin":1000000.25`), 1)
	expected = bytes.Replace(expected, []byte(`"Hp":3`), []byte(`"Hp":2`), 1)
	if !bytes.Equal(updated, expected) || len(preview.Changes) != 2 {
		t.Fatal("unrelated bytes or decimal literals changed")
	}
}

func TestNumberValidation(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "Infinity", "1e1000000", "0e-1000000", "1.5", "2147483648"} {
		if _, err := number(value, true); err == nil {
			t.Errorf("accepted integer %s", value)
		}
	}
	for input, want := range map[string]string{"1e5": "100000", "001.2500": "1.25", "0.0001": "0.0001", "20.0": "20", "0": "0"} {
		got, err := number(input, false)
		if err != nil || got != want {
			t.Errorf("%s got %s (%v), want %s", input, got, err, want)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	raw := fixture()
	tests := []Request{
		{Hash: "stale"},
		{Hash: Hash(raw), Fields: map[string]string{"UnlockedLingYongList": "1"}},
		{Hash: Hash(raw), Fields: map[string]string{"Hp": "4"}},
		{Hash: Hash(raw), Fields: map[string]string{"HpMax": "0"}},
		{Hash: Hash(raw), Fields: map[string]string{"XiaoChouRepositorySlot": "5"}},
		{Hash: Hash(raw), Equipped: []int{1, 2, 10001}},
		{Hash: Hash(raw), Equipped: []int{1, 2, 20039}},
		{Hash: Hash(raw), Equipped: []int{1, 1, 2}},
		{Hash: Hash(raw), Warehouse: []int{1}},
	}
	for i, request := range tests {
		if _, _, err := Build(raw, request, catalogFixture()); err == nil {
			t.Errorf("accepted invalid request %d", i)
		}
	}
}

func TestMovePreservesGrowthExactly(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Equipped = []int{1}
	request.Warehouse = []int{2}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	if !bytes.Equal(before.values["Xcp"], after.values["Xcp"]) {
		t.Fatal("moving reset growth")
	}
}

func TestReplacementPreservesOtherGrowthAndOrphans(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Equipped = []int{14, 2}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	oldGrowth, _ := parse(before.values["Xcp"])
	newGrowth, _ := parse(after.values["Xcp"])
	for _, key := range []string{"2", "99"} {
		if !bytes.Equal(oldGrowth.values[key], newGrowth.values[key]) {
			t.Fatalf("changed existing growth %s", key)
		}
	}
	if _, exists := newGrowth.values["1"]; exists {
		t.Fatal("removed growth retained")
	}
	if !bytes.Contains(newGrowth.values["14"], []byte(`"roundCount":0`)) || len(preview.Warnings) == 0 {
		t.Fatal("new record or warning missing")
	}
	for _, key := range []string{"HadXiaoChouList", "UnlockedLingYongList", "HadRelicList", "UnlockedRelicList"} {
		if !bytes.Equal(before.values[key], after.values[key]) {
			t.Fatalf("mutated collection %s", key)
		}
	}
}

func TestDerivedIsInventoryOnly(t *testing.T) {
	raw := fixture()
	request := req(raw)
	request.Warehouse = []int{98}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	if len(preview.Warnings) == 0 || !bytes.Equal(before.values["UnlockedLingYongList"], after.values["UnlockedLingYongList"]) {
		t.Fatal("derived item touched shop pool")
	}
}

func TestRelicSlotDeltaAndCounterPreservation(t *testing.T) {
	raw := fixture()
	request := req(raw)
	zero, three := 0, 3
	request.Relics = []RelicChoice{{Ref: &zero}, {ID: &three}}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Slots != 6 || preview.SlotDelta != 1 {
		t.Fatal("slot delta incorrect")
	}
	before, _ := parse(raw)
	after, _ := parse(updated)
	old, _ := decode[[]json.RawMessage](before.values["RelicList"])
	next, _ := decode[[]json.RawMessage](after.values["RelicList"])
	if !bytes.Equal(old[0], next[0]) {
		t.Fatal("existing relic counters changed")
	}
	if catalogFixture().SlotDelta(188) != 0 {
		t.Fatal("guessed slot bonus from tag")
	}
}

func TestRelicBatchAddAllowsDuplicateItems(t *testing.T) {
	raw := fixture()
	zero, three, grandmother := 0, 3, 188
	request := req(raw)
	request.Relics = []RelicChoice{{Ref: &zero}, {ID: &three}, {ID: &grandmother}}
	updated, preview, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	after, _ := parse(updated)
	rows, _ := decode[[]json.RawMessage](after.values["RelicList"])
	if len(rows) != 3 || preview.SlotDelta != 1 {
		t.Fatal("batch add or altar delta incorrect")
	}
	if !strings.Contains(preview.Changes[0].After, "6") {
		t.Fatal("slot preview missing")
	}
	request.Relics = append(request.Relics, RelicChoice{ID: &three})
	updated, preview, err = Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal("duplicate relic rejected:", err)
	}
	after, _ = parse(updated)
	rows, _ = decode[[]json.RawMessage](after.values["RelicList"])
	if len(rows) != 4 || preview.SlotDelta != 2 {
		t.Fatal("duplicate altar did not add its own slot")
	}
	request.Relics = []RelicChoice{{Ref: &zero}, {Ref: &zero}}
	if _, _, err := Build(raw, request, catalogFixture()); err == nil {
		t.Fatal("accepted duplicated original record")
	}
}

func TestManyDuplicateRelicsHaveNoGameQuantityCap(t *testing.T) {
	raw := fixture()
	request := req(raw)
	id := 188
	request.Relics = []RelicChoice{}
	for i := 0; i < 4200; i++ {
		request.Relics = append(request.Relics, RelicChoice{ID: &id})
	}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parse(updated)
	rows, _ := decode[[]json.RawMessage](doc.values["RelicList"])
	if len(rows) != 4200 {
		t.Fatal("relic quantity was capped")
	}
}

func TestRealSaveCopiesRemainUnchanged(t *testing.T) {
	dir := os.Getenv("SAVE_EDITOR_FIXTURES")
	if dir == "" {
		t.Skip("optional integration test uses explicitly provided save copies")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "Slot*.json"))
	if err != nil {
		t.Fatal(err)
	}
	cat := catalogFixture()
	for _, path := range paths {
		_, raw, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		next, _, err := Build(raw, req(raw), cat)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !bytes.Equal(raw, next) {
			t.Fatal("real-save no-op changed bytes")
		}
		request := req(raw)
		request.Fields = map[string]string{"Coin": "1000000.25"}
		next, _, err = Build(raw, request, cat)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := parse(raw)
		after, _ := parse(next)
		for _, key := range before.order {
			if key != "Coin" && !bytes.Equal(before.values[key], after.values[key]) {
				t.Fatalf("changed %s", key)
			}
		}
		onDisk, _ := os.ReadFile(path)
		if !bytes.Equal(raw, onDisk) {
			t.Fatal("integration test modified a save copy")
		}
	}
}

func TestLimitsFiveThroughTwentyAndShrink(t *testing.T) {
	cat := catalogFixture()
	for id := 100; id < 110; id++ {
		cat.Lingyong[id] = Item{ID: id}
	}
	for _, slots := range []int{5, 6, 7, 8, 20} {
		raw := bytes.Replace(fixture(), []byte(`"LingYongSlotNum":5`), []byte(`"LingYongSlotNum":`+strconv.Itoa(slots)), 1)
		request := req(raw)
		for i := 0; i < min(slots, 8); i++ {
			request.Equipped = append(request.Equipped, 100+i)
		}
		if _, _, err := Build(raw, request, cat); err != nil {
			t.Fatalf("capacity %d: %v", slots, err)
		}
		request.Equipped = append(request.Equipped, 109)
		if _, _, err := Build(raw, request, cat); err == nil {
			t.Fatalf("accepted overflow at %d", slots)
		}
	}
	raw := fixture()
	request := req(raw)
	request.Fields = map[string]string{"LingYongSlotNum": "2"}
	request.Relics = []RelicChoice{}
	if _, _, err := Build(raw, request, cat); err == nil {
		t.Fatal("silently deleted overflow items on relic removal")
	}
}

func TestDuplicateJSONKeysRejected(t *testing.T) {
	if _, err := parse([]byte(`{"Coin":1,"Coin":2}`)); err == nil {
		t.Fatal("accepted duplicate keys")
	}
	if _, err := parse([]byte(`{"a":1} garbage`)); err == nil {
		t.Fatal("accepted trailing garbage")
	}
}

func TestAtomicSaveBackupConflictAndGameGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Slot0.json")
	raw := fixture()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	request := req(raw)
	request.Fields = map[string]string{"Coin": "123.5"}
	updated, _, err := Build(raw, request, catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { return true, nil }); err == nil {
		t.Fatal("wrote while game running")
	}
	if _, err := AtomicSave(path, "stale", updated, func() (bool, error) { return false, nil }); err == nil {
		t.Fatal("accepted stale file")
	}
	checks := 0
	if _, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { checks++; return checks > 1, nil }); err == nil {
		t.Fatal("ignored game startup during save")
	}
	onDisk, _ := os.ReadFile(path)
	if !bytes.Equal(onDisk, raw) {
		t.Fatal("failed save modified original")
	}
	backup, err := AtomicSave(path, Hash(raw), updated, func() (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(backup)
	next, _ := os.ReadFile(path)
	if !bytes.Equal(old, raw) || !bytes.Equal(next, updated) || !strings.Contains(backup, ".save-editor.") {
		t.Fatal("backup or write incorrect")
	}
}
