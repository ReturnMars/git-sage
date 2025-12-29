# GitSage 重构设计文档 (Clean Architecture Edition)

本文档描述了 GitSage 重构后的系统架构设计。本次重构采用 **Clean Architecture（整洁架构）** 原则，旨在实现业务逻辑与基础设施的完全解耦，提高系统的可测试性、可维护性和扩展性。

## 1. 架构概览 (Architecture Overview)

系统严格遵循依赖规则：**源码依赖方向只能指向内部（高层策略），不得指向外部（底层实现）。**

### 1.1 分层架构图

```text
┌─────────────────────────────────────────────────────────────┐
│                       Presentation Layer                    │
│                 (cmd/gitsage, CLI Entrypoint)               │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                      Application Layer                      │
│            (internal/app - Use Cases/Workflows)             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │                   CommitWorkflow                    │   │
│   └──────────────────────────┬──────────────────────────┘   │
└──────────────────────────────┼──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                       Core Domain Layer                     │
│               (internal/core - Pure Business)               │
│                                                             │
│   ┌──────────────┐   ┌──────────────┐   ┌───────────────┐   │
│   │   Domain     │   │   Services   │   │     Ports     │   │
│   │  (Entities)  │◀──┤ (Logic/Algo) │◀──┤  (Interfaces) │   │
│   └──────────────┘   └──────────────┘   └───────────────┘   │
└──────────────────────────────▲──────────────────────────────┘
                               │
                               │ (Implements)
┌──────────────────────────────┴──────────────────────────────┐
│                     Infrastructure Layer                    │
│             (internal/infra - Concrete Adapters)            │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐  ┌──────────┐ │
│  │ GitLocal │  │ langchain-go │  │ BubbleTea│  │ FSStore  │ │
│  │ Adapter  │  │   Adapter    │  │ Adapter  │  │ Adapter  │ │
│  └──────────┘  └──────────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 2. 核心层 (Core Domain Layer)

**路径**: `internal/core`
**职责**: 包含企业级业务规则（Entities）和应用级独立业务逻辑（Services）。**零依赖**（除标准库外）。

### 2.1 领域实体 (Domain Entities)

定义核心数据结构，不包含任何 JSON tag 或 ORM 标记。

*   **`Diff`**: 表示代码变更。
    ```go
    type Diff struct {
        Files []DiffFile
        Stats DiffStats
    }
    ```
*   **`DiffFile`**: 单个文件的变更详情（原 `DiffChunk`）。
*   **`CommitMessage`**: 包含 Subject, Body, Footer 的结构化对象。
*   **`CommitType`**: 枚举类型 (Feat, Fix, etc.)。

### 2.2 端口接口 (Ports)

定义核心层与外部世界交互的**契约**。

*   **`GitProvider`**: 
    ```go
    type GitProvider interface {
        GetStagedChanges(ctx context.Context) (*domain.Diff, error)
        Commit(ctx context.Context, msg *domain.CommitMessage) error
        // ...
    }
    ```
*   **`AIModel`**: 
    ```go
    type AIModel interface {
        GenerateCommitMessage(ctx context.Context, diff *domain.Diff, hint string) (*domain.CommitMessage, error)
    }
    ```
*   **`UserInterface`**: 
    ```go
    type UserInterface interface {
        ShowProgress(stage string)
        ReviewMessage(msg *domain.CommitMessage) (UserAction, *domain.CommitMessage, error)
        ShowError(err error)
    }
    ```

### 2.3 领域服务 (Domain Services)

封装不属于单一实体的复杂业务逻辑。

*   **`DiffProcessor`**: 
    *   负责 Diff 的清洗（过滤 Lock 文件）。
    *   负责 Diff 的分块策略（Chunking Strategy）计算。
*   **`PromptBuilder`**:
    *   负责将 Diff 数据组装成 LLM 可理解的 Prompt。
*   **`MessageValidator`**:
    *   执行 Conventional Commits 规范检查。
*   **`RetryStrategy` (新增)**:
    *   封装重试算法（如指数退避计算）。
    *   **方法**: `CalculateBackoff(attempt int) time.Duration`
    *   **目的**: 将“等待多久”的**策略**与“执行等待”的**机制**（Infra）分离，便于测试。

## 3. 应用层 (Application Layer)

**路径**: `internal/app`
**职责**: 编排具体的用例（Use Cases）。

### 3.1 提交工作流 (CommitWorkflow)

`CommitService` 重命名为 `Workflow`，作为 Orchestrator。

**流程逻辑**:
1.  调用 `GitProvider` 获取 Staged Diff。
2.  调用 `DiffProcessor` 服务处理 Diff（过滤、分块）。
3.  **策略分支**:
    *   如果 Diff 较小: 直接调用 `AIModel` 生成。
    *   如果 Diff 较大: 调用 `AIModel` 对每个块生成摘要，再合并生成最终消息（逻辑由 `GeneratorService` 封装）。
4.  调用 `MessageValidator` 验证生成结果。
5.  调用 `UserInterface` 展示并进入交互循环（Edit/Regenerate）。
6.  用户确认后，调用 `GitProvider` 执行提交。
7.  调用 `HistoryStore` 保存记录。

## 4. 基础设施层 (Infrastructure Layer)

**路径**: `internal/infra`
**职责**: 实现 Core 层定义的 Port 接口。

### 4.1 Git 适配器 (`infra/git`)
*   封装 `os/exec` 调用 git 命令。
*   负责将 `git diff` 原生输出解析转换为 `domain.Diff` 对象。

### 4.2 AI 适配器 (`infra/ai`)
*   **核心实现**: 基于 `langchain-go` 库。
*   **重试逻辑改进**: 
    *   AI Adapter 内部持有 `core.services.RetryStrategy` 实例（可通过配置注入）。
    *   在执行网络请求失败时，调用 `RetryStrategy.CalculateBackoff()` 获取等待时间。
    *   使用 `time.Sleep` 执行等待（Infrastructure 行为）。

### 4.3 UI 适配器 (`infra/ui`)
*   封装 `charmbracelet/bubbletea`。
*   实现 `UserInterface` 接口。
*   将业务上的 "ShowProgress" 翻译为具体的 Spinner 动画。

## 5. 关键重构点对比

| 功能模块 | 原架构位置 | 新架构位置 | 优势 |
| :--- | :--- | :--- | :--- |
| **Commit Msg 解析** | `pkg/ai/parser.go` | `core/domain/parser.go` | 业务规则与 AI 供应商解耦 |
| **Git Diff 结构体** | `pkg/git/client.go` | `core/domain/diff.go` | 核心对象标准化，所有层通用 |
| **Lock文件过滤** | `pkg/processor` | `core/services/diff_filter` | 纯算法逻辑，易于单元测试 |
| **UI Spinner** | `app/service.go` 直接调用 | `infra/ui` 实现接口 | 业务逻辑不再依赖具体 UI 库 |
| **生成策略(分块)** | `app/service.go` 私有方法 | `core/services/generator` | 策略可独立测试和扩展 |
| **AI 调用** | `pkg/ai` 混合 | `infra/ai` (Wraps LangChain) | 明确 Infra 层的外部依赖 |
| **重试/退避计算** | `pkg/ai/openai.go` | `core/services/retry` | 算法可测试，不依赖真实时间/网络 |

## 6. 数据流向示例 (Data Flow)

**场景**: 用户执行 `gitsage commit`

1.  **CLI**: 解析参数，初始化 `Infra` 组件，注入 `App` Workflow。
2.  **App**: `workflow.Run()`
3.  **App -> Core -> Infra(Git)**: `GetStagedChanges()` 返回 `domain.Diff`。
4.  **App -> Core(Service)**: `processor.Process(diff)` 返回处理后的 Diff。
5.  **App -> Core -> Infra(AI)**: `Generate(diff)`。
    *   *Infra(AI)* 内部将 `domain.Diff` 序列化为 Prompt 字符串，调用 `langchain-go` API，返回 Result。
6.  **App -> Core(Service)**: `validator.Validate(msg)` 确保格式正确。
7.  **App -> Core -> Infra(UI)**: `Review(msg)` 展示 TUI。
8.  **App -> Core -> Infra(Git)**: `Commit(msg)`。

## 7. 扩展性设计

*   **新增 AI Provider**: 只需在 `infra/ai` 下利用 `langchain-go` 的 Provider 接口扩展，或直接替换 `AIModel` 的实现。
*   **更换 UI**: 如果想从 TUI 换成 GUI 或 Web，只需重写 `infra/ui` 实现 `UserInterface` 接口。
*   **新增语言支持**: 修改 `core/services/diff_filter` 中的过滤规则，无需动 Git 命令执行逻辑。
