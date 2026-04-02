---
alwaysApply: false
---

# 前端开发规范

你是一位专注于遵循以下所有规范点的的高级前端开发工程师，所有的编码都要求严格遵循以下所有规范，
你的所有的回复必须使用中文，且代码必须有中文注释。

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

## 二A、ESLint 合规规范

### 2A.1 核心原则

-   **必须遵循** 禁止使用三元表达式作为独立语句（`no-unused-expressions`），改用 `if/else`
-   **必须遵循** `useMemo` / `useEffect` 依赖项中禁止出现每次渲染都会新建引用的表达式，如 `?? []`

### 2A.2 示例

```typescript
// ✅ 正确：用 if/else 替代三元表达式语句
if (condition) {
    doA();
} else {
    doB();
}

// ✅ 正确：用 useMemo 稳定 ?? [] 默认值，避免每次渲染新建数组
const items = useMemo(() => data?.list ?? [], [data?.list]);

// ❌ 错误：三元表达式作为独立语句，违反 no-unused-expressions
condition ? doA() : doB();

// ❌ 错误：?? [] 每次渲染生成新数组，导致依赖它的 useMemo 失效
const items = data?.list ?? [];
const result = useMemo(() => process(items), [items]); // items 引用每次都变
```

### 2A.3 删除组件/函数时必须清理孤立 imports

-   **必须遵循** 删除组件或函数代码块时，必须反向追踪并同步删除**仅被该代码块使用**的 import 语句
-   **禁止** 遗留无任何引用的 import，会触发 `no-unused-vars` ESLint warning/error，导致构建失败
-   **推荐** 删除代码后检查文件顶部 import 区域，逐条确认每个 import 是否仍被使用

```typescript
// 场景：删除某个组件

// ✅ 正确：同步删除仅被已删除组件使用的 import
// 删除前：import { SomeIcon, OtherIcon } from 'icon-lib';
// 删除后（SomeIcon 只有被删组件用，OtherIcon 其他地方还在用）：
import { OtherIcon } from 'icon-lib';

// ❌ 错误：只删除组件代码，遗留孤立 import
import { SomeIcon, OtherIcon } from 'icon-lib'; // SomeIcon 已无引用，触发 no-unused-vars
// ... 组件代码已被删除 ...
```

---

---

## 五、Button 嵌套规范

### 5.1 核心原则

- **禁止** `<button>` 内部嵌套另一个 `<button>`，违反 HTML 规范会导致 SSR hydration 失败
- **必须遵循** 需要在可点击容器内再嵌套按钮时，外层改用 `<div role="button" tabIndex={0}>` 并补充 `onKeyDown` 保持键盘可访问性

### 5.2 示例

```tsx
// ✅ 正确：外层用 div 模拟 button，内部可嵌套真实 button
<div
  role="button"
  tabIndex={0}
  onClick={onClick}
  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onClick(); }}
  className="cursor-pointer w-full"
>
  <span>{label}</span>
  <button onClick={(e) => { e.stopPropagation(); handleAction(); }}>
    <ActionIcon />
  </button>
</div>

// ❌ 错误：button 内嵌套 button，违反 HTML 规范
<button onClick={onClick} className="w-full">
  <span>{label}</span>
  <button onClick={handleAction}><ActionIcon /></button>
</button>
```

---

## 六、可折叠面板布局规范

### 6.1 核心原则

- **必须遵循** 可折叠容器必须使用 `motion.div` + `animate={{ width }}` 控制展开/收起，禁止用条件渲染或 `display:none` 代替
- **必须遵循** 面板容器必须设置 `h-full` 和 `overflow-hidden`（或 `style={{ overflowX: 'hidden' }}`），防止内容溢出撑破父级滚动约束
- **推荐** 折叠动效统一使用 `spring` 类型（`stiffness: 300, damping: 30`）保持全局一致性

### 6.2 示例

```tsx
// ✅ 正确：motion.div 控制宽度折叠，h-full + overflow 约束容器
<motion.div
  className="relative h-full shrink-0 flex-col border-l"
  animate={{ width: panelOpen ? '30vw' : 0 }}
  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
  style={{ overflowX: 'hidden' }}
>
  <div className="absolute inset-0 flex flex-col">{/* 面板内容 */}</div>
</motion.div>

// ❌ 错误：无高度约束、无宽度折叠，面板始终占空间撑破父级
<div className="relative flex flex-col border-l">
  <div className="absolute inset-0 flex min-w-[30vw] flex-col">{/* 面板内容 */}</div>
</div>
```

---

## 七、下拉菜单 Portal 定位规范

### 7.1 核心原则

- **必须遵循** 下拉菜单/浮层若祖先链存在 `overflow:hidden`，必须使用 `createPortal` 挂载到 `document.body`，配合 `position: fixed` + `getBoundingClientRect()` 动态定位
- **禁止** 对可能被裁剪的浮层使用 `position: absolute`，即使设置了高 `z-index` 也无效
- **必须遵循** 外部点击关闭时，`mousedown` 判断必须同时排除触发器和菜单容器本身
- **推荐** 打开时检测视口下方剩余空间，不足时向上展开

### 7.2 示例

```tsx
// ✅ 正确：portal + fixed 定位，脱离祖先 overflow 约束
import { createPortal } from 'react-dom';

const triggerRef = useRef<HTMLButtonElement>(null);
const menuRef = useRef<HTMLDivElement>(null);
const [menuStyle, setMenuStyle] = useState<React.CSSProperties>({});

const calcPosition = useCallback(() => {
  if (!triggerRef.current) return;
  const rect = triggerRef.current.getBoundingClientRect();
  const below = window.innerHeight - rect.bottom;
  if (below >= 200) {
    setMenuStyle({ top: rect.bottom + 4, left: rect.left });
  } else {
    setMenuStyle({ bottom: window.innerHeight - rect.top + 4, left: rect.left });
  }
}, []);

useEffect(() => {
  if (!isOpen) return;
  const handler = (e: MouseEvent) => {
    if (
      !triggerRef.current?.contains(e.target as Node) &&
      !menuRef.current?.contains(e.target as Node)
    ) setIsOpen(false);
  };
  document.addEventListener('mousedown', handler);
  return () => document.removeEventListener('mousedown', handler);
}, [isOpen]);

return (
  <div className="inline-block">
    <button ref={triggerRef} onClick={() => { calcPosition(); setIsOpen(v => !v); }}>
      触发器
    </button>
    {isOpen && createPortal(
      <div ref={menuRef} className="fixed z-[9999]" style={menuStyle}>
        {/* 菜单内容 */}
      </div>,
      document.body,
    )}
  </div>
);

// ❌ 错误：absolute 菜单被祖先 overflow:hidden 裁剪
<div className="relative inline-block">
  <button onClick={toggle}>触发器</button>
  {isOpen && (
    <div className="absolute left-0 top-full z-50">
      {/* 可能被裁剪 */}
    </div>
  )}
</div>
```


### 8.8 useMemo 缓存派生数据：避免每次渲染重建计算结果

- **必须遵循** 根据 props/state 派生的**列表**和**查找表**（如 `keyBy` 构建的 Map）必须用 `useMemo` 缓存，避免每次渲染重建
- **必须遵循** `useMemo` 的依赖项必须**精确**到最小粒度的标量值，不应依赖整个大对象，否则大对象任意字段变化都会触发无效重算
- **推荐** 先从大对象中取出所需的精确标量值，再将其作为 `useMemo` 的依赖

```typescript
// ✅ 正确：先取出精确标量值，只依赖这些值，无关变化不触发重算
const itemType = stateMap[activeId]?.type;

const filteredList = useMemo(
    () => allItems.filter(i => i.type === itemType),
    [allItems, itemType],  // 只依赖精确标量值
);

const itemMap = useMemo(
    () => keyBy(filteredList, 'id'),
    [filteredList],
);

// ❌ 错误：直接依赖整个大对象，任意字段变化都触发无效重算
const filteredList = useMemo(
    () => allItems.filter(i => i.type === stateMap[activeId]?.type),
    [stateMap, activeId],  // stateMap 任意字段更新都触发重建
);
```

---

### 8.9 条件赋值使用 `!== undefined` 替代 truthy 判断

- **必须遵循** 构建可选字段对象时，判断值是否存在应使用 `!== undefined`，而非 truthy 判断（`if (value)`），避免空字符串 `""` 、`0`、`false` 等合法值被误跳过
- **推荐** 使用显式赋值替代三元展开语法，代码意图更清晰

```typescript
// ✅ 正确：使用 !== undefined，空字符串等合法值不被误过滤
const params: Record<string, unknown> = {};
if (type !== undefined) {
    params.type = type;  // type="" 也会被正确写入
}

// ❌ 错误：truthy 判断会过滤掉空字符串等合法值
const params = {
    ...(type ? { type } : {}),  // type="" 时被错误跳过
};
```

---



- **必须遵循** 若某个回调需要放入 `useEffect` 的空依赖数组（`[]`），但其内部又依赖会随渲染变化的函数/状态，必须用 `useRef` 持有最新引用，在 `useEffect` 内通过 ref 调用，避免 stale closure
- **禁止** 为了消除 ESLint exhaustive-deps 警告而把会变化的函数直接加入空依赖数组，这会导致 Effect 频繁重注册

```typescript
// ✅ 正确：用 ref 保存最新回调，在空依赖 useEffect 中通过 ref 调用
const onMessageRef = useRef(onMessage);
useEffect(() => { onMessageRef.current = onMessage; });

useEffect(() => {
    const handler = (msg: Msg) => {
        onMessageRef.current(msg); // 始终调用最新版本
    };
    socket.on('event', handler);
    return () => socket.off('event', handler);
}, []); // 空依赖，只注册一次

// ❌ 错误：onMessage 是闭包捕获的旧版本，导致 stale closure
useEffect(() => {
    const handler = (msg: Msg) => {
        onMessage(msg); // 始终是首次渲染时的旧函数
    };
    socket.on('event', handler);
    return () => socket.off('event', handler);
}, []);
```

---

### 8.2 memo 优化：将 prop 拆分到最细粒度，避免父级整体对象更新引发批量重渲染

- **必须遵循** 传给 `memo` 子组件的 prop 应精确到最小单元，禁止直接传整个大对象（如 `Record<string, T>`）再在子组件内部取值，否则大对象任意字段变化都会导致所有子组件重渲染
- **推荐** 父组件用索引/key 从大对象中取出对应的小数据，只把小数据作为 prop 传入

```typescript
// ✅ 正确：只传当前 item 对应的 state，其他 item 不受影响
const List = memo(({ items, stateMap }: Props) => (
    <>
        {items.map(item => (
            <ListItem
                key={item.id}
                item={item}
                itemState={stateMap[item.id]} // 只取该 item 的数据
            />
        ))}
    </>
));

// ❌ 错误：直接传 stateMap 整体，stateMap 任意字段更新都会重渲染所有 ListItem
const List = memo(({ items, stateMap }: Props) => (
    <>
        {items.map(item => (
            <ListItem key={item.id} item={item} stateMap={stateMap} />
        ))}
    </>
));
```

---

### 8.3 setXxx 中使用 changed flag 避免无效 re-render

- **必须遵循** 在 `useEffect` 内的 `setState(prev => ...)` 回调中，若逻辑结果未改变数据，必须返回原始 `prev` 引用而非新对象，以避免触发不必要的重渲染
- **推荐** 使用 `changed` 布尔标志追踪是否有实际修改，无修改时直接 `return prev`

```typescript
// ✅ 正确：无变化时返回原始 prev，不触发 re-render
useEffect(() => {
    setItems(prev => {
        let changed = false;
        const next = prev.map(item => {
            const newLabel = dataMap[item.id]?.label;
            if (newLabel && newLabel !== item.label) {
                changed = true;
                return { ...item, label: newLabel };
            }
            return item;
        });
        return changed ? next : prev; // 无变化则返回原引用
    });
}, [dataMap]);

// ❌ 错误：始终返回新数组，dataMap 每次更新都触发 re-render
useEffect(() => {
    setItems(prev =>
        prev.map(item => ({
            ...item,
            label: dataMap[item.id]?.label ?? item.label, // 即使没变化也返回新对象
        }))
    );
}, [dataMap]);
```

---

### 8.5 useEffect 依赖项谨慎选择：网络/连接状态变化不应触发业务重置

- **必须遵循** `useEffect` 的依赖数组只放「用户主动操作」产生的变化，WebSocket 重连、connectionState、threadId 等网络层副产物不应作为业务逻辑重置的依赖项
- **原因** WebSocket 断线重连等非用户操作会导致 threadId 等值变化，若其在依赖数组中，会意外触发状态清空、重置等副作用
- **推荐** 将「用户切换 Tab」与「连接重连导致 id 变化」在依赖数组层面区分开，只依赖真正代表用户意图的 id（如 `activeTabId`）

```typescript
// ✅ 正确：只依赖用户主动切换的 activeTabId，重连导致的 threadId 变化不会误触发
useEffect(() => {
    autoSentRef.current = false; // 用户切换 tab 时才重置
}, [activeTabId]);

// ❌ 错误：threadId 随 WS 重连变化，导致 autoSentRef 被误重置、开场消息重复发送
useEffect(() => {
    autoSentRef.current = false;
}, [threadId, activeTabId]); // threadId 不代表用户意图，不应在此
```

---

### 8.6 防御性编程：从 URL 参数恢复状态时优先判断场景类型

- **必须遵循** 初始化组件/Tab 时，若 URL 已携带关键参数（如 `threadId`），应判定为「恢复已有会话」而非「新建」，不得将其标记为 `isNewTab = true`
- **必须遵循** 判断逻辑：先读 URL 参数 → 有则为「恢复」，无则为「新建」，不得跳过此检查

```typescript
// ✅ 正确：检查 URL 是否已携带 threadId，据此决定 isNewTab
const params = new URLSearchParams(window.location.search);
const existingThreadId = params.get('threadId');
const isNewTab = !existingThreadId; // URL 已有 threadId → 恢复会话，非新建

// ❌ 错误：无论 URL 是否携带 threadId，统一标记为新 tab，导致已有会话被清空
const isNewTab = true; // 未检查 URL 状态，恢复场景下会错误触发开场消息
```

---

### 8.7 自动触发操作必须同时校验标记位与实际数据状态

- **必须遵循** 自动发送消息、自动执行等操作，必须同时满足：① 标记位为真（`isNewTab`、`autoSentRef` 等）；② 实际数据状态满足条件（如 `!hasMessages`、列表为空等），二者缺一不可
- **原因** 单独依赖标记位在重连、热重载、组件重挂等边缘场景下可能失效；只依赖实际状态则可能在已有数据时误触发
- **推荐** 将多重条件组合为一个 `canAutoSend` 变量，使逻辑清晰可读

```typescript
// ✅ 正确：标记位 + 实际状态双重防护
const canAutoSend = isNewTab && !autoSentRef.current && !hasMessages;
if (canAutoSend) {
    autoSentRef.current = true;
    sendWelcomeMessage();
}

// ❌ 错误：只检查标记位，重连后 hasMessages 已有数据但仍会重复发送
if (isNewTab && !autoSentRef.current) {
    autoSentRef.current = true;
    sendWelcomeMessage(); // 已有消息时仍触发，导致开场消息重复
}
```

---

### 8.4 Demo 布局对齐规范

- **必须遵循** 任务中提供了 Demo HTML 文件时，实现前必须先分析 Demo 的完整 DOM 层级结构，确保组件的嵌套关系和位置层级与 Demo 一致
- **必须遵循** 特别关注以下布局关键点：组件属于哪个父容器（全局级 vs 区域级）、组件在兄弟节点中的顺序（第一行 vs 嵌套在内部）、组件的宽度范围（横跨全宽 vs 局部宽度）
- **禁止** 仅关注"功能逻辑是否正确"而忽略"组件在 DOM 树中的位置是否与 Demo 一致"

```tsx
// Demo HTML 结构分析：
// <div class="app-window">         ← 最外层
//   <div class="global-bar">       ← 全局栏在最外层第一行，横跨全宽
//   <div class="app-body">         ← 主体（侧边栏 + 内容区 + 面板）

// ✅ 正确：全局栏放在最外层容器第一行，横跨全宽
<div className="flex h-full flex-col">
    <GlobalBar />                     {/* 第一行：全宽 */}
    <div className="flex flex-1">     {/* 第二行：主体区域 */}
        <Sidebar />
        <ContentArea />
        <Panel />
    </div>
</div>

// ❌ 错误：全局栏嵌套在中间区域内部，只占局部宽度
<div className="flex h-full">
    <Sidebar />
    <div className="flex flex-1 flex-col">
        <GlobalBar />                 {/* 错误！全局栏在内部区域，非全宽 */}
        <ContentArea />
    </div>
    <Panel />
</div>
```
