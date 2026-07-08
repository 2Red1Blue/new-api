# web/default 默认前端实现学习指南

这份文档解释 new-api 当前默认前端 `web/default` 的实现。虽然主项目后端是 Go，但默认管理后台是理解整个系统不可缺的一半：用户登录、API Key 管理、渠道管理、用户管理、系统设置、日志查询等工作流，都会从这里发起请求，再落到 Go 后端的 `/api/...`。

## 一、技术栈与启动入口

默认前端位于：

```text
web/default/
```

核心技术栈：

- React 19
- TypeScript
- Rsbuild
- TanStack Router
- TanStack Query
- TanStack Table
- Zustand
- Axios
- i18next / react-i18next
- Base UI / Tailwind CSS

常用命令来自 `web/default/package.json`：

```bash
cd web/default
bun run dev
bun run build
bun run build:check
bun run typecheck
bun run lint
bun run i18n:sync
```

其中：

- `dev`: 启动 Rsbuild 开发服务器。
- `build`: 生产构建。
- `build:check`: 先 TypeScript 检查，再构建。
- `typecheck`: 只跑类型检查。
- `lint`: 使用 oxlint。
- `i18n:sync`: 同步 locale key。

Rsbuild 配置在 `web/default/rsbuild.config.ts`。入口文件是：

```text
web/default/src/main.tsx
```

开发代理会把 `/api`、`/mj`、`/pg` 等请求转发到后端，默认目标通常是 `http://localhost:3000`。这意味着前端开发时不需要在代码里写死后端完整域名。

## 二、目录主线

推荐先按这条线读：

```text
src/main.tsx
  -> src/routes/__root.tsx
  -> src/routes/_authenticated/route.tsx
  -> src/lib/api.ts
  -> src/stores/auth-store.ts
  -> src/features/*
  -> src/components/data-table/*
  -> src/i18n/*
```

主要目录：

| 路径 | 作用 |
| --- | --- |
| `src/main.tsx` | React 应用入口，创建 QueryClient、Router，挂载全局 Provider |
| `src/routes/` | TanStack Router 文件路由 |
| `src/routeTree.gen.ts` | 路由树自动生成文件 |
| `src/features/` | 业务模块，例如 auth、keys、channels、users、system-settings |
| `src/components/` | 通用组件 |
| `src/components/data-table/` | 管理后台表格体系 |
| `src/hooks/` | 复用 hooks |
| `src/lib/api.ts` | Axios 请求封装和后端 API 函数 |
| `src/stores/auth-store.ts` | Zustand 登录态 |
| `src/i18n/` | 前端多语言配置和 locale 文件 |

前端整体是典型的“路由 + feature + shared components”结构：

```text
路由文件负责页面入口、权限、search params
feature 负责业务 UI 和 API 调用组织
components 负责通用 UI 能力
lib/api.ts 负责请求封装
stores 负责跨页面状态
```

## 三、应用启动流程

`src/main.tsx` 做几件事：

1. 创建 `QueryClient`。
2. 创建 TanStack Router。
3. 挂载主题、字体、方向、路由等 Provider。
4. 配置 React Query 全局错误处理和重试策略。

可以把前端启动理解成：

```text
ReactDOM.createRoot
  -> QueryClientProvider
  -> ThemeProvider / FontProvider / DirectionProvider
  -> RouterProvider
  -> route tree
  -> page component
```

React Query 的全局配置很重要：

- 401/403 通常不重试。
- `refetchOnWindowFocus=false`，避免窗口聚焦时无意义刷新。
- mutation 出错会走统一错误处理。
- QueryCache 遇到 401 会清理登录态并跳登录页。

这和后端的 session 机制配套：前端本地有用户缓存，不代表服务端 session 一定还有效，所以受保护路由会验证一次。

## 四、路由与登录态

前端使用 TanStack Router 文件路由。核心文件：

```text
src/routes/__root.tsx
src/routes/_authenticated/route.tsx
src/routes/(auth)/sign-in.tsx
```

### 1. 根路由

`__root.tsx` 是全局外壳。它通常负责：

- setup 状态检查。
- 全局系统配置加载。
- 顶部进度条。
- Toast。
- Devtools。

它不强制每次都请求 `getSelf()`，因为登录态主要由本地 Zustand 缓存和 `_authenticated` 守卫共同维护。

### 2. 受保护路由

`src/routes/_authenticated/route.tsx` 是后台页面的守卫。

逻辑大致是：

```text
beforeLoad
  -> 从 useAuthStore 读取 auth.user
  -> 没有用户：redirect('/sign-in')
  -> 有用户但本会话还没验证：调用 getSelf()
  -> 成功：auth.setUser()
  -> 401：auth.reset() + redirect('/sign-in')
  -> 网络错误/5xx：暂时放行，下次导航再验
```

这里有一个 `sessionVerified` 内存标记，用来避免同一浏览器会话中重复验证。

对 Go 后端读者来说，这里要和后端中间件对应起来：

```text
前端 getSelf()
  -> /api/user/self
  -> 后端 UserAuth()
  -> session 或 access token 校验
  -> 返回当前用户
```

### 3. 登录页

`src/routes/(auth)/sign-in.tsx` 负责登录入口。

核心流程：

```text
sign-in route
  -> validateSearch 校验 redirect
  -> UserAuthForm
  -> login()
  -> useAuthRedirect.handleLoginSuccess()
  -> getSelf()
  -> auth.setUser()
  -> 恢复用户语言
  -> navigate(redirect 或 /dashboard)
```

登录表单还接入 Passkey、OAuth、微信等能力，但主线仍是后端 `/api/user/login` 写 session，前端再拉当前用户。

## 五、API 请求封装

统一请求入口是：

```text
src/lib/api.ts
```

这里创建了一个 Axios 实例：

```text
baseURL = ''
withCredentials = true
Cache-Control = no-store
```

`withCredentials=true` 很关键，因为后端登录态依赖 Cookie Session。没有它，跨域开发或代理场景下可能带不上 cookie。

### 1. 并发 GET 去重

`api.ts` 覆盖了 `api.get`：

```text
相同 url + params 的 GET 如果已经在飞
  -> 复用同一个 Promise
  -> 请求结束后从 inFlightGet 删除
```

这个设计适合后台页面：系统配置、用户信息、列表数据可能被多个组件同时请求，去重可以减少重复 HTTP 请求。

如果某个请求必须绕过去重，可以传：

```ts
{ disableDuplicate: true }
```

### 2. `New-Api-User` header

请求拦截器会从 localStorage 读取 `uid`，然后加 header：

```text
New-Api-User: <uid>
```

后端 `authHelper` 会检查这个 header 是否和 session/access token 对应用户一致。这是防止错用本地缓存或跨用户状态污染的一层保护。

### 3. 业务错误处理

后端普通 API 多数返回统一结构：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

响应拦截器看到 `success: false` 会 toast 错误消息。可以通过：

```ts
{ skipBusinessError: true }
{ skipErrorHandler: true }
```

控制是否跳过默认处理。

### 4. 401 处理

如果响应是 401：

```text
useAuthStore.getState().auth.reset()
toast "Session expired!"
Promise.reject(error)
```

React Query 的全局错误处理也会配合清理登录态和跳登录页。

## 六、管理页数据流

典型管理页可以按这个模板理解：

```text
route validateSearch / beforeLoad 权限守卫
  -> feature page component
  -> useTableUrlState 同步 URL 查询参数
  -> useQuery 拉后端列表 API
  -> useDataTable 创建 TanStack Table 实例
  -> DataTablePage 渲染工具栏、表格、移动列表、分页
  -> mutation 修改数据
  -> invalidate/refetch 刷新
```

### 1. 用户管理

入口：

```text
src/routes/_authenticated/users/index.tsx
src/features/users/components/users-table.tsx
```

重点：

- 路由要求管理员角色。
- 表格用 URL search 保存分页、搜索和筛选。
- `getUsers()` / `searchUsers()` 拉后端数据。
- mutation 后刷新列表。

### 2. 渠道管理

入口：

```text
src/routes/_authenticated/channels/index.tsx
src/features/channels/components/channels-table.tsx
```

渠道页是后台里最复杂的列表之一，包含：

- URL 筛选。
- 排序。
- 标签聚合。
- 桌面表格和移动卡片。
- 批量选择。
- 列可见性持久化。
- 渠道测试、复制、编辑、删除等操作。

读前端时可以用渠道页作为综合案例，因为它同时使用了表格、权限、弹窗、mutation、toast、i18n。

### 3. API Key 管理

入口：

```text
src/features/keys/components/api-keys-table.tsx
```

它会调用：

```text
getApiKeys()
searchApiKeys()
```

并额外实现移动端 API Key 列表。后端对应的是 `controller/token.go` 和 `model/token.go`。

### 4. 系统设置

入口：

```text
src/routes/_authenticated/system-settings/route.tsx
src/features/system-settings/components/settings-page.tsx
```

系统设置页要求更高权限，核心 hooks 包括：

- `useSystemOptions()`
- `useSettingsForm()`
- `useUpdateOption()`

数据流是：

```text
拉取 options
  -> 映射到表单
  -> 用户修改
  -> 只提交脏字段
  -> 后端更新 option
  -> 刷新配置缓存
```

## 七、DataTable 体系

后台大量页面都是列表，所以 `src/components/data-table/` 是前端的核心基础设施。

主要组件和 hooks：

| 文件/对象 | 作用 |
| --- | --- |
| `useDataTable` | 封装 `useReactTable`，支持分页、筛选、排序、列宽、列显示 |
| `DataTablePage` | 统一页面结构，渲染 toolbar、桌面表格、移动列表、批量操作、分页 |
| `DataTableToolbar` | 搜索、faceted filter、重置、展开筛选、视图选项 |
| `useTableUrlState` | 把分页、搜索、筛选映射到 URL search params |

这里最值得学习的是“URL 作为状态源”：

```text
用户改筛选
  -> 写入 URL search
  -> 路由状态变化
  -> queryKey 变化
  -> useQuery 重新拉数据
  -> 表格渲染新结果
```

这样做的好处：

- 页面可刷新恢复。
- 链接可分享。
- 浏览器前进后退自然可用。
- 搜索/筛选状态不需要额外全局 store。

## 八、i18n

前端 i18n 在：

```text
src/i18n/config.ts
src/i18n/languages.ts
src/i18n/locales/{en,zh,fr,ja,ru,vi}.json
```

特点：

- 使用 i18next。
- `fallbackLng` 是 `en`。
- 通过 `LanguageDetector` 从 localStorage 和浏览器语言检测。
- locale JSON 是 flat key。
- 用户可见文案应该通过 `t('English key')`。

语言切换组件：

```text
src/components/language-switcher.tsx
```

登录用户切换语言时，会 best-effort 调后端保存偏好。同步脚本：

```bash
bun run i18n:sync
```

读前端源码时，如果看到裸字符串，要判断它是不是用户可见文案；如果是，应该接入 `useTranslation()`。

## 九、React/TypeScript 学习点

通过 `web/default` 可以学习这些现代前端模式：

1. TanStack Router 文件路由。
2. `beforeLoad` 做权限和重定向。
3. `zod` 校验 search params。
4. React Query 管理服务端状态。
5. Zustand 管理本地登录态。
6. Axios interceptor 做统一请求行为。
7. React Hook Form + Zod 做表单。
8. TanStack Table 泛型表格。
9. URL search params 作为列表状态。
10. i18next 多语言。

对 Go 后端学习者来说，建议你把每个前端页面都映射到后端 API：

```text
前端 route / feature
  -> src/lib/api.ts 中的函数
  -> router/api-router.go
  -> controller
  -> service/model
```

这样读，前后端会连成一张图，而不是两个孤立项目。

