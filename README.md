# Harness — Agent 自愈循环管理框架

## 问题

Agent 生成测试用例和 E2E 脚本，一次生成达不到预期，必须人盯着反复跑、反复修。核心矛盾：

- **批量生成**，质量不够
- **需要迭代**，但全靠人介入
- **希望 Agent 自己收敛**，减少人的介入次数

## 解法

不是改生成逻辑，而是搭建 **Harness**——Agent 运行的执行环境。借鉴 OpenAI Harness Engineering 三层模型：

| 层 | 优化什么 | 本项目对应 |
|---|---|---|
| Prompt Engineering | 单次交互质量 | prompts 表：模板版本管理 |
| Context Engineering | 模型能看到什么 | runs 表：执行上下文记录 |
| Harness Engineering | Agent 长时间自主运行 | 自动评估 + 止损 + 策略升级 |

核心闭环：

```
读prompt → Agent生成 → 自动评估(Observe) → 不通过? → 策略升级(Adapt) → 新prompt → 再生成
                                              → 通过? → 收敛(converged)
                                              → 连续3次同类失败? → 止损(stuck) → 人工介入
```

## 数据模型

### prompts 表 — 提示词版本管理 + 策略层级

| 字段 | 类型 | 说明 |
|------|------|------|
| 名称 | text | 提示词名称 |
| 分类 | 单选 | test_case / e2e / other |
| 模板内容 | text | 提示词模板，支持 `{{变量}}` 占位 |
| 版本 | number | 版本号，迭代递增 |
| 状态 | 单选 | active / draft / deprecated |
| **策略层级** | 单选 | basic → constrained → few_shot → strict |
| **覆盖报告** | text | 对上一版反馈的核销结果 |
| 备注 | text | 版本变更说明 |

### runs 表 — 执行上下文记录 + 评估 + 收敛

| 字段 | 类型 | 说明 |
|------|------|------|
| 关联提示词 | 双向关联→prompts | 本次用的是哪个 prompt |
| 任务描述 | text | 具体任务 |
| 输出摘要 | text | Agent 生成了什么 |
| 状态 | 单选 | ok / partial / fail / pending |
| **评估信号** | text | 机器可判读的具体信号 |
| **评估方式** | 单选 | auto_rule / auto_run / manual |
| 反馈 | text | 人的反馈：哪里不对 |
| 纠偏内容 | text | 怎么调的：prompt 改了什么 |
| 迭代轮次 | number | 第几轮 |
| 下轮提示词 | text | 纠偏后新 prompt 的 record_id |
| **收敛状态** | 单选 | converging / stuck / converged |
| 执行时间 | datetime | 执行时间 |

### 关键关系

```
run.关联提示词 ──(双向关联)──→ prompts.执行记录
run.下轮提示词 ──────────────→ prompts(record_id)  ← 迭代链指针
prompt.策略层级 ─────────────→ basic → constrained → few_shot → strict  ← 策略升级路径
```

## 三大核心能力

### 1. 自动评估（Observe）

借鉴自 OpenAI Harness Engineering 的"Give the Agent Eyes"原则和童天浩文章的三信号验证栈。

- **test_case 类**：关键词覆盖度检查（批量/权限/异常/自定义/模板/联动）
- **e2e 类**：反模式检测（data-testid/sleep）+ 好模式检测（aria-label/waitForSelector/POM）
- 输出结构化评估信号，如 `scenario_coverage=4/6; missing=批量,异常`

```bash
$ ./harness evaluate recvkwFp
评估结果: scenario_coverage=1/6; missing=批量,异常,自定义,模板,联动
评估方式: auto_rule
```

### 2. 止损机制（Stop-loss）

借鉴自童天浩文章："连续 3 次同类失败 → 停下上抛给人"。

- 按 prompt 分组统计连续失败次数
- 连续 ≥3 次 partial/fail → 标记 stuck，提醒人工介入
- 有 ok 出现 → 重置计数，标记 converged

```bash
$ ./harness check-convergence
prompt recvkwFm: converging (连续失败=1, runs=1)
prompt recvkwFs: converged (连续失败=0, runs=1)
prompt recvkwFz: stuck ⚠️ 止损触发！连续 3 次未通过，建议人工介入
```

### 3. 策略升级（Adapt）

借鉴自 OpenAI "Ask what capability is missing"原则和知识库的 coverage 核销思路。

策略层级定义了 prompt 升级的方向，不是瞎试：

| 层级 | 含义 | 升级动作 |
|------|------|---------|
| basic | 无约束的通用模板 | — |
| constrained | 加场景枚举约束、格式要求 | 增加场景枚举、边界条件 |
| few_shot | 加高质量示例 | 增加 2-3 个 few-shot 示例 |
| strict | 加反例+格式校验+禁止项 | 增加反例、输出格式校验规则 |

```bash
$ ./harness next-strategy recvkwFm
当前策略: basic → 建议升级到: constrained
升级动作：在 prompt 中增加场景枚举约束、格式要求、边界条件

最近一次执行: status=partial
评估信号: scenario_coverage=1/6; missing=批量,异常,自定义,模板,联动
```

## Demo 实录

任务：**为 Meego 工作项创建功能生成测试用例和 E2E 脚本**

### 链路 1：测试用例 basic → constrained → converged

```
[recvkwFp] iter=1 status=partial
  评估: scenario_coverage=1/6; missing=批量,异常,自定义,模板,联动 (auto_rule)
  反馈: 缺少4类场景
  纠偏: 增加场景枚举约束
  收敛: converging
[recvkwFw] iter=2 status=ok
  收敛: converged
```

prompt 策略：basic → constrained，覆盖报告：✅6/6 场景全覆盖

### 链路 2：E2E 脚本 basic → constrained → converged

```
[recvkwFC] iter=1 status=fail
  评估: good_patterns=0; bad_patterns=data-testid (auto_rule)
  反馈: 选择器/等待/架构三问题
  纠偏: 指定aria-label+POM+等待策略
  收敛: converging
[recvkwFJ] iter=2 status=ok
  收敛: converged
```

prompt 策略：basic → constrained，覆盖报告：✅5/6（缺视觉验证）

## CLI 用法

```bash
go build -o harness .

./harness prompts                       # 列出所有提示词（含策略层级）
./harness runs                          # 列出所有执行记录（含收敛状态）
./harness add-prompt                    # 交互式添加提示词
./harness add-run                       # 交互式添加执行记录
./harness iterate <record_id>           # 查看迭代链（含评估信号）
./harness evaluate <record_id>          # 自动评估执行结果
./harness check-convergence             # 检查所有迭代链的收敛状态
./harness next-strategy <prompt_id>     # 建议下一步策略升级
```

## 飞书多维表格

https://bytedance.larkoffice.com/base/ZhSDbHEVuaW99ks8uFrcfdo9nAe

## 参考文章

- [OpenAI: Harness Engineering — Leveraging Codex in an Agent-First World](https://openai.com/index/harness-engineering/) — 三层模型、策略升级、"Ask what capability is missing"
- [Anthropic: Effective Harnesses for Long-Running Agents](https://loreai.dev/blog/effective-harnesses-for-long-running-agents) — 初始化 Agent + 增量编码 Agent + 进度文件
- [The Anatomy of an Agent Harness](https://www.dailydoseofds.com/p/the-anatomy-of-an-agent-harness/) — 11 组件模型、验证循环、止损规则
- [童天浩: Agent 验证闭环实践](https://bytetech.info/articles/7636619750239698970?from=skill) — 三信号验证栈、4 个 checkpoint、止损规则
- [徐志: knowledge_forge_skill 知识库构建框架](https://bytetech.info/articles/7636606482238472198?from=skill) — 两阶段编译、coverage 核销、健康分

## 下一步

1. **LLM 反思（Reflect）** — 把 prompt+输出+评估信号喂给 LLM，自动生成反馈和纠偏建议
2. **自动写 run** — Agent 执行完自动追加记录
3. **自动升版 prompt** — 根据反馈+策略层级自动生成新版本
4. **批量调度** — 从表格读一批 task，批量跑 Observe→Reflect→Adapt 循环
5. **multi-agent verification** — 第二个 Agent 做独立交叉检查
