package editor

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func GameRunning() (bool, error) {
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("无法检测游戏进程，拒绝写入：%w", err)
	}
	defer windows.CloseHandle(handle)
	var process windows.ProcessEntry32
	process.Size = uint32(unsafe.Sizeof(process))
	err = windows.Process32First(handle, &process)
	for err == nil {
		name := strings.ToLower(windows.UTF16ToString(process.ExeFile[:]))
		if name == "demonic mahjong.exe" || name == "demonicmahjong.exe" {
			return true, nil
		}
		err = windows.Process32Next(handle, &process)
	}
	if err != windows.ERROR_NO_MORE_FILES {
		return false, err
	}
	return false, nil
}

// AtomicSave backs up exact bytes and checks for game/cloud changes before replacement.
func AtomicSave(path, expectedHash string, updated []byte, checkGame func() (bool, error)) (string, error) {
	guard := func() error {
		running, err := checkGame()
		if err != nil {
			return err
		}
		if running {
			return fmt.Errorf("游戏仍在运行，请正常退出游戏后再保存")
		}
		return nil
	}
	if err := guard(); err != nil {
		return "", err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if Hash(original) != expectedHash {
		return "", fmt.Errorf("存档已被修改，请重新加载")
	}
	d, err := parse(updated)
	if err != nil {
		return "", err
	}
	if err = validate(d); err != nil {
		return "", err
	}
	if bytes.Equal(original, updated) {
		return "", nil
	}
	var suffix [4]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return "", err
	}
	backup := path + ".save-editor." + time.Now().Format("20060102_150405") + "_" + hex.EncodeToString(suffix[:]) + ".bak"
	b, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := b.Write(original)
	syncErr := b.Sync()
	closeErr := b.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return "", fmt.Errorf("备份失败，未修改存档")
	}
	verify, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(verify, original) {
		return "", fmt.Errorf("备份校验失败，未修改存档")
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, writeErr = f.Write(updated)
	syncErr = f.Sync()
	closeErr = f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return "", fmt.Errorf("临时文件写入失败，未修改存档")
	}
	if err = guard(); err != nil {
		return "", err
	}
	latest, err := os.ReadFile(path)
	if err != nil || Hash(latest) != expectedHash {
		return "", fmt.Errorf("写入前存档发生变化，已取消保存")
	}
	from, _ := windows.UTF16PtrFromString(f.Name())
	to, _ := windows.UTF16PtrFromString(path)
	if err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return "", err
	}
	verify, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(verify, updated) {
		return "", fmt.Errorf("写入校验失败，备份位于 %s", backup)
	}
	return backup, nil
}
