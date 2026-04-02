## 指令：按模板重整 Rule 文件

读取现有 rule 文件，按对应模板的章节结构重新整理格式，补充缺失的好坏示例。

### 执行步骤

1. 确认目标文件和对应模板：
   - 前端：`rules/frontend-rule.mdc` ↔ `skills/frontend/rules/frontend-rule-template.md`
   - 后端：`rules/backend-rule.mdc` ↔ `skills/backend/rules/backend-rule-template.md`

2. 阅读模板，理解标准的章节顺序和格式规范

3. 阅读目标 rule 文件，盘点现有内容

4. 重整规则：
   - **保留**所有现有规范内容，不删除任何规范点
   - **重排**章节顺序，与模板对齐
   - **补充**缺少好坏示例的规范点（示例不超过 15 行，注释中文）
   - **格式统一**：约束标记（必须遵循/推荐/禁止）、示例代码块风格

5. 写入目标文件，输出：调整了几个章节、补充了几处示例
