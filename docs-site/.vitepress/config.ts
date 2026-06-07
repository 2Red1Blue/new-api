import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'laiber.cloud Docs',
  description: 'laiber.cloud AI 编程与 API 中转服务使用文档',
  cleanUrls: true,
  lastUpdated: true,
  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      link: '/zh/',
      themeConfig: {
        nav: [{ text: '进入中文文档', link: '/zh/' }],
      },
    },
    zh: {
      label: '简体中文',
      lang: 'zh-CN',
      title: 'laiber.cloud 使用文档',
      description: 'AI 编程与 API 中转服务使用指南',
      themeConfig: {
        outline: {
          level: [2, 3],
          label: '本页目录',
        },
        nav: [
          { text: '首页', link: '/zh/' },
          { text: '使用指南', link: '/zh/guide/registration' },
          { text: '配置工具', link: '/zh/tools/cc-switch' },
          {
            text: '部署指南',
            items: [
              { text: 'Claude Code', link: '/zh/deploy/claude-code' },
              { text: 'Codex', link: '/zh/deploy/codex' },
            ],
          },
          {
            text: '第三方应用',
            items: [
              { text: 'OpenAI 兼容', link: '/zh/clients/openai-compatible' },
              { text: 'Cherry Studio', link: '/zh/clients/cherry-studio' },
            ],
          },
          { text: '支持与 FAQ', link: '/zh/support/faq' },
        ],
        sidebar: {
          '/zh/': [
            {
              text: '快速开始',
              collapsed: false,
              items: [
                { text: '欢迎使用', link: '/zh/intro/welcome' },
                { text: '什么是 API 中转站', link: '/zh/intro/overview' },
              ],
            },
            {
              text: '使用指南',
              collapsed: false,
              items: [
                { text: '注册账号', link: '/zh/guide/registration' },
                { text: '创建专属 Key', link: '/zh/guide/create-key' },
                { text: '修改令牌设置', link: '/zh/guide/modify-token' },
                { text: '模型选择', link: '/zh/guide/model-selection' },
                { text: '费用与用量', link: '/zh/guide/usage-and-billing' },
              ],
            },
            {
              text: '快速配置工具',
              collapsed: false,
              items: [
                { text: 'CC-Switch 配置工具', link: '/zh/tools/cc-switch' },
              ],
            },
            {
              text: '部署指南',
              collapsed: false,
              items: [
                { text: 'Claude Code', link: '/zh/deploy/claude-code' },
                { text: 'Codex', link: '/zh/deploy/codex' },
              ],
            },
            {
              text: '第三方应用',
              collapsed: false,
              items: [
                { text: 'OpenAI 兼容接入', link: '/zh/clients/openai-compatible' },
                { text: 'Cherry Studio', link: '/zh/clients/cherry-studio' },
              ],
            },
            {
              text: '支持',
              collapsed: false,
              items: [
                { text: '排障指南', link: '/zh/support/troubleshooting' },
                { text: '常见问题', link: '/zh/support/faq' },
              ],
            },
          ],
        },
        footer: {
          message: '稳定、清晰、方便接入',
          copyright: 'Copyright © 2026 laiber.cloud',
        },
        docFooter: {
          prev: '上一页',
          next: '下一页',
        },
        lastUpdatedText: '最后更新',
        returnToTopLabel: '返回顶部',
        darkModeSwitchLabel: '外观',
        sidebarMenuLabel: '菜单',
      },
    },
  },
  themeConfig: {
    search: {
      provider: 'local',
      options: {
        locales: {
          zh: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档',
              },
              modal: {
                noResultsText: '没有找到相关内容',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                },
              },
            },
          },
        },
      },
    },
    socialLinks: [],
  },
})
