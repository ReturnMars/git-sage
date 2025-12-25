# Implementation Plan: LangChain Go AI Layer Refactor

## Overview

本任务清单将 GitSage 的 AI 层重构为基于 LangChain Go 的实现。采用增量式开发，每个任务都可独立验证。

## Tasks

- [x] 1. 添加 LangChain Go 依赖并创建基础包装器
  - [x] 1.1 更新 go.mod 添加 langchaingo 依赖
    - 执行 `go get github.com/tmc/langchaingo`
    - 验证依赖安装成功
    - _Requirements: 2.1_

  - [x] 1.2 创建 LangChainWrapper 结构和基础方法
    - 创建 `internal/pkg/ai/langchain_wrapper.go`
    - 实现 `LangChainWrapper` 结构体
    - 实现 `NewLangChainWrapper` 构造函数
    - 实现 `generate` 基础调用方法
    - _Requirements: 2.1, 7.2_

  - [x] 1.3 实现带重试的生成方法
    - 实现 `GenerateWithRetry` 方法
    - 复用现有的 `calculateBackoff` 函数
    - 保持 MaxRetries = 3
    - _Requirements: 4.1, 4.2_

- [ ] 2. 实现 LangChain Prompt 模板
  - [ ] 2.1 创建 LangChain Prompt 模板结构
    - 创建 `internal/pkg/ai/langchain_prompt.go`
    - 实现 `LangChainPromptTemplate` 结构体
    - 实现 `NewLangChainPromptTemplate` 使用默认 prompt
    - 实现 `NewLangChainPromptTemplateWithCustom` 支持自定义 prompt
    - _Requirements: 3.1, 3.2, 3.3_

  - [ ] 2.2 实现消息渲染方法
    - 实现 `RenderMessages` 方法
    - 将 PromptData 转换为 `[]llms.MessageContent`
    - 支持 system 和 user 消息
    - _Requirements: 3.4, 3.5_

  - [ ]* 2.3 编写 Prompt 模板属性测试
    - **Property 2: Prompt Rendering Equivalence**
    - 验证 LangChain 模板渲染与原实现等价
    - **Validates: Requirements 3.4, 3.5**

- [ ] 3. 重构 OpenAI Provider
  - [ ] 3.1 使用 LangChain 重写 OpenAI Provider
    - 修改 `internal/pkg/ai/openai.go`
    - 使用 `langchaingo/llms/openai` 创建 LLM
    - 使用 `LangChainWrapper` 封装调用
    - 保持 `SetPromptTemplate` 方法兼容
    - _Requirements: 2.2, 1.1, 1.4_

  - [ ] 3.2 更新 OpenAI Provider 测试
    - 更新 `internal/pkg/ai/openai_test.go`
    - 验证接口兼容性
    - _Requirements: 1.1, 1.4_

- [ ] 4. 重构 Ollama Provider
  - [ ] 4.1 使用 LangChain 重写 Ollama Provider
    - 修改 `internal/pkg/ai/ollama.go`
    - 使用 `langchaingo/llms/ollama` 创建 LLM
    - 使用 `LangChainWrapper` 封装调用
    - 保持默认 endpoint 为 `http://localhost:11434`
    - _Requirements: 2.3, 1.1, 1.4_

  - [ ] 4.2 更新 Ollama Provider 测试
    - 更新 `internal/pkg/ai/ollama_test.go`
    - 验证接口兼容性
    - _Requirements: 1.1, 1.4_

- [ ] 5. 重构 DeepSeek Provider
  - [ ] 5.1 使用 LangChain 重写 DeepSeek Provider
    - 修改 `internal/pkg/ai/deepseek.go`
    - 使用 `langchaingo/llms/openai` 配置 DeepSeek endpoint
    - 使用 `LangChainWrapper` 封装调用
    - 保持默认 endpoint 为 `https://api.deepseek.com/v1`
    - _Requirements: 2.4, 1.1, 1.4_

  - [ ] 5.2 更新 DeepSeek Provider 测试
    - 更新 `internal/pkg/ai/deepseek_test.go`
    - 验证接口兼容性
    - _Requirements: 1.1, 1.4_

- [ ] 6. 实现统一错误处理
  - [ ] 6.1 实现 LangChain 错误包装
    - 在 `langchain_wrapper.go` 中实现 `wrapError` 方法
    - 映射认证错误 (401)
    - 映射速率限制错误 (429)
    - 映射超时错误
    - 映射连接错误 (Ollama 特有)
    - _Requirements: 4.3, 4.4, 4.5_

  - [ ]* 6.2 编写错误处理属性测试
    - **Property 4: Error Wrapping Consistency**
    - 验证错误包装产生用户友好消息
    - **Validates: Requirements 4.3**

- [ ] 7. Checkpoint - 验证核心功能
  - 运行所有现有测试确保通过
  - 验证 `go build` 成功
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. 更新工厂函数和清理代码
  - [ ] 8.1 更新 Factory 函数
    - 修改 `internal/pkg/ai/factory.go`
    - 确保 `NewProvider` 使用新的 provider 实现
    - 确保 `NewProviderWithCustomPrompt` 正常工作
    - _Requirements: 5.1, 5.2, 5.3_

  - [ ]* 8.2 编写工厂函数属性测试
    - **Property 3: Provider Factory Correctness**
    - 验证工厂为各 provider 创建正确实例
    - **Validates: Requirements 5.2, 5.4, 5.5**

  - [ ] 8.3 清理废弃代码
    - 移除 `sashabaranov/go-openai` 依赖
    - 移除 Ollama 的自定义 HTTP 客户端代码
    - 移除重复的重试逻辑代码
    - _Requirements: 2.5, 7.1, 7.3_

- [ ] 9. 编写输出格式属性测试
  - [ ]* 9.1 编写输出格式一致性属性测试
    - **Property 1: Output Format Consistency**
    - 验证 GenerateResponse 结构一致
    - **Validates: Requirements 1.4, 6.3, 6.4**

- [ ] 10. Final Checkpoint - 完整验证
  - 运行 `go test ./...` 确保所有测试通过
  - 运行 `go build ./...` 确保编译成功
  - 验证 `go mod tidy` 无多余依赖
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
