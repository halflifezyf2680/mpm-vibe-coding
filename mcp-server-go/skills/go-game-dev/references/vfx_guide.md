# Ebitengine 视觉特效指南 (VFX & Juice Guide)

> “游戏好不好玩看核心 Loop，卖不卖得动看 Juice。” —— 本指南提供开箱即用的“打击感”与“氛围感”代码。

## 🎬 1. 屏幕震动 (Screen Shake) - 基于创伤模型 (Trauma)
不要使用随机震动 `Random(-5, 5)`，那看起来像 Bug。请使用 GDC 推荐的 **Trauma** 模型：震动是 Trauma 的平方，且随时间线性衰减。

```go
// camera.go 中集成
type ScreenShake struct {
    trauma      float64 // 范围 0.0 ~ 1.0
    decay       float64 // 每帧衰减量 (如 0.02)
    maxAngle    float64 // 最大旋转角度 (弧度)
    maxOffset   float64 // 最大位移 (像素)
    seed        float64 // 柏林噪声种子 (可选，简单起见用随机数)
}

func (s *ScreenShake) AddTrauma(amount float64) {
    s.trauma = math.Min(s.trauma+amount, 1.0)
}

func (s *ScreenShake) Update() {
    if s.trauma > 0 {
        s.trauma = math.Max(s.trauma-s.decay, 0)
    }
}

// 在 WorldToScreen 矩阵生成时调用
func (s *ScreenShake) Apply(geom *ebiten.GeoM, timeTick int) {
    if s.trauma <= 0 {
        return
    }
    
    // 震动强度是 trauma 的平方 (让剧烈震动更明显，微弱震动更平滑)
    shake := s.trauma * s.trauma
    
    // 生成基于时间的伪随机位移 (Perlin Noise 效果更好，这里用简易版)
    // 关键：不要每一帧都随机跳变，那样太闪。可以用 timeTick 控制频率。
    angle := (rand.Float64()*2 - 1) * s.maxAngle * shake
    offsetX := (rand.Float64()*2 - 1) * s.maxOffset * shake
    offsetY := (rand.Float64()*2 - 1) * s.maxOffset * shake
    
    geom.Rotate(angle)
    geom.Translate(offsetX, offsetY)
}
```

## ⚡ 2. 命中闪白 (Hit Flash)
简单的将 Sprite 变白，用于受击反馈。Ebitengine 通过 `ColorM` 实现。

```go
// draw_utils.go
var whiteShader, _ = ebiten.NewShader([]byte(`
    package main
    func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
        // 获取原图 alpha
        srcColor := imageSrc0At(texCoord)
        if srcColor.a == 0.0 {
            return vec4(0)
        }
        // 返回纯白，保留原 alpha
        return vec4(1, 1, 1, srcColor.a)
    }
`))

func DrawHitFlash(screen *ebiten.Image, sprite *ebiten.Image, geom ebiten.GeoM) {
    op := &ebiten.DrawRectShaderOptions{}
    op.GeoM = geom
    op.Images[0] = sprite
    
    // 绘制纯白 Shader
    screen.DrawRectShader(sprite.Bounds().Dx(), sprite.Bounds().Dy(), whiteShader, op)
}
```

## 📺 3. Kage Shader: CRT 复古滤镜
Ebitengine 独有的 Go 风格着色器语言 (Kage)。这是一个高性能的全屏 CRT 效果。

```go
// assets/shaders/crt.kage
//go:embed assets/shaders/crt.kage
var crtKage []byte

// 在 Game.Draw 的最后一步调用
// screen 是通过 Convert 得到的全屏 Image
func DrawCRT(finalScreen *ebiten.Image) {
    op := &ebiten.DrawRectShaderOptions{}
    op.Uniforms = map[string]interface{}{
        "Time": float32(gameTick) / 60.0,
    }
    op.Images[0] = finalScreen
    // 绘制到物理屏幕
}
```

`crt.kage` 源码:
```go
package main

// Uniforms (由 Go 传入)
var Time float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    // 1. 简单的扫描线 (Scanlines)
    // 根据 Y 坐标生成正弦波
    scanline := sin(texCoord.y * 200.0 + Time*5.0) * 0.1
    
    // 2. 采样原色
    srcColor := imageSrc0At(texCoord)
    
    // 3. 简单的色差 (Chromatic Aberration)
    // R 通道稍微偏移
    red := imageSrc0At(texCoord + vec2(0.003, 0.0)).r
    
    return vec4(red, srcColor.g, srcColor.b, 1.0) - vec4(scanline)
}
```

## ✨ 4. 粒子系统 (CPU Particles)
对于 <2000 个粒子，直接在 Go 中计算位置、在 GPU 批量绘制是最简单的。

- **数据结构**: 使用我们之前提到的 `Zero-Alloc Pool`。
- **渲染技巧**: 使用 `DrawImageOptions.ColorScale` 控制透明度衰减。

```go
func (p *Particle) Draw(screen *ebiten.Image) {
    op := &ebiten.DrawImageOptions{}
    op.GeoM.Translate(p.x, p.y)
    
    // 随生命周期淡出
    alpha := float32(p.life) / float32(p.maxLife)
    op.ColorScale.ScaleAlpha(alpha)
    
    // 叠加模式 (让火焰/光效更亮)
    op.Blend = ebiten.BlendLighter
    
    screen.DrawImage(p.img, op)
}
```

## 🌟 5. 辉光特效 (Bloom / Gaussian Blur)
Bloom 本质上是“提取高亮 -> 模糊 -> 叠加”。这里提供一个高效的单Pass高斯模糊 Shader。

```go
// assets/shaders/blur.kage
//go:embed assets/shaders/blur.kage
var blurKage []byte

// Dir: 分别传 (1, 0) 和 (0, 1) 进行两次 Pass 可以获得更好性能
func DrawBloom(screen *ebiten.Image, dirX, dirY float32) {
    op := &ebiten.DrawRectShaderOptions{}
    op.Uniforms = map[string]interface{}{
        "Dir": []float32{dirX, dirY},
    }
    op.Images[0] = screen
    op.Blend = ebiten.BlendLighter // 关键：叠加模式
    screen.DrawRectShader(w, h, blurShader, op)
}
```

`blur.kage` 源码:
```go
package main

var Dir vec2

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    // 简单的 5-tap 高斯模糊
    // 注意：在实际生产中，应根据 texCoord 步长 (1/width) 进行调整
    sum := vec4(0)
    step := Dir * 0.004 // 模糊半径
    
    sum += imageSrc0At(texCoord - step*2.0) * 0.1
    sum += imageSrc0At(texCoord - step)     * 0.25
    sum += imageSrc0At(texCoord)            * 0.3
    sum += imageSrc0At(texCoord + step)     * 0.25
    sum += imageSrc0At(texCoord + step*2.0) * 0.1
    
    return sum * color // 乘上顶点颜色 (如果有)
}
```

## 🌊 6. 冲击波畸变 (Shockwave Distortion)
用于爆炸、高能释放。原理是根据离中心的距离偏移 UV 坐标。

```go
// assets/shaders/shockwave.kage
// Game logic: 只有在 shockwaveActive 时才绘制此层
func DrawShockwave(screen *ebiten.Image, centerX, centerY, time float32) {
    op := &ebiten.DrawRectShaderOptions{}
    op.Uniforms = map[string]interface{}{
        "Center": []float32{centerX, centerY}, // 归一化坐标 (0~1)
        "Time":   time, // 0.0 ~ 1.0 (生命周期)
        "Ratio":  float32(screenHeight) / float32(screenWidth), // 修正宽高比
    }
    op.Images[0] = screen
    screen.DrawRectShader(w, h, waveShader, op)
}
```

`shockwave.kage` 源码:
```go
package main

var Center vec2
var Time float // 0 -> 1
var Ratio float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    // 计算当前像素到波心的距离
    uv := texCoord
    uv.y = uv.y * Ratio // 修正宽高比，画圆而不是椭圆
    centerFixed := Center
    centerFixed.y = centerFixed.y * Ratio
    
    dist := distance(uv, centerFixed)
    
    // 波纹扩散参数
    waveWidth := 0.05
    wavePos := Time * 0.8 // 扩散速度
    
    // 仅在波环范围内计算偏移
    if dist > wavePos && dist < wavePos + waveWidth {
        diff := (dist - wavePos) / waveWidth // 0~1 inside the ring
        
        // 简单的sin波形偏移
        offset := sin(diff * 6.28) * 0.02 * (1.0 - Time) // 随时间衰减
        
        // 向波心偏移
        dir := normalize(texCoord - Center)
        return imageSrc0At(texCoord - dir*offset)
    }
    
    return imageSrc0At(texCoord)
}
```
