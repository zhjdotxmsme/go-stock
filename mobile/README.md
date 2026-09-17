# go-stock Android 移动端 (Wails v3)

复用主仓库 `backend/` 的业务代码，通过 [Wails v3](https://v3.wails.io)（beta.20）打包为 Android APK。
桌面端（仓库根目录，Wails v2）**不受影响**，两套入口并行共存：

| | 桌面端 | 移动端 |
|---|---|---|
| 位置 | 仓库根目录 | `mobile/`（独立 Go module，`replace go-stock => ../` 复用 backend） |
| Wails | v2.13（无 Android 能力） | v3.0.0-beta.20（移动端为 Experimental） |
| 构建 | `wails build` | `wails3 task android:package`（或 GitHub Actions） |

## 已适配内容（PoC）

- **服务绑定**：`StockHandler`（自选列表 / K线）、`AgentHandler`（AI 流式问答）、`MobileService`（脱敏 AI 配置）
- **事件流**：backend 层经 `backend/emitter` 注入 `app.Event.Emit`，`agent-message` 流式对话可用
- **数据**：SQLite（纯 Go 驱动）落在应用私有目录（`application.Mobile.StoragePath()` + chdir）
- **前端**：极简两页（行情 K线 + AI 问答），`@wailsio/runtime` + ECharts，深色竖屏布局

## 云端打包（推荐，本地无需 Android 工具链）

GitHub → Actions → **Android APK** → Run workflow → 选架构（arm64 / amd64 / fat）。
产物在 run 的 Artifacts 里（`go-stock-android-<arch>`），debug keystore 签名可直接安装。

## 本地打包

前置：Go 1.26+、JDK 21（`JAVA_HOME`）、Android SDK（platform-tools / platforms;android-35 /
build-tools;35.0.0 / **ndk;26.3.11579264**）、wails3 CLI：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.20
cd mobile
wails3 task android:package          # arm64 release APK → bin/go-stock-mobile.apk
wails3 task android:run:device       # 真机安装运行（需 adb + 设备）
wails3 task android:run              # 模拟器安装运行
```

前端开发预览：`cd mobile/frontend && npm install && npm run dev`。

## 架构与已知限制

- **backend 解耦**：backend 层不再直接调用 `runtime.EventsEmit`，事件统一经注入的
  `emitter.Emitter` 发出（桌面=v2 适配、移动端=v3 适配，见 `app.go` / `mobile/main.go`）。
- **每设备独立数据**：手机与桌面各自独立 SQLite，自选/持仓/AI 配置不互通（设计决策）。
  AI 配置需先在桌面端配置好后随数据库初始化，或后续增加移动端配置页。
- **已知的 v3 移动端上游问题**（Experimental）：
  - wailsapp/wails#5859 —— 流式请求进行中切后台/杀进程可能崩溃（真机回归重点）
  - wailsapp/wails#6110 —— `System.IsAndroid()` 恒 false，平台检测勿依赖该 API
- **包名**：当前沿用模板默认 `com.wails.app`（与 gradle 模板 Java 包名一致）；正式发布前统一改名为
  `com.sparkmemory.gostock`（移动 Java 源码目录 + namespace + `build/android/Taskfile.yml` 的 APP_ID）。
- **后续路线**：完整 24 页迁移、底部 Tab 导航、K线触摸优化等见主计划（阶段 3）。
