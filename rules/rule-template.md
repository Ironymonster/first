---
alwaysApply: true
---

# Pipeline-UI 开发规范

你是一位专注于遵循以下所有规范点的的高级前端开发工程师，所有的编码都要求严格遵循以下所有规范，
你的所有的回复必须使用中文，回复的内容统一在开头加上一句 '命中了项目rule'

---

## 一、技术栈规范

### 1.1 核心技术栈（优先级最高）

| 类别  | 技术               | 说明                             |
| ----- | ------------------ | -------------------------------- |
| 框架  | React + TypeScript | 前端框架和类型系统               |
| Hooks | ahooks             | React Hooks 工具库               |
| UI库  | Antd V5            | UI 组件库                        |
| 工具  | lodash             | 工具函数库（按需引入）           |
| 状态  | zustand            | 全局状态管理（完整模块必须使用） |
| 请求  | useSWR             | 数据请求必须使用                 |
| xxxx  | xxx                | xxxx                             |
| xxxx  | xxx                | xxx                              |

---

## 二、【大分类技术规范的名字】

### 2.1 核心原则

-   **必须遵循**立即触发的请求必须使用 useSWR
-   **推荐**手动触发的请求推荐使用 `wrapApiRes` 包裹
-   **禁止**用useEffect自己来请求数据

### 2.2 示例

```typescript
// ✅ 正确：立即触发的请求使用 useSWR
import { useContext } from 'use-context-selector';
import useSWR from 'swr';
import { sleep } from '@/utils';
import { mockProductList } from './mock';
import { BackendAAAContext } from '@/renderer/contexts/BackendAAAContext';

const { backend } = useContext(BackendAAAContext);
// openRef.current 主要是解决再次打开的时候，loadMore 会一直加载到原本的页码，因为open在改变 所
const listSWR = useSWR(
    openRef.current && newSearchParams ? ['getVoiceList', newSearchParams] : null,
    backend?.getVoiceList,
);

// ✅ 正确：手动触发的请求使用 wrapApiRes 包裹
import { wrapApiRes } from '@/utils';

const handleSubmit = async () => {
    await wrapApiRes(
        fetchApi?.translateSuno({
            type: 'gpt-image',
            image: img,
        }),
        {
            successCallBack(res) {
                // 处理成功响应
                dispatch({
                    type: APP_CLEAR_FORM,
                    payload: {
                        prompt: res.lyrics,
                        tags: res.style,
                        title: res.title,
                    },
                });
            },
            errorCallBack(err) {
                // 处理错误（可选）
                console.error('请求失败:', err);
            },
            successMsg: '操作成功',
            errorMsg: '操作失败，请稍后重试',
        },
    );
};

// ❌ 错误：禁止自定义 fetch
useEffect(() => {
    fetch('/api/products')
        .then((res) => res.json())
        .then(setData);
}, []);

// ❌ 错误：手动请求未使用 wrapApiRes 包裹
const handleClick = async () => {
    try {
        const res = await api.submitData(data);
        message.success('成功');
    } catch (err) {
        message.error('失败');
    }
};
```

---
