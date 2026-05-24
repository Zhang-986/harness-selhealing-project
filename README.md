# Harness — Agent 自愈循环的上下文管理层

## 问题

Agent 生成测试用例和 E2E 脚本，一次生成达不到预期，必须人盯着反复跑、反复修。核心矛盾：

- **批量生成**，质量不够
- **需要迭代**，但全靠人介入
- **希望 Agent 自己收敛**，减少人的介入次数

## 解法

不是改生成逻辑，而是先把"记忆"搭起来。用飞书多维表格做结构化的上下文记录 + 提示词管理，让每次迭代有迹可循、可回溯、可自动串联。

核心数据模型只有两张表，一条闭环：

```
执行(run) → 反馈(feedback) → 纠偏(correction) → 新提示词(prompt v2) → 再执行(run v2)
```

## 数据模型

### prompts 表 — 提示词版本管理

| 字段 | 类型 | 说明 |
|------|------|------|
| 名称 | text | 提示词名称 |
| 分类 | 单选 | test_case / e2e / other |
| 模板内容 | text | 提示词模板，支持 `{{变量}}` 占位 |
| 版本 | number | 版本号，迭代递增 |
| 状态 | 单选 | active / draft / deprecated |
| 备注 | text | 版本变更说明 |

### runs 表 — 执行上下文记录

| 字段 | 类型 | 说明 |
|------|------|------|
| 关联提示词 | 双向关联→prompts | 本次用的是哪个 prompt |
| 任务描述 | text | 具体任务 |
| 输出摘要 | text | Agent 生成了什么 |
| 状态 | 单选 | ok / partial / fail / pending |
| 反馈 | text | 人的反馈：哪里不对 |
| 纠偏内容 | text | 怎么调的：prompt 改了什么 |
| 迭代轮次 | number | 第几轮 |
| 下轮提示词 | text | 纠偏后新 prompt 的 record_id（迭代链指针） |
| 执行时间 | datetime | 执行时间 |

### 关键关系

```
run.关联提示词 ──(双向关联)──→ prompts.执行记录    ← 结构化关联，非文本 ID
run.下轮提示词 ──────────────→ prompts(record_id)  ← 迭代链的"指针"
```

在飞书多维表格里，点开一条 prompt，"执行记录"列直接展开所有关联 run；点开一条 run，"关联提示词"直接跳转。这就是选多维表格而非电子表格的原因。

## Demo 实录

任务：**为 Meego 工作项创建功能生成测试用例和 E2E 脚本**

### 链路 1：测试用例 partial → ok

**iter=1 | partial**

- prompt v1：基础模板，只说"生成测试用例"
- 结果：8 条用例，覆盖正常创建、必填校验、权限校验
- 反馈：缺少批量创建、自定义字段、模板选择、工作流联动 4 类场景
- 纠偏：在 prompt 中增加场景枚举约束，要求必须覆盖 6 大类

**iter=2 | ok**

- prompt v2：增加场景覆盖要求（6 大类枚举）
- 结果：22 条用例，6 大类全覆盖，带优先级标注

### 链路 2：E2E 脚本 fail → ok

**iter=1 | fail**

- prompt v1：基础模板，只说"生成 E2E 脚本"
- 结果：8 个 test 文件，3 个超时失败
- 反馈：(1) data-testid 项目没有 (2) 无等待策略 (3) 未封装公共操作
- 纠偏：指定选择器策略(aria-label/role)、等待策略(waitForSelector+networkIdle)、POM 架构

**iter=2 | ok**

- prompt v2：增加编码规范五大约束
- 结果：1 个 BasePage + 6 个 test 文件，8/8 通过

### 迭代链 CLI 输出

```
$ ./harness iterate recvkwFpkyOWEh

=== 迭代链 (从 recvkwFp 开始) ===
[recvkwFp] iter=1 status=partial
  任务: 为Meego工作项创建功能生成测试用例
  反馈: 缺少：1.批量创建场景 2.自定义字段场景 3.模板选择场景 4.工作流状态联动场景
  纠偏: 在prompt中增加场景枚举约束：必须覆盖批量操作、自定义字段、模板选择、工作流联动四类场景
[recvkwFw] iter=2 status=ok
  任务: 为Meego工作项创建功能生成测试用例(v2纠偏后)
```

## CLI 用法

```bash
go build -o harness .

./harness prompts              # 列出所有提示词
./harness runs                 # 列出所有执行记录
./harness add-prompt           # 交互式添加提示词
./harness add-run              # 交互式添加执行记录
./harness iterate <record_id>  # 查看迭代链
```

## 飞书多维表格

https://bytedance.larkoffice.com/base/ZhSDbHEVuaW99ks8uFrcfdo9nAe

## 下一步

当前是"记忆层" MVP。往上可以叠：

1. **自动写 run** — Agent 执行完自动追加记录
2. **自动升版 prompt** — 根据反馈自动生成新版本
3. **收敛判断** — 统计 iteration 数和 status 变化，判断是否收敛
4. **批量调度** — 从表格读一批 task，批量跑生成+执行+反馈循环
