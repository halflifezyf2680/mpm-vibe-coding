---
name: performance-analysis
description: A strict prohibition list of common performance anti-patterns (The Penal Code).
---

# Performance Penal Code (性能刑法典)

本文档包含**授权**与**禁令**两部分。

## 🟢 Mandate (核心授权)

**You are the Expert.**
我们信任你的编程直觉和通用优化能力。对于常规的代码优化（算法改进、数据结构选型、并发模型设计），请 **Boldly use your best judgment**。
本文档仅用于标记那些**容易被忽视的隐形陷阱**。只要不触犯以下禁令，你可以自由选择最优解。

## 🔴 Class A Felonies (一级重罪 - 必须立即修复)

### 1. Loop-Invariant String Concatenation
**Pattern**: 在循环中使用 `+=` 拼接字符串。
**Why**: 字符串在多种语言中是不可变的。拼接导致 O(n²) 复杂度及大量内存分配。
**Strict Ban**:
- ❌ `s = ""; for x in items: s += x`
- ✅ `"".join(items)` (Python) / `StringBuilder` (Java/C#) / `strings.Builder` (Go)

### 2. Linear Search in Hot Path
**Pattern**: 在热点循环中对 List/Array 进行成员检查 (`x in list_obj`)。
**Why**: 每次迭代 O(n)，总复杂度 O(n*m)。
**Strict Ban**:
- ❌ `if x in heavy_list:` (inside loop)
- ✅ `heavy_set = set(heavy_list); if x in heavy_set:`

### 3. Loop-Invariant Computation
**Pattern**: 在循环内计算不依赖于迭代变量的值。
**Why**: 重复执行无用功。
**Strict Ban**:
- ❌ `for x in data: threshold = get_config() * 0.8; if x > threshold:...`
- ✅ `threshold = get_config() * 0.8; for x in data:...`

---

## 🟠 Class B Misdemeanors (二级轻罪 - 强烈建议优化)

### 4. Global/Dotted Lookup in Tight Loops (Python Specific)
**Pattern**: 在密集循环中重复访问全局变量或多层属性 (e.g., `os.path.exists`)。
**Why**: 每次迭代触发多次 hashtable lookup。
**Optimization**:
- ❌ `for x in massive_list: os.path.exists(x)`
- ✅ `exists = os.path.exists; for x in massive_list: exists(x)` (Local var is faster)

### 5. Try-Except in Tight Loops
**Pattern**: 仅仅为了控制流而在热循环内使用 `try-except`。
**Why**: 在部分解释型语言中破坏流水线优化，增加栈帧开销。
**Optimization**:
- ❌ `for x in items: try: ...`
- ✅ 将 try 移至循环外，或改用 `if` 预检查 (Look-Before-You-Leap)。

### 6. Memory Suicide (Eager Loading)
**Pattern**: 对大文件/数据库结果集使用 `readlines()` / `fetchall()`。
**Why**: 瞬间内存峰值，可能导致 OOM。
**Optimization**:
- ❌ `for line in f.readlines():`
- ✅ `for line in f:` (Lazy Iterator)

---

## 🔍 Detection Strategy (自查指令)

Agent 执行代码审查时，请优先使用以下正则探测“犯罪现场”：

1.  **Suspicious String Concat**:
    `grep_search(query=r"\+= +", is_regex=True, includes=["*.py", "*.js", "*.go"])`
    *(需人工复核是否在循环体内)*

2.  **Suspicious Lookups (Python)**:
    `grep_search(query=r" in ", is_regex=False)`
    *(重点检查右侧变量类型)*

3.  **IO inside Loop**:
    `find_code(mode='map')` 查看循环结构，确认是否有 IO 调用 (DB/File/Network) 在循环体内 (N+1 Problem)。
