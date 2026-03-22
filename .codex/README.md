# damai-go Codex Context

本目录为当前 `damai-go` 项目提供 Codex 本地上下文补充。

优先级：

1. 项目根目录 `AGENTS.md`
2. 本文件
3. `.codex/ai-context/00-instructions.md`
4. `.codex/ai-context/workflows.md`
5. `.codex/ai-context/patterns.md`
6. `.codex/ai-context/tools.md`

使用约束：

- `ai-context` 只作为 go-zero 开发流程、常见模式、goctl 用法的补充参考。
- 不能覆盖项目根目录 `AGENTS.md` 中已经定义的服务边界、目录规范和命名约束。
- 本项目所有新增或重生成的 `goctl` 代码一律使用 `--style go_zero`，不要根据仓库里的历史混用文件名反推 style。
- `zero-skills` 已通过全局 skills 提供，本项目不在仓库内重复存放。

更新说明：

- 上游来源：`https://github.com/zeromicro/ai-context`
- 当前快照日期：`2026-03-16`
- 如需同步上游，优先替换 `.codex/ai-context/` 下的静态文件，再检查本文件和根 `AGENTS.md` 的引用是否仍然准确。
