import { Link, useLocation } from '@tanstack/react-router'
import { useEffect, useState, useRef } from 'react'
import { Modal, Tag, Button, Spin, message } from 'antd'

const mainNavItems = [
  { icon: 'home', label: 'Home', to: '/' },
  { icon: 'add_circle', label: 'Creation', to: '/new-requirement' },
  { icon: 'schema', label: 'Workflows', to: '/workflows' },
  { icon: 'monitoring', label: 'Monitoring', to: '/monitoring' },
  { icon: 'fact_check', label: 'Approvals', to: '/approvals' },
  { icon: 'local_shipping', label: 'Delivery', to: '/delivery' },
  { icon: 'inventory', label: 'Assets', to: '/assets' },
]

interface User {
  id: string
  name: string
  avatarUrl: string
  departments: string[]
  // GitHub 绑定信息
  githubId?: string
  githubLogin?: string
  githubAvatar?: string
}

interface Session {
  id: string
  title: string
  summary: string
  status: string
  ownerName: string
  messageCount: number
  updatedAt: string
}

interface SidebarProps {
  convCollapsed?: boolean
  onConvCollapse?: (collapsed: boolean) => void
}

export default function Sidebar({ convCollapsed = false, onConvCollapse }: SidebarProps) {
  const location = useLocation()
  const pathname = location.pathname
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [showMenu, setShowMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const [sessions, setSessions] = useState<Session[]>([])
  const [sessionsLoading, setSessionsLoading] = useState(true)
  const [settingsModalVisible, setSettingsModalVisible] = useState(false)
  const [userBindings, setUserBindings] = useState<{feishu?: {openId: string, name: string}, github?: {id: string, login: string}}>({})
  const [loadingBindings, setLoadingBindings] = useState(false)

  // 在主页和会话详情页均显示会话面板
  const showConversationPanel = pathname === '/' || pathname.startsWith('/sessions')

  const isActive = (to: string) => pathname === to || (to !== '/' && pathname.startsWith(to))

  const toggleConvCollapse = () => {
    onConvCollapse?.(!convCollapsed)
  }

  // 获取当前用户状态
  useEffect(() => {
    fetch('/api/me', { credentials: 'include' })
      .then(res => {
        if (res.ok) return res.json()
        throw new Error('Not logged in')
      })
      .then(data => {
        if (data.data) setUser(data.data)
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  // 获取会话列表
  useEffect(() => {
    fetch('/api/sessions', { credentials: 'include' })
      .then(res => {
        if (res.ok) return res.json()
        throw new Error('Failed to load sessions')
      })
      .then(data => {
        if (data.data) setSessions(data.data)
      })
      .catch(() => setSessions([]))
      .finally(() => setSessionsLoading(false))
  }, [])

  // 点击外部关闭菜单
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // 跳转到飞书登录 - 先获取配置再构建授权 URL
  const handleLogin = async () => {
    try {
      const res = await fetch('/api/auth/feishu/config')
      if (!res.ok) throw new Error('Failed to get config')
      const data = await res.json()
      const config = data.data
      if (!config?.enabled) { alert('飞书登录未启用'); return }
      const state = Math.random().toString(36).substring(2)
      sessionStorage.setItem('feishu_auth_state', state)
      const redirectUri = `${window.location.origin}/auth/callback`
      const authUrl = `https://open.feishu.cn/open-apis/authen/v1/authorize?` +
        `app_id=${config.appId}&` +
        `redirect_uri=${encodeURIComponent(redirectUri)}&` +
        `state=${state}`
      window.location.href = authUrl
    } catch (err) {
      console.error('Login failed:', err)
      alert('获取登录配置失败')
    }
  }

  // 登出
  const handleLogout = async () => {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'include'
      })
      setUser(null)
      setShowMenu(false)
    } catch (err) {
      console.error('Logout failed:', err)
    }
  }

  // 打开设置弹窗
  const openSettings = async () => {
    setShowMenu(false)
    setSettingsModalVisible(true)
    await loadUserBindings()
  }

  // 加载账号绑定信息
  const loadUserBindings = async () => {
    setLoadingBindings(true)
    try {
      // 获取当前用户信息来解析绑定的账号类型
      const res = await fetch('/api/me', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        const userData = data.data
        const bindings: typeof userBindings = {}

        // 飞书登录用户（id 以 fs_ 开头）
        if (userData?.id?.startsWith('fs_')) {
          bindings.feishu = {
            openId: userData.feishuOpenID || userData.id.replace('fs_', ''),
            name: userData.name || '飞书用户'
          }
          // 检查是否绑定了 GitHub
          if (userData.githubId) {
            bindings.github = {
              id: userData.githubId,
              login: userData.githubLogin || 'GitHub用户'
            }
          }
        }
        // GitHub 登录用户（id 以 gh_ 开头）
        else if (userData?.id?.startsWith('gh_')) {
          bindings.github = {
            id: userData.id.replace('gh_', ''),
            login: userData.name || 'GitHub用户'
          }
        }

        setUserBindings(bindings)
      }
    } catch (err) {
      console.error('加载账号绑定信息失败:', err)
    } finally {
      setLoadingBindings(false)
    }
  }

  // 解绑飞书
  const handleUnbindFeishu = () => {
    message.info('请联系管理员解绑飞书账号')
  }

  // 解绑 GitHub
  const handleUnbindGitHub = async () => {
    try {
      const res = await fetch('/api/auth/github/unbind', {
        method: 'POST',
        credentials: 'include',
      })
      if (res.ok) {
        message.success('GitHub 解绑成功')
        loadUserBindings()
      } else {
        message.error('解绑失败，请重试')
      }
    } catch (err) {
      console.error('解绑 GitHub 失败:', err)
      message.error('解绑 GitHub 账号失败')
    }
  }

  // 绑定飞书
  const handleBindFeishu = async () => {
    try {
      const res = await fetch('/api/auth/feishu/config')
      if (!res.ok) throw new Error('Failed to get config')
      const data = await res.json()
      const config = data.data
      if (!config?.enabled) { message.error('飞书登录未启用'); return }
      
      const state = Math.random().toString(36).substring(2)
      sessionStorage.setItem('feishu_auth_state', state)
      const redirectUri = `${window.location.origin}/auth/callback`
      const authUrl = `https://open.feishu.cn/open-apis/authen/v1/authorize?` +
        `app_id=${config.appId}&` +
        `redirect_uri=${encodeURIComponent(redirectUri)}&` +
        `state=${state}`
      window.location.href = authUrl
    } catch (err) {
      console.error('绑定飞书失败:', err)
      message.error('绑定飞书账号失败')
    }
  }

  // 绑定 GitHub - 前端直接跳转 GitHub 授权，回调到前端页面
  const handleBindGitHub = async () => {
    try {
      const clientId = 'Ov23ligSIYJehzkAIv5R'
      const redirectUri = `${window.location.origin}/auth/callback`
      const state = Math.random().toString(36).substring(2)
      sessionStorage.setItem('github_bind_mode', 'true')
      sessionStorage.setItem('github_auth_state', state)

      // 构建 GitHub 授权 URL，回调到前端 AuthCallback 页面
      const authUrl = `https://github.com/login/oauth/authorize?client_id=${clientId}&redirect_uri=${encodeURIComponent(redirectUri)}&scope=read:user,user:email,repo&state=${state}`
      window.location.href = authUrl
    } catch (err) {
      console.error('绑定 GitHub 失败:', err)
      message.error('绑定 GitHub 账号失败')
    }
  }

  return (
    <>
      {/* COLUMN 1: Global Navigation (SideNavBar - main_nav) - 固定宽度 80px */}
      <aside className="fixed left-0 top-0 h-full flex flex-col z-40 bg-slate-50 dark:bg-slate-900 w-20 border-r border-slate-200/50 dark:border-slate-800/50">
        <div className="flex items-center justify-center h-20">
          <span className="text-xl font-bold tracking-tight text-blue-800 dark:text-blue-300">AF</span>
        </div>
        <nav className="flex-1 flex flex-col items-center py-4 space-y-6">
          {mainNavItems.map((item) => {
            const active = isActive(item.to)
            return (
              <Link
                key={item.label}
                to={item.to}
                title={item.label}
                className={`relative p-3 rounded-xl transition-all duration-150 ${
                  active
                    ? 'bg-blue-100/50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 after:content-[""] after:absolute after:left-0 after:w-1 after:h-8 after:bg-blue-600 after:rounded-r-full scale-95'
                    : 'text-slate-500 dark:text-slate-400 hover:bg-slate-200/50 dark:hover:bg-slate-800/50'
                }`}
              >
                <span className="material-symbols-outlined">{item.icon}</span>
              </Link>
            )
          })}
        </nav>
        <div className="pb-8 flex flex-col items-center space-y-4 relative">
          <button className="text-slate-500 dark:text-slate-400 hover:bg-slate-200/50 dark:hover:bg-slate-800/50 transition-colors p-3 rounded-xl">
            <span className="material-symbols-outlined">settings</span>
          </button>
          
          {/* 用户头像 / 登录按钮 */}
          <div ref={menuRef} className="relative">
            {loading ? (
              // 加载中
              <div className="w-10 h-10 rounded-full bg-surface-container-highest flex items-center justify-center border border-outline-variant/30">
                <span className="material-symbols-outlined text-slate-400 text-sm animate-spin">progress_activity</span>
              </div>
            ) : user ? (
              // 已登录 - 显示头像
              <button
                onClick={() => setShowMenu(!showMenu)}
                className="w-10 h-10 rounded-full bg-surface-container-highest flex items-center justify-center overflow-hidden border border-outline-variant/30 hover:ring-2 hover:ring-primary/30 transition-all"
              >
                {user.avatarUrl ? (
                  <img src={user.avatarUrl} alt={user.name} className="w-full h-full object-cover" />
                ) : (
                  <span className="material-symbols-outlined text-slate-500">person</span>
                )}
              </button>
            ) : (
              // 未登录 - 显示登录按钮
              <button
                onClick={handleLogin}
                className="w-10 h-10 rounded-full bg-primary-container flex items-center justify-center border border-primary/20 hover:bg-primary transition-all group"
                title="Login with Feishu"
              >
                <span className="material-symbols-outlined text-on-primary-container group-hover:text-white text-sm">login</span>
              </button>
            )}

            {/* 设置菜单 */}
            {showMenu && user && (
              <div className="absolute bottom-full left-0 mb-2 w-56 bg-white dark:bg-slate-800 rounded-xl shadow-lg border border-slate-200 dark:border-slate-700 py-2 animate-fade-in">
                <div className="px-4 py-3 border-b border-slate-100 dark:border-slate-700">
                  <p className="text-sm font-medium text-slate-900 dark:text-slate-100 truncate">{user.name}</p>
                  <p className="text-xs text-slate-500 truncate">{user.departments?.length > 0 ? user.departments[0] : user.id}</p>
                </div>
                <button
                  onClick={openSettings}
                  className="w-full px-4 py-2.5 text-left text-sm text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700/50 flex items-center gap-3 transition-colors"
                >
                  <span className="material-symbols-outlined text-base">settings</span>
                  设置
                </button>
                <button
                  className="w-full px-4 py-2.5 text-left text-sm text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700/50 flex items-center gap-3 transition-colors"
                >
                  <span className="material-symbols-outlined text-base">help</span>
                  帮助与反馈
                </button>
                <div className="border-t border-slate-100 dark:border-slate-700 mt-1 pt-1">
                  <button
                    onClick={handleLogout}
                    className="w-full px-4 py-2.5 text-left text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 flex items-center gap-3 transition-colors"
                  >
                    <span className="material-symbols-outlined text-base">logout</span>
                    退出登录
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* COLUMN 2: Contextual Navigation (SideNavBar - sub_nav) - 只在主页显示，可折叠 */}
      {showConversationPanel && (
        <>
          {/* 折叠/展开按钮 - 固定在左侧导航旁边 */}
          <button
            onClick={toggleConvCollapse}
            title={convCollapsed ? '展开会话' : '收起会话'}
            className={`fixed top-1/2 -translate-y-1/2 w-6 h-12 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-full flex items-center justify-center shadow-md hover:shadow-lg transition-all z-50 ${convCollapsed ? 'left-20' : 'left-[296px]'}`}
          >
            <span className="material-symbols-outlined text-xs text-slate-500">
              {convCollapsed ? 'chevron_right' : 'chevron_left'}
            </span>
          </button>
          
          {/* Conversations 栏 */}
          <aside className={`fixed left-20 top-0 h-full flex flex-col z-30 bg-slate-100/80 dark:bg-slate-950/80 backdrop-blur-xl border-r border-slate-200/30 dark:border-slate-800/30 transition-all duration-300 ${convCollapsed ? 'w-0 opacity-0 overflow-hidden' : 'w-64 opacity-100'}`}>
            <div className="px-6 h-20 flex flex-col justify-center flex-shrink-0">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">会话</h2>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-slate-500">最近 7 天</p>
            </div>
            <div className="px-4 mb-6 flex-shrink-0">
              <Link to="/new-requirement">
                <button className="w-full py-3 px-4 flex items-center justify-center gap-2 bg-primary-container text-on-primary-container rounded-xl shadow-sm hover:shadow-md transition-all duration-300 group">
                  <span className="material-symbols-outlined text-sm">add</span>
                  <span className="text-sm font-semibold">新建会话</span>
                </button>
              </Link>
            </div>
            <nav className="flex-1 overflow-y-auto space-y-1 min-w-0">
              <div className="px-6 mb-2">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">最近会话</span>
              </div>
              {sessionsLoading ? (
                <div className="px-6 py-4 text-center">
                  <span className="material-symbols-outlined text-slate-400 text-sm animate-spin">progress_activity</span>
                </div>
              ) : sessions.length === 0 ? (
                <div className="px-6 py-4 text-xs text-slate-400 text-center">
                  暂无会话
                </div>
              ) : (
                sessions.map((session) => {
                  const isActive = pathname === `/sessions/${session.id}`
                  return (
                    <a
                      key={session.id}
                      href={`/sessions/${session.id}`}
                      className={`px-3 py-2 mx-2 flex items-center gap-3 cursor-pointer group transition-all duration-200 rounded-lg ${
                        isActive
                          ? 'bg-white dark:bg-slate-800 text-blue-700 dark:text-blue-300 shadow-sm'
                          : 'text-slate-600 dark:text-slate-400 hover:bg-white/50 dark:hover:bg-slate-800/50'
                      }`}
                    >
                      <span className={`material-symbols-outlined text-sm ${isActive ? 'opacity-70' : 'opacity-40'}`}>
                        {isActive ? 'chat_bubble' : 'history'}
                      </span>
                      <span className="text-sm font-medium truncate">{session.title}</span>
                    </a>
                  )
                })
              )}
            </nav>
            <div className="p-4 bg-white/40 dark:bg-slate-900/40 border-t border-slate-200/20 flex-shrink-0">
              <div className="flex items-center gap-3 text-xs text-slate-500">
                <span className="material-symbols-outlined text-sm">cloud_done</span>
                <span>All changes synced</span>
              </div>
            </div>
          </aside>
        </>
      )}

      {/* 设置弹窗 */}
      <Modal
        title="账号设置"
        open={settingsModalVisible}
        onCancel={() => setSettingsModalVisible(false)}
        footer={null}
        width={480}
      >
        {loadingBindings ? (
          <div className="flex justify-center py-8">
            <Spin />
          </div>
        ) : (
          <div className="py-4">
            {/* 当前登录方式 */}
            <div className="mb-6">
              <h4 className="text-sm font-medium text-slate-700 mb-3">当前登录方式</h4>
              <div className="bg-slate-50 rounded-lg p-4">
                {userBindings.feishu ? (
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center">
                      <span className="text-white font-bold text-sm">F</span>
                    </div>
                    <div className="flex-1">
                      <div className="font-medium text-slate-800">飞书账号</div>
                      <div className="text-sm text-slate-500">{userBindings.feishu.name}</div>
                    </div>
                    <Tag color="green">已登录</Tag>
                  </div>
                ) : userBindings.github ? (
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-slate-800 flex items-center justify-center">
                      <span className="text-white font-bold text-sm">G</span>
                    </div>
                    <div className="flex-1">
                      <div className="font-medium text-slate-800">{userBindings.github.login}</div>
                      <div className="text-sm text-slate-500">GitHub</div>
                    </div>
                    <Tag color="green">已登录</Tag>
                  </div>
                ) : (
                  <div className="text-slate-500">未绑定任何账号</div>
                )}
              </div>
            </div>

            {/* 账号绑定管理 */}
            <div>
              <h4 className="text-sm font-medium text-slate-700 mb-3">绑定管理</h4>
              <div className="space-y-3">
                {/* 飞书 */}
                <div className="flex items-center justify-between p-3 border border-slate-200 rounded-lg">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded bg-blue-500 flex items-center justify-center">
                      <span className="text-white font-bold text-xs">F</span>
                    </div>
                    <div>
                      <div className="text-sm font-medium text-slate-800">飞书账号</div>
                      {userBindings.feishu ? (
                        <div className="text-xs text-slate-500">已绑定</div>
                      ) : (
                        <div className="text-xs text-slate-400">未绑定</div>
                      )}
                    </div>
                  </div>
                  {userBindings.feishu ? (
                    <Button size="small" danger onClick={handleUnbindFeishu}>解绑</Button>
                  ) : (
                    <Button size="small" type="primary" onClick={handleBindFeishu}>绑定</Button>
                  )}
                </div>

                {/* GitHub */}
                <div className="flex items-center justify-between p-3 border border-slate-200 rounded-lg">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded bg-slate-800 flex items-center justify-center">
                      <span className="text-white font-bold text-xs">G</span>
                    </div>
                    <div>
                      <div className="text-sm font-medium text-slate-800">GitHub 账号</div>
                      {userBindings.github ? (
                        <div className="text-xs text-slate-500">{userBindings.github.login}</div>
                      ) : (
                        <div className="text-xs text-slate-400">未绑定</div>
                      )}
                    </div>
                  </div>
                  {userBindings.github ? (
                    <Button size="small" danger onClick={handleUnbindGitHub}>解绑</Button>
                  ) : (
                    <Button size="small" type="primary" onClick={handleBindGitHub}>绑定</Button>
                  )}
                </div>
              </div>
            </div>

            <div className="mt-6 pt-4 border-t border-slate-100">
              <p className="text-xs text-slate-400">
                解绑操作需要管理员权限，如需解绑请联系管理员。
              </p>
            </div>
          </div>
        )}
      </Modal>
    </>
  )
}
