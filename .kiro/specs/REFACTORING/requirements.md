# GitSage 2.0 重构需求文档 (Refactoring Requirements)

本文档基于 `gitsage` 原有需求、LangChain 集成需求以及 Clean Architecture 重构目标制定。它是 GitSage 2.0 (Clean Architecture Edition) 的顶层规范。

## 1. 业务目标 (Business Goals)

1.  **架构解耦**: 通过实施 Clean Architecture，将业务逻辑与基础设施代码完全分离，消除“God Class”和循环依赖，提高可维护性。
2.  **技术债务偿还**: 统一 AI Provider 实现（基于 LangChain），消除重复的 HTTP 调用和重试代码。
3.  **性能与可靠性**: 引入明确的性能指标（并发处理、缓存）和安全规范（API 密钥管理），提升企业级可用性。
4.  **接口兼容性**: 保持 CLI 接口和配置文件格式不变，对终端用户透明升级。

## 2. 功能需求 (Functional Requirements)

### 2.1 核心业务逻辑 (Core Domain)

*   **Diff 处理**:
    *   **REQ-CORE-001**: 系统必须能从 Git 获取 Staged Changes，并转换为标准的 `domain.Diff` 对象。
    *   **REQ-CORE-002**: 系统必须支持过滤 Lock 文件（如 `package-lock.json`），过滤规则应在业务层定义。
    *   **REQ-CORE-003**: 系统必须支持大 Diff 分块处理策略（Two-Phase Generation），当 Diff 大小超过阈值（默认 10KB）时触发。

*   **Commit Message 生成**:
    *   **REQ-CORE-004**: 系统必须能根据 Diff 生成符合 Conventional Commits 规范的消息。
    *   **REQ-CORE-005**: 消息生成逻辑应独立于具体的 AI Provider（OpenAI/DeepSeek/Ollama）。
    *   **REQ-CORE-006**: 系统必须支持用户自定义 Prompt 模板。

*   **消息校验**:
    *   **REQ-CORE-007**: 系统必须校验生成的 Commit Message 格式（Subject < 50字，包含 Type/Scope）。
    *   **REQ-CORE-008**: 校验不通过时应提供警告或自动修复建议，而非直接报错。

### 2.2 基础设施支持 (Infrastructure)

*   **AI 集成 (LangChain)**:
    *   **REQ-INFRA-001**: 所有 LLM 调用必须通过 `langchain-go` 统一实现。
    *   **REQ-INFRA-002**: 支持 OpenAI, DeepSeek, Ollama 三种 Provider。
    *   **REQ-INFRA-003**: 提供统一的重试机制，支持指数退避，最大重试 3 次。
    *   **REQ-INFRA-004**: 处理 API 限流 (Rate Limit) 和 超时 (Timeout)。

*   **用户界面 (TUI)**:
    *   **REQ-INFRA-005**: 使用 BubbleTea 实现交互式 Review 界面。
    *   **REQ-INFRA-006**: 支持 Edit（编辑）、Regenerate（重生成）、Commit（提交）操作。
    *   **REQ-INFRA-007**: 显示实时进度（特别是多块 Diff 处理时）。

### 2.3 性能需求 (Performance)

*   **REQ-PERF-001**: 小型 Diff (<10KB) 的生成延迟不超过 5秒（不含网络延迟）。
*   **REQ-PERF-002**: 支持并发处理多个 Diff Chunks（最多 3 并发）。
*   **REQ-PERF-003**: 实现 LRU 缓存（最近 100 条），避免对相同 Diff 重复调用 AI。
*   **REQ-PERF-004**: Git 命令执行超时设置为 10秒。

### 2.4 安全需求 (Security)

*   **REQ-SEC-001**: 配置文件权限必须为 0600。
*   **REQ-SEC-002**: 日志中严禁明文记录 API Key，必须脱敏（只显示后4位）。
*   **REQ-SEC-003**: 首次运行时警告 Diff 代码将上传第三方服务器（除非使用 Ollama）。

## 3. 非功能需求 (Non-Functional Requirements)

*   **NFR-001 (架构原则)**: 核心层 (`internal/core`) 不得依赖 `internal/infra` 或第三方库（除了 `langchain-go` 封装在适配器内）。
*   **NFR-002 (测试覆盖率)**: `internal/core` 层单元测试覆盖率需达到 80% 以上。
*   **NFR-003 (代码规范)**: 遵循 Uber Go Style Guide，所有公共接口需有注释。

## 4. 迁移与兼容性 (Migration)

*   **MIG-001**: 现有的 `~/.gitsage/config.yaml` 必须完全兼容。
*   **MIG-002**: 所有的 CLI Flag（如 `--provider`, `--model`）行为保持不变。
