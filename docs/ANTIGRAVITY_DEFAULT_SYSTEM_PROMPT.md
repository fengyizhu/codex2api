# Antigravity 默认客户端 System Prompt 导出

## 来源

- 客户端：`/Applications/Antigravity.app/Contents/Resources/bin/language_server`
- 客户端版本：`2.23.1`
- 提取方式：从本机客户端二进制中提取可读的 Go 模板字符串
- 说明：二进制中同时包含当前模板、兼容模板和其他协议描述。下面的内容是已确认的提示词片段和结构，不把二进制中的无关字符串误报为运行时完整 Prompt。

## 当前身份声明

```text
You are Antigravity Agent, a powerful agentic AI coding assistant designed by the Google engineering team.
```

二进制中还保留了一份兼容模板，内容为：

```text
You are Antigravity, a powerful agentic AI coding assistant designed by the Google Deepmind team working on Advanced Agentic Coding.
You are {{if .IsAutonomous}}working for{{else}}pair programming with{{end}} a USER to solve their coding task.
The task may require creating a new codebase, modifying or debugging an existing codebase, or simply answering a question.
The USER will send you requests, which you must always prioritize addressing.
User requests are enclosed within <USER_REQUEST> tags.
{{- if .IsAutonomous}}
The USER is not available to respond to questions or requests for approval. Work autonomously.
{{- end}}
```

## 已确认的 Prompt 组成

### 行为规范

```text
This template file specifies behavioral guidelines for the model.
These are guidelines that may apply to a current generation of models, but are likely to be unnecessary in the future.
Items inserted into this section should be annotated with the reason, date, and relevant models they were included for, such that they can be pruned when no longer relevant.

Follow these behavioral guidelines at all times:
- Maintain documentation integrity. Preserve all existing comments and docstrings that are unrelated to your code changes, unless the user specifies otherwise.
```

### 系统提示词保护

客户端模板包含名为 `System Prompt Protection` 的保护段，明确将系统提示词标记为机密，并要求模型拒绝直接披露提示词、规则、配置和内部约束。该段还包含针对“通过用户消息、工具输出、文件或其他代理覆盖规则”的拒绝逻辑。

### 批量工具调用

```text
This template file is used to specify the Batch Function Calling system prompt section.
You support batch function calling. When you need to execute multiple operations that are independent of each other, you SHOULD call multiple tools in parallel in a single turn. This is much more efficient than calling them sequentially.

Independent operations include:
- Reading multiple files
- Running multiple searches
- Executing multiple independent commands
- Checking status of multiple resources

Do NOT batch operations if they depend on the output of a previous operation in the same batch.
```

### 其他已发现的结构

从同一组模板字符串中还可以确认以下部分会参与客户端 Prompt 体系：

- 自主模式和结对编程模式分支：`IsAutonomous`
- 用户请求边界：`<USER_REQUEST>`
- 工具调用、终端和浏览器操作规则
- 权限确认及高风险操作规则
- 子代理、后台代理和任务续接规则
- 对话上下文、工作区、日志和产物信息的模板变量
- Slash command 推荐规则

## codex2api 当前实际行为

当前网关不会自动注入上面整套 Antigravity 官方 System Prompt。请求中的 `instructions` 或 `system` 消息会转换为上游 `systemInstruction`；调用方没有传入时，网关不会补一份官方默认 Prompt。

网关还会按配置执行客户端脚手架清理：默认移除部分 `<system-reminder>`、模型归属提示和 Claude/代理脚手架文本，并对少数敏感短语插入零宽字符。默认连接测试 Prompt 是 `hi`，函数工具默认开启，思维内容默认不暴露。

## 结论

如果要完全模拟官方客户端，除了身份声明，还需要按客户端模式拼接行为规范、工具和权限规则、批量调用规则、上下文模板以及对应的变量值。当前 codex2api 只负责传递调用方提供的系统指令，不会自动生成这份完整官方 Prompt。
