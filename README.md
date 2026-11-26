# 我在地府打麻将 · 存档编辑器

《我在地府打麻将》Steam 版的 Windows 存档编辑器。可以搜索灵俑、查看效果，批量调整上场与仓库配置。保存前可以预览改动，写入时会自动备份原存档。

**[下载最新 Windows x64 版](https://github.com/wufeng5702/demonicmahjong-save-editor/releases/latest)** · 运行需要 Microsoft Edge WebView2 Runtime。

![灵俑编辑界面](docs/images/lingyong-editor.png)

## 灵俑编辑

- 按名称、ID 或效果搜索，用稀有度和标签筛选。候选卡片直接显示灵俑效果。
- 多选添加、移除灵俑，或在上场与仓库之间批量移动。移动会保留成长记录。
- 界面显示上场上限和仓库容量。选中数量超过空位时，不能添加到对应位置。

## 快速开始

1. 退出游戏，解压下载的 ZIP，运行 `DemonicMahjongSaveEditor.exe`。
2. 在下拉框选择 Steam 存档，或用 **手动选择文件** 打开一份 `.json` 存档副本。
3. 打开 **灵俑** 页，勾选目标，然后点击对应的添加、删除或移动按钮。
4. 点击 **预览修改**，核对差异后勾选确认，再点击 **确认保存并备份**。

备份放在原存档旁边。需要撤回时可在 **备份恢复** 中选择备份，恢复会替换整份存档。

## 使用说明

- 存档只在本机处理。首次启动需要联网下载[图鉴](https://demonicmahjong.wufeng.site/wiki/)数据；已有缓存后可以离线使用。
- 游戏运行中，或存档已被游戏、Steam 云同步等改动时，工具会拒绝覆盖。
- 适配 `Version: 9` 的存档。灵俑的发现记录、解锁状态和商店解锁池保持原样。

## 开发与许可

[构建、测试和发布](docs/development.md) · [MIT 许可](LICENSE) · [第三方资料与许可范围](NOTICE.md)
