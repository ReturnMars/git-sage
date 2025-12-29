# 全局重构计划 - Clean Architecture 实施手册

本文档旨在规划 `gitsage` 项目的全局重构，确保代码库遵循 **Clean Architecture**、**单一职责原则 (SRP)** 和 **高内聚低耦合** 的设计规范。

## 1. 现状分析与问题诊断

(基于对 `cmd`, `internal/app`, `internal/pkg` 的代码审查)

### 1.1 核心问题
*   **依赖倒置原则 (DIP) 违例**: 核心业务对象（如 `DiffChunk`, `ChangeType`）定义在 `internal/pkg/git` (Infrastructure Layer) 中，导致 Use Case 层（`internal/app`）和甚至其他 Infra 层（`processor`）都依赖于 Git 的具体实现包。
*   **领域逻辑泄露**: 
    *   Commit Message 的解析与验证逻辑 (`parser.go`) 位于 `internal/pkg/ai` 中，但这是核心业务规则，不应与 AI Provider 绑定。
    *   Diff 分块策略 (`processor.go`) 位于 Infra 层，但它实际上是核心业务策略。
*   **App Service 臃肿 (God Object)**: `CommitService` 混合了工作流编排、UI 状态管理、文件处理算法和缓存逻辑。
*   **Infrastructure 混合**: `internal/pkg` 下的包（如 `git`, `ai`）同时包含了“数据结构定义”和“具体实现”，难以解耦。

## 2. 架构愿景 (Refactored Architecture)

我们将采用标准的 Clean Architecture 分层：

```text
gitsage/
├── cmd/                # 仅负责 CLI 入口与参数解析
├── internal/
│   ├── core/           # [新建] 核心业务层 (不依赖任何外部库)
│   │   ├── domain/     # 实体 (Entities): Diff, CommitMessage, ChangeType
│   │   ├── ports/      # 接口 (Interfaces): GitPort, AIPort, UIPort
│   │   └── services/   # 领域服务: DiffSplitter, CommitValidator
│   ├── app/            # 应用层 (Use Cases)
│   │   └── workflow/   # 编排服务: CommitWorkflow (原 CommitService)
│   └── infra/          # [重组] 基础设施层 (实现 ports)
│       ├── git/        # 实现 GitPort
│       ├── ai/         # 实现 AIPort
│       ├── ui/         # 实现 UIPort
│       └── store/      # 历史记录与缓存
```

## 3. 全局重构详细实施方案

### 3.1 第一阶段：建立 Core 层 (解耦核心依赖)

**目标**: 将散落在各处的领域对象收敛到 `internal/core/domain`，切断 App 对 Infra 的直接类型依赖。

1.  **定义 Domain Objects**:
    *   移动 `DiffChunk`, `DiffStats`, `ChangeType` 从 `internal/pkg/git` 到 `internal/core/domain`。
    *   移动 `CommitMessage`, `ValidationResult` 相关逻辑到 `internal/core/domain`。
2.  **定义 Ports (抽象接口)**:
    *   在 `internal/core/ports` 中定义 `GitProvider`, `AIModel`, `UserInterface` 接口。
    *   Use Case 层只能引用 `core/ports` 和 `core/domain`。

### 3.2 第二阶段：重构 Infrastructure 层

**目标**: 让 Infra 层依赖 Core 层，实现接口。

1.  **Git Infra**:
    *   修改 `internal/pkg/git`，使其引用 `core/domain` 并实现 `core/ports.GitProvider`。
    *   移除包内定义的 `DiffChunk`，改用 Domain 对象。
2.  **AI Infra**:
    *   将 `parser.go` 中的 Conventional Commits 验证逻辑剥离，移动到 `internal/core/services/validator`。
    *   AI Provider 只负责 "Send Prompt -> Receive Text"，不负责业务校验。
3.  **UI Infra**:
    *   确保 UI 实现（Bubble Tea）不泄露到 Core/App 层。

### 3.3 第三阶段：重构 App 层 (精简 Orchestrator)

**目标**: `CommitService` 只做流程控制。

1.  **引入 Generator Strategy**:
    *   实现前文提到的 `Generator` 模式，将分块策略逻辑移动到 `internal/core/services/generator`。
2.  **重写 Workflow**:
    *   `CommitWorkflow` 只负责：`Git.GetDiff` -> `Generator.Generate` -> `UI.Review` -> `Git.Commit`。

## 4. 模块重构细则

### 4.1 AI 模块 (`internal/pkg/ai`)
*   **当前**: 包含 Prompt 模板构建、HTTP 请求、结果解析、格式验证。
*   **重构后**:
    *   **Prompt 构建**: 移至 `core/services/prompt_builder`。
    *   **格式验证**: 移至 `core/domain/commit_message.go` (Validator)。
    *   **AI Package**: 仅保留“调用 LLM API”的纯管道功能。

### 4.2 Git 模块 (`internal/pkg/git`)
*   **当前**: 定义了 Diff 数据结构，执行 Command 并解析。
*   **重构后**:
    *   **数据结构**: 引用 `core/domain`。
    *   **职责**: 仅负责与 git 二进制文件交互，返回标准 Domain 对象。

### 4.3 Processor 模块
*   **当前**: `internal/pkg/processor` 负责过滤和分块。
*   **重构后**: 这是一个纯粹的领域服务，应移至 `internal/core/services/diff_processor`。它不应该依赖外部 IO，只处理内存中的 `Diff` 对象。

## 5. 实施路线图 (Roadmap)

1.  **基础建设 (Core Setup)**:
    *   创建 `internal/core/{domain,ports,services}` 目录。
    *   迁移核心结构体 (`Diff`, `CommitMessage`)。
2.  **Infra 适配 (Adapter Pattern)**:
    *   修改 `pkg/git` 和 `pkg/ai` 适配新的 Domain 对象。
3.  **业务逻辑迁移 (Extract Logic)**:
    *   从 `CommitService` 抽取 `processLargeFiles` 等逻辑到 Core Service。
4.  **应用层重组 (App Refactor)**:
    *   重写 `CommitService`，注入 Ports 接口。

## 6. 成功标准 (Definition of Done)

*   `internal/core` 包不依赖任何 `internal/infra` 或第三方 IO 库（除了标准库）。
*   `internal/app` 只依赖 `internal/core`。
*   `internal/infra` 依赖 `internal/core` (实现接口)。
*   所有的业务规则（如“Diff 大于 10KB 需分块”、“Subject 不能超过 50 字”）都有独立的单元测试，且不依赖 Mock Git/AI。
