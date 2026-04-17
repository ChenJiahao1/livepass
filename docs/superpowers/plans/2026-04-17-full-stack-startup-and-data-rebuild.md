# Full Stack Startup And Data Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让仓库具备真正的一键启动脚本，以及独立的运行数据重建脚本。

**Architecture:** 保留 `scripts/deploy/start_backend.sh` 作为统一启动入口，补足基础设施拉起、空库初始化、全部 MCP 与 `agents` 前置生成；新增 `scripts/deploy/rebuild_databases.sh` 作为独立的数据层重置入口，复用现有 SQL 导入脚本并重置 Redis、Kafka 业务状态。整个实现按脚本级 TDD 推进，以仓库内 shell 测试作为回归约束。

**Tech Stack:** Bash、Docker Compose、MySQL、Redis、Kafka、Go、uv

---

## 目标文件结构

- 修改：`scripts/deploy/start_backend.sh`
- 新增：`scripts/deploy/rebuild_databases.sh`
- 修改：`tests/start_backend_script_test.sh`
- 新增：`tests/rebuild_databases_script_test.sh`
- 可选修改：`README.md`

### Task 1: 先锁定脚本契约测试

**Files:**
- Modify: `tests/start_backend_script_test.sh`
- Create: `tests/rebuild_databases_script_test.sh`

- [ ] **Step 1: 写启动脚本失败测试**
- [ ] **Step 2: 写重建脚本失败测试**
- [ ] **Step 3: 运行脚本测试确认失败**

### Task 2: 扩展一键启动脚本

**Files:**
- Modify: `scripts/deploy/start_backend.sh`
- Test: `tests/start_backend_script_test.sh`

- [ ] **Step 1: 增加基础设施自动拉起与等待逻辑**
- [ ] **Step 2: 增加空库自动导入逻辑**
- [ ] **Step 3: 增加 `program-mcp` 与 proto stub 生成**
- [ ] **Step 4: 运行启动脚本测试确认通过**

### Task 3: 新增运行数据重建脚本

**Files:**
- Create: `scripts/deploy/rebuild_databases.sh`
- Test: `tests/rebuild_databases_script_test.sh`

- [ ] **Step 1: 实现 MySQL 删除重建并导入**
- [ ] **Step 2: 实现 Redis 清空**
- [ ] **Step 3: 实现 Kafka Topic 删除重建**
- [ ] **Step 4: 运行重建脚本测试确认通过**

### Task 4: 补充文档与最终验证

**Files:**
- Modify: `README.md`
- Test: `tests/start_backend_script_test.sh`
- Test: `tests/rebuild_databases_script_test.sh`

- [ ] **Step 1: 更新 README 命令说明**
- [ ] **Step 2: 运行脚本测试集合**
