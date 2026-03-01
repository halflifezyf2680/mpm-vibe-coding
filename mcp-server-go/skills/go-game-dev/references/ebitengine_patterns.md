# Ebitengine (原 Ebiten) 实战代码模式 (Code Cookbook)

> 拒绝废话，直接上生产级代码。以下模式直接复制到项目中即可使用。

## 🟢 1. 场景管理模式 (The Scene Manager)
不要在 `Game` 结构体里写一堆 `if mode == "MENU"`。使用状态机。

```go
// scene.go
type Scene interface {
    Update() error
    Draw(screen *ebiten.Image)
}

// game.go
type Game struct {
    currentScene Scene
}

func (g *Game) Update() error {
    if g.currentScene == nil {
        return nil // 这里的 nil check 视情况而定
    }
    return g.currentScene.Update()
}

func (g *Game) Draw(screen *ebiten.Image) {
    if g.currentScene != nil {
        g.currentScene.Draw(screen)
    }
}

// 切换场景方法
func (g *Game) SwitchTo(s Scene) {
    g.currentScene = s
    // 可在此处加入 Transition 动画逻辑
}
```

## 🟢 2. 2D 摄像机 (The Camera 2D)
在 Ebitengine 中，摄像机本质上是一个全局的变换矩阵 (`GeoM`)。

```go
// camera.go
type Camera struct {
    X, Y     float64
    Zoom     float64
    Rotation float64
}

func NewCamera() *Camera {
    return &Camera{Zoom: 1.0}
}

// WorldToScreen 将世界坐标转换为屏幕渲染矩阵
//screenWidth, screenHeight: 逻辑屏幕宽高
func (c *Camera) WorldToScreen(screenWidth, screenHeight int) ebiten.GeoM {
    m := ebiten.GeoM{}

    // 1. 将原点平移到摄像机位置 (反向)
    m.Translate(-c.X, -c.Y)

    // 2. 旋转
    m.Rotate(c.Rotation)

    // 3. 缩放
    m.Scale(c.Zoom, c.Zoom)

    // 4. 将原点移回屏幕中心 (这样缩放和旋转都是以屏幕为中心)
    m.Translate(float64(screenWidth)/2, float64(screenHeight)/2)

    return m
}

// 使用示例
func (g *GameScene) Draw(screen *ebiten.Image) {
    op := &ebiten.DrawImageOptions{}
    // 获取摄像机变换矩阵
    camMatrix := g.camera.WorldToScreen(320, 240)
    
    op.GeoM.Concat(camMatrix) // 应用摄像机变换
    screen.DrawImage(g.playerSprite, op)
}
```

## 🟢 3. 逻辑输入映射 (Input Action Mapping)
别在逻辑代码里写 `KeySpace`。让美术策划也能改按键。

```go
// input.go
type Action int

const (
    ActionJump Action = iota
    ActionShoot
)

type InputSystem struct {
    keyMap map[Action][]ebiten.Key
}

func (s *InputSystem) IsActionJustPressed(action Action) bool {
    keys, ok := s.keyMap[action]
    if !ok {
        return false
    }
    for _, k := range keys {
        if inpututil.IsKeyJustPressed(k) {
            return true
        }
    }
    // TODO: 这里加上手柄 (Gamepad) 逻辑
    return false
}

// 初始化
inputSys := InputSystem{
    keyMap: map[Action][]ebiten.Key{
        ActionJump: {ebiten.KeySpace, ebiten.KeyW},
    },
}
```

## 🟢 4. 零分配对象池 (Zero-Alloc Object Pool)
Go 泛型让对象池变得简单。这是解决卡顿的终极武器。

```go
// pool.go
type Pool[T any] struct {
    store []T
    factory func() T
}

func NewPool[T any](initialSize int, factory func() T) *Pool[T] {
    p := &Pool[T]{
        store: make([]T, 0, initialSize),
        factory: factory,
    }
    return p
}

func (p *Pool[T]) Get() T {
    if len(p.store) == 0 {
        return p.factory()
    }
    // 弹出最后一个
    idx := len(p.store) - 1
    item := p.store[idx]
    p.store = p.store[:idx]
    return item
}

func (p *Pool[T]) Put(item T) {
    p.store = append(p.store, item)
}

// 实战用法
// 在 Entity Draw 完成后，不要销毁，Put 回去。
// 在 Update 需要生成子弹时，Get 出来并 Reset 状态。
```
