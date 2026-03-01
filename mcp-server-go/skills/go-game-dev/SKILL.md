---
name: go-game-dev
description: 针对 Go (Golang) 语言及其游戏引擎（如 Ebitengine 或 Raylib-go）的独立游戏开发专家指南。涵盖内存管理、并发安全游戏循环、资产嵌入 (go:embed) 以及多平台交叉编译等核心领域。适用于利用 Go 生态构建高性能 2D/3D 游戏的场景。
---

# Go 游戏开发专家 (Go Game Development Expert)

本技能为独立游戏开发者提供了一套使用 Go 语言构建专业级游戏的标准化工程流和深度决策模型。

## ✅ DOS (必须做的事)
1.  **热路径零分配 (Zero Allocation)**:
    - 必须在 `Update()` 和 `Draw()` 循环复用切片（`slice = slice[:0]`）和对象池（`sync.Pool` 或泛型 Pool），确保每帧 GC 压力为零。
2.  **主线程封印 (Main Thread Binding)**:
    - 所有的绘图逻辑（`DrawImage`, `NewImage`）必须严格限制在 `Game.Draw()` 主回调中执行，以确保运行在 OS 主线程。
3.  **并发安全 (Concurrency Safety)**:
    - 在 Goroutine 中处理 AI 或物理运算时，必须通过 `Channel` 回传结果到主循环，或者对共享状态加 `sync.RWMutex` 锁。
4.  **显存手动管理 (Manual VRAM Disposal)**:
    - 任何动态创建的 `ebiten.Image` (非嵌入资源)，必须在使用完毕后显式调用 `.Dispose()`，不得依赖 GC。

## ❌ DON'TS (绝对禁止的事)
1.  **🚫 禁止在循环中创建对象**:
    - 严禁在 `Update` 中使用 `fmt.Sprintf`（隐式接口转换）、`make([]int)` 或 `&Vector{}`。
2.  **🚫 禁止跨线程渲染**:
    - 绝不要在自己启动的 `go func()` 中调用任何图形 API。这会导致随机崩溃或 `invalid memory address`。
3.  **🚫 禁止并发读写 Map**:
    - 严禁在未加锁的情况下从多个 Goroutine 访问同一个 `map`。Go 的 Map 竞态检测会直接 Panic 整个进程。
4.  **🚫 禁止 CGO 频繁交互**:
    - 避免在 `Update` 循环中高频调用微小的 C 函数（如 Raylib 的单点绘图）。必须在 Go 侧批处理数据，一次性传递给 C 侧。

## Workflows

### Phase 1: 标准化工程脚手架 (Project Scaffolding)
> 切勿将所有代码塞进 `main.go`。请遵循 Go 标准项目布局。

- **目录结构**:
    - `cmd/game/main.go`: 程序入口，仅负责初始化窗口和启动 Game Loop。
    - `internal/game/`: 核心逻辑，对外部不可见。
    - `internal/assets/`: 嵌入式文件系统 (`embed.FS`) 的对接口。
    - `internal/ecs/`: (可选) 存放 Component 和 System 定义。
- **动作**:
    1. `go mod init <module>`
    2. 创建 `internal/game/game.go` 并定义 `Game` 结构体。
    3. 在 `cmd/game/main.go` 中调用 `ebiten.RunGame(&game.Game{})`。

### Phase 2: 场景管理状态机 (Scene Manager FSM)
> 游戏不是一个大循环，而是一系列场景的切换（Logo -> Menu -> Play -> Over）。

- **模式**: 
    - 定义 `Scene` 接口：必须包含 `Update() error` 和 `Draw(screen *ebiten.Image)`。
    - 在 `Game` 结构体中持有 `currentScene Scene`。
- **动作**:
    1. 实现 `SceneManager`，提供 `SwitchTo(Scene)` 方法。
    2. 确保 `Game.Update()` 只是一层代理：`return g.currentScene.Update()`。
    3. 实现第一个 `TitleScene` 并挂载。

### Phase 3: 数据导向实体设计 (Data-Oriented Entity Design)
> 随着实体数量增加，OOP 继承树会成为性能瓶颈。

- **策略**:
    - **少量实体 (<500)**: 使用 **组合模式 (Composition)**。定义 `GameObject` 结构体，内嵌 `*Sprite` 和 `*Position`。
    - **海量实体 (>1000)**: 必须上 **ECS** (Entity Component System)。推荐使用 `donburi` 或 `arche` 库。
- **动作**:
    1. 定义 `Component` 接口或结构体数据。
    2. 实现 `System`（如 `MovementSystem`），只遍历拥有 `Velocity` 组件的实体。

### Phase 4: 输入与资源抽象 (Input & Asset Abstraction)
> 不要直接在逻辑代码里写 `ebiten.IsKeyPressed(ebiten.KeySpace)`。

- **输入映射 (Input Mapping)**:
    - 创建 `InputSystem`，将物理按键（Space, A, GamepadA）映射为逻辑意图（`ActionJump`, `ActionShoot`）。
    - 逻辑层只判断 `input.IsActionJustPressed(ActionJump)`。
- **资源池化**:
    - 建立全局（或场景级）`AssetLoader`。
    - 使用 `sync.Map` 或普通 `map` 缓存加载过的图片，避免重复 I/O。

### Phase 5: 构建自动化 (Build Automation)
> 本地能跑不代表能发给玩家。

- **动作**:
    1. 配置 `.air.toml` 实现代码热重载（虽然对图形窗口支持有限，但对逻辑调试有用）。
    2. 编写 `Makefile` 或 `scripts/build.py`，固化构建参数：
       - Windows: `GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w"`
       - Web: `GOOS=js GOARCH=wasm go build -o game.wasm`

---

## Bundled Resources
- **References**:
  - `references/ebitengine_patterns.md`: Ebitengine 深度设计模式、性能陷阱与优化建议。
  - `references/vfx_guide.md`: 视觉特效指南 (Screen Shake, Shaders, Particles)。
  - `references/publishing.md`: Steam, Itch.io 及移动端商店的官方发布规则、编译参数与审核指南。
  - `references/genre_cardgame.md`: 卡牌游戏工程化指南 (手牌算法, 数据结构, 状态机)。
  - `references/genre_simulation.md`: 模拟经营架构通识 (区块网格, 资源流图, 确定性 Tick)。
- **Scripts**:
  - `scripts/build_all.py`: 一键式多平台交叉编译自动化工具（支持 Wasm, Windows, Linux, macOS）。
