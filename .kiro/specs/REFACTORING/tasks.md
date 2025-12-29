# GitSage 重构任务清单 (Refactoring Tasks)

本文档基于 `design.md` 和 `plan.md` 制定，作为代码重构的执行指南。请严格按顺序执行，以确保构建过程的稳定性。

## Phase 1: 核心层建设 (Core Layer Foundation)

此阶段目标是建立不依赖任何外部实现的核心业务逻辑。

- [x] **Task 1: 建立核心目录与领域实体**
  - 创建 `internal/core/domain` 目录。
  - 定义 `Diff` 和 `DiffFile` 结构体 (从原 `git.DiffChunk` 迁移并净化)。
  - 定义 `CommitMessage` 结构体 (包含 Subject, Body, Footer)。
  - 定义 `CommitType` 枚举与常量。
  - 定义 `ValidationResult` 等辅助结构。

- [x] **Task 2: 定义端口接口 (Ports)**
  - 创建 `internal/core/ports` 目录。
  - 定义 `GitProvider` 接口 (GetStaged, Commit, etc.)。
  - 定义 `AIModel` 接口 (GenerateCommitMessage)。
  - 定义 `UserInterface` 接口 (ShowProgress, Review, Error)。
  - 定义 `ConfigStore` 接口 (如果配置逻辑需要抽象)。

- [x] **Task 3: 实现核心服务 - 基础部分**
  - 创建 `internal/core/services` 目录。
  - **3.1 DiffProcessor**: 实现 Lock 文件过滤 (`filterLockFiles`) 和 Diff 大小计算逻辑。
  - **3.2 MessageValidator**: 迁移 Conventional Commits 校验逻辑。
  - **3.3 RetryStrategy**: 实现指数退避算法 (`CalculateBackoff`)。
  - **3.4 PromptBuilder**: 实现将 `Diff` 对象转换为 String Prompt 的逻辑。
  - **3.5 CacheService**: 实现 LRU 缓存策略 (REQ-PERF-003)，定义 Cache 接口与内存实现 (TTL, Capacity)。

- [x] **Task 4: 实现核心服务 - 生成策略 (Generator)**
  - 创建 `internal/core/services/generator`。
  - 实现 `SmartGenerator` 服务：
    - 接收 `ports.AIModel` 作为依赖。
    - 实现分块策略逻辑 (Two-Phase Generation)。
    - **实现并发控制**: 使用 Semaphore 或 Worker Pool 限制并发请求数为 3 (REQ-PERF-002)。
    - 决定是直接调用 AI 还是分步摘要后再合并。

## Phase 2: 基础设施适配 (Infrastructure Adapters)

此阶段目标是实现 Core 层定义的接口，适配现有的底层代码。

- [x] **Task 5: 实现 Git 适配器**
  - 创建 `internal/infra/git`。
  - 实现 `ports.GitProvider` 接口。
  - 迁移并重构原 `internal/pkg/git/client.go` 的逻辑。
  - 核心工作：将 `os/exec` 的原始输出映射为 `domain.Diff` 实体。

- [x] **Task 6: 实现 AI 适配器与日志**
  - 创建 `internal/infra/ai`。
  - 实现 `ports.AIModel` 接口。
  - 集成 `langchain-go`。
  - 注入 `core.services.RetryStrategy` 处理重试等待 (已在 SmartGenerator 实现)。
  - **日志安全**: 封装现有日志库，实现 API Key 脱敏逻辑 (REQ-SEC-002)，确保 Infra 层所有输出安全。

- [x] **Task 7: 实现 UI 适配器**
  - 创建 `internal/infra/ui`。
  - 实现 `ports.UserInterface` 接口。
  - 核心工作：使用 Bubbletea 实现 ReviewMessage 和简单的 Spinner。

## Phase 3: 应用层编排 (Application Layer)

此阶段目标是组装各部件，替代原有的 God Class。

- [x] **Task 8: 实现提交工作流 (CommitWorkflow)**
  - 创建 `internal/app/workflow`。
  - 定义 `CommitWorkflow` 结构体，注入所有 Ports 和 Services。
  - 实现 `Run(ctx)` 方法，处理 Accept, Edit, Regenerate 循环。

## Phase 4: 集成与清理 (Integration & Cleanup)

- [x] **Task 9: CLI 入口集成**
  - 修改 `cmd/gitsage/main.go` 和 `cmd` 包。
  - 在 `main` 函数中实例化所有 Adapters (Infra)。
  - 实例化 Core Services。
  - 注入到 `CommitWorkflow`。
  - 执行 Workflow。

- [x] **Task 10: 清理旧代码**
  - **前置检查**: 仔细扫描所有待删除的包（特别是 `pkg/ai` 和 `pkg/git` 中的工具函数），确保所有有用的辅助逻辑都已迁移至新的 `core` 或 `infra` 层。
  - 删除 `internal/pkg/ai` (旧)。
  - 删除 `internal/pkg/git` (旧)。
  - 删除 `internal/pkg/ui` (旧)。
  - 删除 `internal/pkg/processor` (旧)。
  - 删除 `internal/app/service.go` (旧)。

- [ ] **Task 12: 性能优化 (In Progress)**
  - [x] 实现 Hunk-Level Chunking：当单个文件 Diff 过大时，按 Git Hunk 进一步拆分，防止 Token 溢出。
  - [x] 优化 Prompt 压缩策略。
  - [x] **Diff 匹配算法优化**: 优化 `parseNumstat` 与 `diff` 输出的匹配算法，从简单的字符串包含升级为精确路径匹配。
- [x] **Task 13: Cache 集成**
  - 在 `SmartGenerator` 中注入 `CacheService`。
  - 在调用 AI 生成前检查 Cache。
  - 生成成功后写入 Cache。
  - 缓存 Key 生成逻辑 (Diff Hash + Prompt Hash)。

- [x] **Task 14: AI 响应解析增强**
  - 增强 `SmartGenerator` 的解析逻辑，支持从 Markdown 代码块中提取 commit message。
  - 集成 `MessageValidator` 到解析流程（可选，或者在 Workflow 中做）。
  - 处理 AI 前缀（如 "Here is the commit message:"）。

- [ ] **Task 15: Config 迁移**
  - 创建 `internal/infra/config` 包。
  - 将 `internal/pkg/config` 逻辑迁移过去。
  - 更新 `cmd` 层引用。
  - 删除 `internal/pkg/config`。

- [ ] **Task 16: PathCheck 重构**
  - 将 `internal/pkg/pathcheck` 重构为 `core/services/pathcheck` 或 `infra/pathcheck`。
  - 使用 `ports.UserInterface` 替代直接 I/O。
  - 更新 `root.go`。
  - 删除 `internal/pkg/pathcheck`。

- [x] **Task 11: 全局测试与验证**
  - 运行所有单元测试。
  - 执行 `go build` 确保无编译错误。
  - 进行手动 Dry Run 测试验证功能一致性。
