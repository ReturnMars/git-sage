# Implementation Plan: PATH Detection

## Overview

实现 GitSage 首次运行时的 PATH 检测和自动添加功能。该功能集成到现有 CLI 框架中，支持 Windows、macOS 和 Linux 平台，以及 bash、zsh、fish 等常见 shell。

## Tasks

- [x] 1. 扩展配置结构
  - 在 `SecurityConfig` 中添加 `PathCheckDone` 字段
  - 添加环境变量绑定 `GITSAGE_SECURITY_PATH_CHECK_DONE`
  - 设置默认值为 `false`
  - _Requirements: 1.4, 1.5, 3.2_

- [x] 2. 创建 pathcheck 包核心接口
  - [x] 2.1 创建 `internal/pkg/pathcheck/checker.go`
    - 定义 `Checker` 接口
    - 定义 `ShellType` 枚举
    - 定义 `PathAddResult` 结构体
    - 实现 `NewChecker()` 工厂函数
    - _Requirements: 1.1, 2.1_

  - [x] 2.2 实现 Unix 平台检测器 `internal/pkg/pathcheck/unix.go`
    - 实现 `UnixChecker` 结构体
    - 实现 `IsInPath()` 方法
    - 实现 `GetExecutableDir()` 方法
    - 实现 `detectShell()` 内部方法
    - 实现 `GetShellProfile()` 方法
    - _Requirements: 1.1, 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 2.3 编写 Shell Profile 选择属性测试
    - **Property 3: Shell profile selection**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5**

  - [x] 2.4 实现 Windows 平台检测器 `internal/pkg/pathcheck/windows.go`
    - 实现 `WindowsChecker` 结构体
    - 实现 `IsInPath()` 方法
    - 实现 `GetExecutableDir()` 方法
    - 实现 `GetShellProfile()` 方法（返回空，Windows 不使用）
    - _Requirements: 1.1, 2.2_

- [x] 3. 实现 PATH 添加功能
  - [x] 3.1 实现 Unix `AddToPath()` 方法
    - 生成正确的 export 语句（bash/zsh vs fish）
    - 追加到 shell profile 文件
    - 保留文件权限
    - _Requirements: 2.3, 4.2, 4.3, 4.4_

  - [x] 3.2 编写 Export 语句格式属性测试
    - **Property 5: Unix export statement format**
    - **Validates: Requirements 2.3**

  - [x] 3.3 实现 Windows `AddToPath()` 方法
    - 使用 `setx` 命令添加到用户 PATH
    - 处理命令执行错误
    - _Requirements: 2.2_

- [x] 4. 实现错误处理和回退指引
  - [x] 4.1 创建 `internal/pkg/pathcheck/errors.go`
    - 定义 `PathCheckError` 类型
    - 定义错误码枚举
    - 实现错误包装函数
    - _Requirements: 2.5_

  - [x] 4.2 创建 `internal/pkg/pathcheck/instructions.go`
    - 实现 `GetManualInstructions()` 函数
    - 为 Windows/Unix/Fish 生成对应的手动配置说明
    - _Requirements: 2.5_

- [x] 5. 集成到 CLI
  - [x] 5.1 修改 `internal/cmd/root.go`
    - 添加 `--skip-path-check` 全局 flag
    - 在 `PersistentPreRunE` 中添加 PATH 检测逻辑
    - 跳过 config 和 help 命令的检测
    - _Requirements: 1.1, 1.2, 1.3, 3.1_

  - [x] 5.2 编写 Skip Flag 行为属性测试
    - **Property 4: Skip flag behavior**
    - **Validates: Requirements 3.1**

  - [x] 5.3 实现用户交互流程
    - 使用现有 UI Manager 显示提示
    - 询问用户是否自动添加
    - 显示成功/失败消息
    - _Requirements: 1.2, 2.4, 2.5, 2.6_

- [x] 6. 实现配置持久化
  - [x] 6.1 添加 `SetPathCheckDone()` 方法到 config manager
    - 设置 `security.path_check_done` 为 true
    - 确保配置文件存在（如不存在则创建）
    - _Requirements: 1.4, 2.6_

  - [x] 6.2 添加 `IsPathCheckDone()` 方法到 config manager
    - 读取 `security.path_check_done` 值
    - 默认返回 false
    - _Requirements: 1.5_

  - [x] 6.3 编写配置标志持久化属性测试
    - **Property 2: Config flag persistence round-trip**
    - **Validates: Requirements 1.4, 1.5, 2.6, 3.2**

- [x] 7. 编写单元测试
  - [x] 7.1 创建 `internal/pkg/pathcheck/checker_test.go`
    - 测试 `NewChecker()` 工厂函数
    - 测试 `IsInPath()` 各种场景
    - 测试 `GetExecutableDir()`
    - _Requirements: 1.1_

  - [x] 7.2 创建 `internal/pkg/pathcheck/unix_test.go`
    - 测试 shell 检测逻辑
    - 测试 profile 路径选择
    - 测试 export 语句生成
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 7.3 创建 `internal/pkg/pathcheck/windows_test.go`
    - 测试 Windows PATH 检测
    - 测试 setx 命令生成
    - _Requirements: 2.2_

- [x] 8. Checkpoint - 确保所有测试通过
  - 运行 `go test ./...`
  - 确保所有测试通过，如有问题请询问用户

- [x] 9. 更新文档
  - [x] 9.1 更新 README.md
    - 添加 PATH 检测功能说明
    - 添加 `--skip-path-check` flag 文档
    - _Requirements: All_

  - [x] 9.2 更新 README_CN.md
    - 同步中文文档
    - _Requirements: All_

- [ ] 10. 最终验证
  - 在 Windows 上测试完整流程
  - 在 macOS/Linux 上测试 bash/zsh/fish
  - 验证配置持久化
  - 验证手动配置说明的准确性
  - _Requirements: All_

## Notes

- 所有任务均为必需任务
- 每个属性测试引用设计文档中的对应属性
- 检测逻辑应在 config 和 help 命令时跳过
- Windows 使用 setx 命令，Unix 使用 shell profile 修改
