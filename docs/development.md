# 开发指南

## 构建与测试

在 Windows x64 上开发需要 Go 1.27+、Node.js 24+ 和 Microsoft Edge WebView2 Runtime。构建脚本会通过 Go 下载 Wails v3.1.0；前端使用原生 HTML、CSS 和 JavaScript，无需安装 npm 依赖。

在仓库根目录执行：

```powershell
.\build.ps1
```

脚本执行测试和 `go vet`，再构建 `bin/DemonicMahjongSaveEditor.exe` 并生成 `THIRD_PARTY_NOTICES.txt`。用 `-Run` 在构建后启动应用；用 `-Version` 和 `-OutputDir` 指定窗口中显示的版本与输出目录。

单独运行检查：

```powershell
go test ./... -count=1
go vet ./...
node --check web/app.js
node --test tests/model.test.mjs
```

Go 测试在临时目录创建合成存档，图鉴网络测试使用本地 HTTP 测试服务器。测试无需安装游戏或提供真实存档。

## 只读调试

准备一个放有 `Slot0.json` 等存档副本的目录。`--save-dir` 会指定存档扫描目录：

```powershell
.\bin\DemonicMahjongSaveEditor.exe --readonly --save-dir .\save-copies
go run . --preview 127.0.0.1:8787 --save-dir .\save-copies
```

浏览器预览只绑定本机，支持读取和对比改动。它从当前目录的 `web/` 读取前端源文件，修改后刷新即可。桌面版内嵌前端资源。

## 图鉴与资源

默认从 [Wiki](https://demonicmahjong.wufeng.site/wiki/) 下载 `data.json` 和 `icons/{分类}/{ID}.webp`。图标分类为 `lingyong`、`relics`、`pailing`、`baopai`。应用图标和 34 张牌面图内嵌在 EXE 中。

缓存保存在 `%LOCALAPPDATA%\DemonicMahjongSaveEditor\cache`。图标按图鉴版本缓存；网络不可用或新数据无效时，使用最近一次有效的图鉴缓存。缺图显示占位。可用本地 JSON 和 PNG 图标调试：

```powershell
.\bin\DemonicMahjongSaveEditor.exe --readonly --data .\resources\data.json --icons .\resources\icons
```

指定 `--data` 后使用本地图鉴。省略 `--icons` 时，从 `data.json` 同级的 `icons/` 目录读取 `{分类}/{ID}.png`。

## 存档读写

- **发现存档**：从 Steam 注册表和常见安装路径查找 `userdata/*/3444020/remote/Slot*.json`。界面将 `Slot0/1/2.json` 显示为 1／2／3 号，保留原文件名。
- **保留原文**：使用 Go 的 `encoding/json/v2` 和 `encoding/json/jsontext` 定位并替换目标字段。未改动字段保留原始字节；编辑数值以字符串在前后端传递，避免大数字被浮点转换。
- **牌库**：`WorkbenchPaiMianBindList` 可以是空数组或稀疏列表。未编辑牌库时保留该字段原文；编辑后按万、筒、索、东南西北中发白的顺序写回 34 条。补全的牌默认 `DeltaBaseScore: 0.0`、`BaoId: 0`、`LingId: 0`，已有条目中的未知字段保留。
- **物品与槽位**：移动灵俑保留成长记录，移除最后一件同 ID 灵俑时删除对应成长记录。原有遗物保留各自的计数。扩槽遗物根据明确的「灵俑槽±N」描述联动累计槽位；实际上场上限为 `min(累计槽位, 8)`，仓库容量按存档字段校验。减槽超限时要求用户先移出物品。
- **保存与恢复**：预览生成目标内容的哈希，保存时重新计算并比对。写入前核对存档哈希和游戏进程，在同目录写入并校验 `.save-editor.时间_随机值.bak` 备份。替换文件前再次检查存档和游戏进程。恢复仅接受本工具为当前存档生成的备份，并走同一套写入流程。

部分数值由 GameConfig、角色技能或遗物在游戏内重算。离线编辑只改存档字段，不模拟拾取事件，也不修补当前对局缓存。

## CI 与发布

- [CI 工作流](../.github/workflows/ci.yml)：分支推送、PR 或手动触发时，在 Windows runner 运行 `build.ps1`，上传 Windows x64 ZIP，产物保留 14 天。
- [Release 工作流](../.github/workflows/release.yml)：推送 `vx.y.z` 格式的标签后，测试并构建对应版本，打包 Windows x64 ZIP 和 `SHA256SUMS.txt`。发布任务先将附件上传到草稿，成功后发布；已有 Release 不会被覆盖。

CI 和 Release 构建任务使用读取权限，仅 Release 发布任务可写入仓库内容。第三方 Actions 固定到提交 SHA。发行说明使用[固定模板](../.github/release-notes.md)，推送版本标签前先审核发布内容。

## 目录结构

| 路径 | 用途 |
| --- | --- |
| `main.go`、`service.go` | Wails 窗口、文件选择、编辑接口与只读预览 |
| `resources.go` | 图鉴下载与缓存 |
| `internal/editor/` | 存档解析、编辑校验、备份与写入 |
| `web/`、`assets/paimian/` | 前端页面与内嵌牌面图 |
| `tests/` | 前端测试 |
| `build.ps1` | 执行检查、构建 EXE 和生成许可清单 |
