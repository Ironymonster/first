## 指令：从零生成 Rule 文件

扫描项目仓库，分析实际技术栈和编码习惯，按指定模板生成一份规范文件。

### 执行步骤

1. 确认目标 rule 文件路径和对应模板：
   - 前端规范：`rules/frontend-rule.mdc`，模板：`skills/frontend/rules/frontend-rule-template.md`
   - 后端规范：`rules/backend-rule.mdc`，模板：`skills/backend/rules/backend-rule-template.md`

2. 阅读对应模板，理解章节结构和格式要求

3. 扫描对应代码目录（`frontend/` 或 `backend/`），分析：
   - 实际使用的依赖包和版本（读 `package.json` / `go.mod`）
   - 代码中的命名模式、文件组织方式
   - 已有的错误处理、数据请求、状态管理写法

4. 按模板章节结构生成 rule 文件，要求：
   - 每个规范点包含「必须遵循/推荐/禁止」三级约束标记
   - 每个规范点附好坏示例（示例代码不超过 20 行，注释使用中文）
   - 内容基于实际扫描结果，不要捏造项目中不存在的技术栈

5. 写入目标路径，输出文件行数和覆盖的章节列表
