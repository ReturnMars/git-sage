# Requirements Document

## Introduction

本文档定义了将 GitSage 项目的 AI 层从当前的多 provider 独立实现重构为基于 LangChain Go (`langchaingo`) 统一抽象的需求。

当前实现存在以下问题：
- 每个 AI provider（OpenAI、DeepSeek、Ollama）都有独立的 HTTP 调用逻辑
- 重复的重试机制、错误处理代码
- 添加新 provider 需要大量样板代码
- Prompt 管理与 provider 实现耦合

通过引入 LangChain Go，可以获得统一的 LLM 抽象、内置的 prompt 模板支持，以及更简洁的代码结构。

## Glossary

- **LangChain_Go**: Go 语言版本的 LangChain 框架 (`github.com/tmc/langchaingo`)，提供统一的 LLM 抽象层
- **Provider**: AI 服务提供商的实现，如 OpenAI、DeepSeek、Ollama
- **LLM**: Large Language Model，大语言模型
- **PromptTemplate**: LangChain 中用于管理和渲染 prompt 的模板系统
- **GenerateRequest**: 包含 diff 信息的请求结构，用于生成 commit message
- **GenerateResponse**: 包含生成的 commit message 的响应结构
- **Factory**: 工厂模式，根据配置创建对应的 provider 实例

## Requirements

### Requirement 1: 保持现有接口兼容性

**User Story:** As a developer, I want the refactored AI layer to maintain the same public interface, so that existing code using the AI package doesn't need to change.

#### Acceptance Criteria

1. THE Refactored_System SHALL preserve the existing `Provider` interface definition unchanged
2. THE Refactored_System SHALL preserve the existing `GenerateRequest` and `GenerateResponse` structures unchanged
3. THE Refactored_System SHALL preserve the existing `NewProvider` factory function signature unchanged
4. WHEN existing code calls `provider.GenerateCommitMessage()`, THE Refactored_System SHALL return results in the same format as before

### Requirement 2: 使用 LangChain Go 统一 LLM 调用

**User Story:** As a developer, I want to use LangChain Go for all LLM interactions, so that I can benefit from its unified abstraction and reduce code duplication.

#### Acceptance Criteria

1. THE Refactored_System SHALL use `langchaingo/llms` package for all LLM API calls
2. THE Refactored_System SHALL use `langchaingo/llms/openai` for OpenAI provider implementation
3. THE Refactored_System SHALL use `langchaingo/llms/ollama` for Ollama provider implementation
4. WHEN DeepSeek provider is used, THE Refactored_System SHALL configure OpenAI-compatible client with DeepSeek endpoint
5. THE Refactored_System SHALL remove direct HTTP client code from provider implementations

### Requirement 3: 使用 LangChain Prompt 模板

**User Story:** As a developer, I want to use LangChain's prompt template system, so that prompt management is more standardized and maintainable.

#### Acceptance Criteria

1. THE Refactored_System SHALL use `langchaingo/prompts` package for prompt template management
2. THE Refactored_System SHALL convert existing `DefaultSystemPrompt` to LangChain prompt format
3. THE Refactored_System SHALL convert existing `DefaultUserPromptTemplate` to LangChain prompt format
4. WHEN custom prompts are provided, THE Refactored_System SHALL support them through LangChain's template system
5. THE Refactored_System SHALL preserve the existing prompt content and variables

### Requirement 4: 保持错误处理和重试机制

**User Story:** As a developer, I want the refactored system to maintain robust error handling and retry logic, so that the system remains reliable.

#### Acceptance Criteria

1. WHEN an API call fails with a retryable error, THE Refactored_System SHALL retry with exponential backoff
2. THE Refactored_System SHALL preserve the maximum retry count of 3 attempts
3. THE Refactored_System SHALL preserve the existing error wrapping with user-friendly messages
4. IF a rate limit error occurs, THEN THE Refactored_System SHALL return appropriate rate limit error
5. IF an authentication error occurs, THEN THE Refactored_System SHALL return appropriate authentication error

### Requirement 5: 保持配置兼容性

**User Story:** As a user, I want my existing configuration to work without changes, so that I don't need to reconfigure the tool after the update.

#### Acceptance Criteria

1. THE Refactored_System SHALL accept the same `ProviderConfig` structure
2. THE Refactored_System SHALL support the same provider names: "openai", "deepseek", "ollama"
3. THE Refactored_System SHALL use the same default values for model, temperature, and max tokens
4. WHEN endpoint is specified in config, THE Refactored_System SHALL use it for API calls
5. WHEN API key is specified in config, THE Refactored_System SHALL use it for authentication

### Requirement 6: 保持 Commit Message 解析逻辑

**User Story:** As a developer, I want the commit message parsing logic to remain unchanged, so that the output format stays consistent.

#### Acceptance Criteria

1. THE Refactored_System SHALL preserve the existing `ParseCommitMessage` function
2. THE Refactored_System SHALL preserve the existing `ParsedCommitMessage` structure
3. WHEN LLM returns a response, THE Refactored_System SHALL parse it using the existing parser
4. THE Refactored_System SHALL preserve the existing response format with Subject, Body, Footer, and RawText

### Requirement 7: 简化代码结构

**User Story:** As a maintainer, I want the refactored code to be simpler and more maintainable, so that adding new providers or features is easier.

#### Acceptance Criteria

1. THE Refactored_System SHALL reduce code duplication across provider implementations
2. THE Refactored_System SHALL have a single unified LLM calling mechanism
3. THE Refactored_System SHALL remove provider-specific HTTP client code
4. WHEN a new provider needs to be added, THE Refactored_System SHALL require minimal boilerplate code
