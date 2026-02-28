# MPM `task_chain` 工具引擎说明 (V3 纯净版)

`task_chain` 是 MPM 提供的一个基于本机的 SQLite 数据库所构建的 **状态机（State Machine）**。

它的**唯一作用**是：提供一条强制约束任务流程执行先后顺序的轨道，记录多步拆解任务的流转进度。
它**绝对不会**自动创建任何本机的目录或文件，也**绝对不会**自动帮你执行任何代码和命令。它纯粹是一个基于数据库交互的“考勤打卡系统”。

## 1. 核心运行机制

`task_chain` 的所有状态数据保存于本机的 SQLite 数据库（`symbols.db`）中。大模型通过给工具的 `mode` 参数传入不同动作，来改变数据库里的流转状态。

### 1.1 阶段考勤点类型 (Phase Type)
一条完整的任务链（Task Chain）由多个打卡节点（Phase）组成。节点类型严格分为以下 3 种：
- **`execute` (普通执行节点)**：执行完你负责的动作后，调用 `complete` 打卡通过即可前往下一关。
- **`gate` (质量门控节点)**：这是一种审查/防守节点。打卡调用 `complete` 时**必须显式附带** `result="pass"` 或 `result="fail"`。如果是 `pass` 放行去下一个阶段；如果是 `fail`，系统会强制让你退回上一阶段重干（会有最大重试次数限制兜底）。
- **`loop` (循环节点)**：专门用来拆分并行工作的。进入这个节点后，不能直接完成，必须先用 `spawn` 下发具体的微型子任务（SubTasks）。随后通过 `complete_sub` 把子任务一个一个歼灭。当所有微型子任务被消灭干净，系统才会判定该节点达标放行。

### 1.2 内置固定流水线 (Protocol)
如果你在初始化 (`init`) 阶段不手搓路线，可以直接调用系统内设的套件：
- **`linear`** (默认): 极简一条龙。只有一个单线节点 `main`，适合一气呵成的小活。
- **`develop`** (标准开发): `analyze`(普通) -> `plan_gate`(门控) -> `implement`(循环) -> `verify_gate`(门控) -> `finalize`(普通)
- **`debug`** (排查修复): `reproduce`(普通) -> `locate`(普通) -> `fix`(循环) -> `verify_gate`(门控) -> `finalize`(普通)
- **`refactor`** (代码重构): `baseline`(普通) -> `analyze`(普通) -> `refactor`(循环) -> `verify_gate`(门控) -> `finalize`(普通)

---

## 2. API 模式 (Modes) 及使用范式

以最复杂的 `develop` 协议为例，演示一整套无缝连接的 API 调用顺序：

### 步骤 1：初始化拉起流水线 (`init`)
- **场景**：接到大的任务，最开始调用这个进行全链路大盘建档。
- **指令格式**：`task_chain(mode="init", task_id="REQ_102", description="开发登录功能", protocol="develop")`
- **系统反馈**：数据库中生成该任务的 5 个阶段。并且，**第一站阶段 `analyze` 的状态会自动被赋为 `active`（进行中）**。

### 步骤 2：上交本阶段成果 (`complete`)
- **场景**：你看一眼状态大盘，发现当前所处阶段已经是 `active` 了，并且你在本地文件或者系统代码里干完活了，你需要去销账。*(由于上面 init 默认把 analyze 活动化了，你此时直接 complete)*。
- **指令格式**：`task_chain(mode="complete", task_id="REQ_102", phase_id="analyze", summary="我的本地工作做完了。")`
- **系统反馈**：数据库中 `analyze` 节点状态由 `active` 变更为 `passed`。

### 步骤 3：申请开启下个阶段 (`start`)
- **场景**：上一站完成了，下一站 `plan_gate` 依然是灰色锁死的 `pending` 状态。你需要主动敲门去推开它。
- **指令格式**：`task_chain(mode="start", task_id="REQ_102", phase_id="plan_gate")`
- **系统反馈**：`plan_gate` 状态被翻转为 `active`。
*(门打开了，你干活检查后，由于它是 gate 节点，你再次调用 complete 并带上 `result="pass"` 彻底通过它)*。

### 步骤 4：循环节点派发子任务并入场 (`spawn`)
- **场景**：当你经过层层打卡来到了 `loop` 节点（如 `implement`），且处于 `active`。此时无法直接 complete，必须把待办任务挂载上去。
- **指令格式**：`task_chain(mode="spawn", task_id="REQ_102", phase_id="implement", sub_tasks=[{"id":"s1","name":"后端API"},{"id":"s2","name":"前端页面"}])`
- **系统反馈**：`implement` 节点的肚子内生成了 2 个未处理的条目。其中第一条 `s1` 会自动进入并开始跑。

### 步骤 5：击破子任务 (`complete_sub`)
- **场景**：专门给 `loop` 阶段收尸销账使用。
- **指令格式**：`task_chain(mode="complete_sub", task_id="REQ_102", phase_id="implement", sub_id="s1", summary="全部代码修改完毕测试通过。")`
- **系统反馈**：自动标记 `s1` 销账完成，并顺接激活下一个目标 `s2`。一旦 `s2` 也被你 `complete_sub`，系统触发联锁反应：`implement` 大关卡自动判为 `passed` 过关。

### 步骤 6：全链路清账结算 (`finish`)
- **场景**：流水线上的每一个 phase 全部呈 `passed` 状态，走向结项大一统。
- **指令格式**：`task_chain(mode="finish", task_id="REQ_102")`
- **系统反馈**：这根记录链条被标记为 `finished` 深埋结项。

---

## 3. 辅助功能 API

除了在轨道上推进，你随时可以使用下面两种 `mode` 观摩你的大盘状况：

* **查看任务最新底牌 (`status` 或 `resume`)**:
  * 传入：`task_chain(mode="status", task_id="REQ_102")`
  * 返回：实时输出整个大盘目前的 JSON 数据全貌（当前所在的是第几关？是正在打卡，还是等你去主动 `start` 激活？）。你一旦对流程迷茫，甚至在恢复上下文记忆时，第一时间调用此功能。
* **查看内置出厂协议说明 (`protocol`)**:
  * 传入：`task_chain(mode="protocol")`
  * 返回：打印所有的默认任务轨道流程。

---

## 4. 关键：关于 `summary` 填写的“自然收敛”法则

在使用 `complete` 和 `complete_sub` 模式销账时，你会需要填写一个 `summary` 参数。
很多时候系统设计者会陷入一种误区：要么想把所有的结果（如千字设计档、大量报错）硬塞进 `summary` 里当成接力棒传下去；要么走向另一个极端，要求强制使用物理文件指针来传递一切信息。

**其实，请遵循大模型最“自然”的输出习惯：**

1. **轻量信息直接写**
   如果一个阶段（Phase）的结论本身就很简短，只是一两句决策（例如：“检查过了，没发现冲突” 或 “API 的 base_url 应该设为 /v2/api”），**请直接写在 `summary` 里。** 下一个阶段的模型能通过 Context 无缝读取，丝滑且高效，没必要脱裤子放屁去建个专门的文件。
   
2. **重型产物顺其自然**
   如果阶段任务是“设计数据库结构”或者“拟定前后端接口”，大模型在执行过程中，**天然就会去创建并编写真实的源码或 Markdown 文件（如编写 `schema.sql` 或 `docs/api_spec.md`）**。这是项目的自然产物。
   此时，`task_chain` 的 `summary` 只需要扮演一个**极其自然的进度汇报**：
   > `summary="订单系统数据库设计已完成，建表语句已写入 docs/schema.sql。"`
   
   **绝对禁止**尝试把 `schema.sql` 里的几百行建表语句塞进 `summary`！因为未来的节点去调 `status` 时，会被这个包含了无数行生硬代码的巨型 JSON 彻底淹没，从而产生 Token 超限与死循环幻觉。

简而言之：`task_chain` 就是一个打卡机。干轻活，直接说一两句结论；干重活，把产出物写进正常的工作代码/文档里，打卡时只汇报产出物的名字即可。
