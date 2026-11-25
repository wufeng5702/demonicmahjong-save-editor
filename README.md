# 我在地府打麻将 · 存档编辑器

独立 Windows 桌面工具，使用 **Go + Wails v3 + 原生 HTML/CSS/JS**。不依赖 Python、Node 运行环境，也不启动用于写存档的网络服务。

## 使用

从 GitHub Actions 的成功构建中下载 `DemonicMahjongSaveEditor-windows-amd64` 产物，解压其中的 ZIP，再双击 `DemonicMahjongSaveEditor.exe`。保留随包附带的许可文件。

从源码构建后，双击 `bin/DemonicMahjongSaveEditor.exe`。

编辑正式存档前先正常退出游戏，避免游戏退出时再次保存并覆盖修改。

1. 从不可输入的下拉框选择存档，显示“用户ID · 0号存档 · 完整路径”。也可以通过“手动选择文件”打开 `.json` 副本，读取成功后加入下拉框；无法识别 Steam 账号时标为“外部文件”。
2. 在“数值／灵俑／遗物”页面编辑。此时不会写入文件。
3. 点击“预览修改”。
4. 核对差异和注意事项，勾选确认，再保存。

请先用**存档副本**试用，再用于正式存档。Steam 同步可能改变文件；保存前检测文件变化，并拒绝覆盖已经变化的存档。

## 功能

- 数值：金币、血量、最大血量、累计已损失血量、开局调换牌数、每回合摸牌数、和牌槽数量。
- 灵俑容量：保留累计槽位原始值（可超过 8），实际上场数量为 `min(累计槽位, 8)`；仓库容量读取自存档，通常为 2～4。
- 上场与仓库：网格、多选删除、批量移动。移动保留成长，删除会移除对应成长记录。
- 灵俑候选：默认含衍生灵俑，排除 BOSS 和角色技能；效果直接显示在卡片中。多选添加，数量超出目标空位时禁用添加，后端再次校验。首次新增创建初始成长记录，已有同 ID 成长记录则保留；已持有同 ID 物品不能重复添加。
- 遗物：持有网格多选删除，候选多选添加，卡片直接展示效果；每次为选中的每种遗物增加 1 件，可重复添加同种遗物，不设置游戏持有数量上限。
- 供台／莲台：仅根据明确的“灵俑槽±N”描述联动累计槽位。减槽后若物品超限，必须先处理物品，不会自动删除。
- 保存前预览，自动备份，备份恢复。

**不编辑发现／解锁记录，不改变商店解锁池。**

## 在线数据与离线缓存

只内嵌界面，不内嵌图鉴或游戏图标。默认读取：

- `https://demonicmahjong.wufeng.site/wiki/data.json`
- `https://demonicmahjong.wufeng.site/wiki/icons/lingyong/{ID}.webp`
- `https://demonicmahjong.wufeng.site/wiki/icons/relics/{ID}.webp`

启动时更新图鉴，“更新图鉴”按钮可手动刷新。网络失败或新数据无效时使用最近一次有效缓存；图标按图鉴版本缓存。缺少图标时显示占位，不影响存档编辑。

缓存目录：`%LOCALAPPDATA%\DemonicMahjongSaveEditor\cache`。首次使用需要联网；也可以指定本地图鉴：

```powershell
.\DemonicMahjongSaveEditor.exe --data .\resources\data.json --icons .\resources\icons
```

存档文件、路径、备份和数值不上传到网站。网络仅用于下载图鉴与图标。更新数据不会自动删除存档中的未知物品。

## 安全与限制

- 适配当前 `Version: 9` 的存档结构；其他版本需先用副本验证。
- 未修改的 JSON 字段保留原始字节；大数字不会经过 JavaScript Number 或 Go float64 回写。
- 已有灵俑的成长、遗物的回合和触发计数保留原文；未知字段、未知物品不会被自动清理。
- 保存必须先生成同目录 `.save-editor.时间_随机值.bak`，再写临时文件、检查冲突并替换目标文件。
- 检测到游戏运行或无法检测进程时，拒绝写入；保存过程中游戏启动也会取消保存。
- 恢复备份是**整份恢复**，并先备份当前文件。只列出本修改器生成的备份。
- 离线添加遗物**不等于游戏内拾取**：不生成随机诅咒、不改牌组、不销毁灵俑。除明确的灵俑槽增减外，其他拾取时数值变化不模拟。
- 保存不会修补当前对局内部缓存。部分数值可能被 GameConfig、技能或遗物重算；实际游戏效果仍需验证。

## 构建与测试

开发需要 Windows、Go 1.26+ 和 Node.js 24+。Wails v3.1.0 通过 Go 模块下载，无需额外安装 Wails CLI。前端无框架和 npm 依赖。运行构建后的 EXE 不需要 Go 或 Node，只需要 WebView2。

```powershell
git clone https://github.com/wufeng5702/demonicmahjong-save-editor.git
cd demonicmahjong-save-editor
.\build.ps1 -Version dev
go test ./... -v
go vet ./...
node --test tests/model.test.mjs
```

开发时可用一条命令完成构建并启动：`./build.ps1 -Run`。不传 `-Run` 时只构建，适用于 CI。

构建脚本执行全部测试、JavaScript 语法检查与 `go vet`，使用 `-trimpath` 移除编译路径，输出稳定文件名和第三方许可清单。自定义输出目录和版本：

```powershell
.\build.ps1 -Version v0.1.0 -OutputDir .\dist
```

桌面只读模式与浏览器只读预览：

```powershell
.\bin\DemonicMahjongSaveEditor.exe --readonly
go run . --preview 127.0.0.1:8787 --save-dir .\save-copies
```

浏览器预览只绑定本机，并且**不提供写入接口**。测试文件只写入临时目录，测试不会修改 Steam 正式存档。

在项目目录启动浏览器预览时直接读取 `web/` 源文件，修改 HTML/CSS/JS 后刷新即可；正式 EXE 仍内嵌界面。

测试在临时目录创建合成存档，网络测试使用本地 HTTP 测试服务器；不需要游戏安装目录、真实存档或相邻 wiki 项目。

## GitHub Workflow

`.github/workflows/ci.yml` 在 push、pull request 或手动触发时运行：

1. 在 Windows runner 安装 Go 和 Node。
2. 执行 Go 测试、前端检查、`go vet` 和 Windows x64 构建。
3. 打包 EXE、说明文档和许可文件，上传 Actions artifact（保留 14 天）。

工作流仅有仓库读取权限，第三方 Actions 固定完整 commit SHA。它不自动创建 Release、发布评论或推送分支；正式发行需要另行审核和发布。

## 许可

修改器源码采用 MIT 许可，见 [LICENSE](LICENSE)。游戏资料、图标及第三方组件的权利说明见 [NOTICE.md](NOTICE.md)。仓库与构建产物不包含真实存档、下载缓存或游戏资源。

## 结构

```text
main.go / service.go     Wails 窗口、文件选择、编辑服务
resources.go            在线图鉴与图标缓存
internal/editor/        精确 JSON 替换、槽位校验、安全保存及测试
web/                    原生 HTML/CSS/JS 页面
tests/                  前端逻辑测试
build.ps1               测试、构建及可选启动
```
