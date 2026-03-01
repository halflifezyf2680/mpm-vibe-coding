# 游戏发布规则与平台索引 (Publishing & Rules)

一份面向专业程序员的 Go 语言游戏发布指南。

## 🏪 1. Steam (独立游戏的终点站)
- **准备流程**: 在 [Steamworks](https://partner.steamgames.com/) 注册并支付 App Fee。
- **Go 语言注意事项**: 
    - 如果使用了 CGO（如 Raylib 或特定物理库），必须确保目标系统的动态库（如 `.dll` 或 `.so`）正确打包，或采用**静态链接**。
    - **成就与云存档**: 推荐使用 `github.com/lucasb-eyer/go-steamworks` 封装库来接入 Steam API。
- **合规性要求**: 
    - 必须通过 Steam Deck 兼容性测试（手柄适配是核心）。
    - 游戏必须支持原生全屏模式切换。
- **官网链接**: [Steamworks 文档中心](https://partner.steamgames.com/doc/home)

## 🏪 2. Itch.io (开发者的实验场)
- **自动化工具**: 强烈建议使用 [Butler](https://itch.io/docs/butler/) 命令行工具整合到 CI/CD 流程中，实现一键推包。
- **Web (Wasm) 发布**: 
    - Go 默认编译出的 Wasm 较大，提审前必须使用 `wasm-opt -Oz` 进行极限优化。
    - 使用 `gzip` 或 `brotli` 压缩 Wasm 文件以缩短玩家加载时间。
- **官网链接**: [Itch.io 创作者指南](https://itch.io/docs/creators/getting-started)

## 🏪 3. 移动端 (App Store & Google Play)
- **非主流选择的突围**: 在 Go 中通常不建议直接写 Android 环境，而是使用 `gomobile` 将 Go 逻辑导出为 `.aar` (Android) 或 `.framework` (iOS) 给原生工程调用。
- **核心准则**:
    - **Apple**: 必须在 Mac 上编译，且必须处理 Bitcode。
    - **Google**: 必须提交 AAB (Android App Bundle) 格式，而非传统的 APK。
- **官方库**: [golang.org/x/mobile](https://pkg.go.dev/golang.org/x/mobile)

## 🏗️ 4. 交叉编译全局矩阵 (Cross-Compilation Matrix)
Go 的交叉编译是其杀手锏。发布不同版本时请参考以下参数：

| 目标平台 | GOOS | GOARCH | 交付产物 | 备注 |
| :--- | :--- | :--- | :--- | :--- |
| **Windows** | `windows` | `amd64` | `.exe` | 建议增加 `-ldflags="-H windowsgui"` 隐藏控制台窗口 |
| **macOS** | `darwin` | `arm64` | `App Bundle` | 必须针对 Apple Silicon 签名 (Code Signing) |
| **Linux** | `linux` | `amd64` | `Elf Binary` | 建议在 Ubuntu 环境下进行静态编译以增强兼容性 |
| **浏览器** | `js` | `wasm` | `.wasm` | 需要配套 `wasm_exec.js` 启动脚本 |

> 💡 **专家提示**: 为了减小二进制体积并防止被轻易反编译，请在编译时使用 `-ldflags="-s -w"`。
