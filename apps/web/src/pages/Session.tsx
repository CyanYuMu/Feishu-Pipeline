import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { Bubble, Sender } from '@ant-design/x'
import type { BubbleListProps } from '@ant-design/x'
import XMarkdown from '@ant-design/x-markdown'
import { Avatar, Button, message } from 'antd'
import { UserOutlined, RobotOutlined } from '@ant-design/icons'
import Sidebar from '../components/Sidebar'
import { fetchPipelineRuns } from '../lib/pipeline'

interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt: string
}

interface SessionDetail {
  session: {
    id: string
    title: string
    status: string
  }
  messages: Message[]
  tasks: Array<{
    id: string
  }>
}

interface CurrentUser {
  id: string
  name: string
  avatarUrl: string
}

// loading 占位消息 id
const LOADING_MSG_ID = '__loading__'
// 流式输出时 assistant 气泡 id
const STREAMING_MSG_ID = '__streaming__'

export default function Session() {
  const { sessionId } = useParams({ strict: false })
  const [session, setSession] = useState<SessionDetail | null>(null)
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [resolvingApproval, setResolvingApproval] = useState(false)
  const [convCollapsed, setConvCollapsed] = useState(false)
  const sidebarWidth = convCollapsed ? 80 : 336 // 折叠 80，展开 80+256=336
  // 防止首条消息重复发送
  const pendingSentRef = useRef(false)

  // 获取当前登录用户（用于头像）
  useEffect(() => {
    fetch('/api/me', { credentials: 'include' })
      .then(res => res.ok ? res.json() : null)
      .then(data => { if (data?.data) setCurrentUser(data.data) })
      .catch(() => {})
  }, [])

  // 使用 ref 存储 sending 状态，避免闭包问题
  const sendingRef = useRef(sending)
  useEffect(() => {
    sendingRef.current = sending
  }, [sending])

  // 发送消息（抽出独立函数，供手动发送和自动发送复用）
  const sendMessage = useCallback(async (content: string) => {
    if (!content.trim() || sendingRef.current) return

    const triggerAutoPublishCheck = async (assistantContent: string) => {
      if (!sessionId || !assistantContent.trim()) return
      try {
        await fetch(`/api/sessions/${sessionId}/auto-publish-check`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ content: assistantContent }),
        })
      } catch (err) {
        console.error('Failed to run auto publish check:', err)
      }
    }

    // 立即更新 ref，避免并发问题
    sendingRef.current = true
    setSending(true)
    void triggerAutoPublishCheck(content)

    try {
      // 乐观渲染：立即把用户消息 + loading 气泡追加到本地
      setSession(prev => {
        if (!prev) return prev
        return {
          ...prev,
          messages: [
            ...prev.messages,
            { id: `local_${Date.now()}`, role: 'user', content, createdAt: new Date().toISOString() },
            { id: LOADING_MSG_ID, role: 'assistant', content: '', createdAt: new Date().toISOString() },
          ],
        }
      })

      const res = await fetch(`/api/sessions/${sessionId}/messages/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ content }),
      })

      if (!res.ok || !res.body) {
        throw new Error('Stream request failed')
      }

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let accumulated = ''

      // 把 loading 气泡变成真实的 assistant 消息
      setSession(prev => {
        if (!prev) return prev
        return {
          ...prev,
          messages: prev.messages.map(m =>
            m.id === LOADING_MSG_ID
              ? { ...m, id: STREAMING_MSG_ID, content: '' }
              : m
          ),
        }
      })

      let streamDone = false
      while (!streamDone) {
        const { done, value } = await reader.read()
        if (done) {
          break
        }

        const chunk = decoder.decode(value, { stream: true })
        const lines = chunk.split('\n')
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const data = line.slice(6).trim()
          if (data === '[DONE]') {
            streamDone = true
            break
          }
          accumulated += data
          // 逐字更新 assistant 气泡
          setSession(prev => {
            if (!prev) return prev
            return {
              ...prev,
              messages: prev.messages.map(m =>
                m.id === STREAMING_MSG_ID
                  ? { ...m, content: accumulated }
                  : m
              ),
            }
          })
        }
      }

      // 流式结束：移除 streaming 消息，重新添加普通消息
      const finalContent = accumulated
      const finalId = `msg_${Date.now()}`
      setSession(prev => {
        if (!prev) return prev
        return {
          ...prev,
          messages: [
            ...prev.messages.filter(m => m.id !== STREAMING_MSG_ID),
            { id: finalId, role: 'assistant' as const, content: finalContent, createdAt: new Date().toISOString() },
          ],
        }
      })

    } catch (err) {
      console.error('Failed to send message:', err)
      setSession(prev => {
        if (!prev) return prev
        return { ...prev, messages: prev.messages.filter(m => m.id !== LOADING_MSG_ID && m.id !== STREAMING_MSG_ID) }
      })
    } finally {
      sendingRef.current = false
      setSending(false)
    }
  }, [sessionId])

  // 获取会话详情，加载完毕后检查是否有待发消息（从 Home 跳转过来的首条消息）
  useEffect(() => {
    if (!sessionId) return
    fetch(`/api/sessions/${sessionId}`, { credentials: 'include' })
      .then(res => {
        if (res.ok) return res.json()
        throw new Error('Failed to load session')
      })
      .then(data => {
        if (data.data) setSession(data.data)
        // 检查 Home 传来的首条消息
        const pendingKey = `pending_msg_${sessionId}`
        const pendingMsg = sessionStorage.getItem(pendingKey)
        if (pendingMsg && !pendingSentRef.current) {
          pendingSentRef.current = true
          sessionStorage.removeItem(pendingKey)
          // 微任务延迟，确保 setSession 已完成
          setTimeout(() => sendMessage(pendingMsg), 0)
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId])

  // 构建 Bubble.List items
  const bubbleItems: BubbleListProps['items'] = (session?.messages ?? []).map(msg => {
    const isStreaming = msg.id === STREAMING_MSG_ID
    return {
      key: msg.id,
      role: msg.role === 'user' ? 'user' : 'assistant',
      content: msg.content,
      loading: msg.id === LOADING_MSG_ID,
      typing: isStreaming,
    }
  })

  // role 配置（@ant-design/x v2 用 role 单数，每条 item 的 role 字段指向此配置）
  const roleConfig: BubbleListProps['role'] = {
    user: {
      placement: 'end',
      avatar: currentUser?.avatarUrl
        ? <Avatar src={currentUser.avatarUrl} alt={currentUser.name} />
        : <Avatar icon={<UserOutlined />} style={{ background: '#1677ff' }} />,
    },
    assistant: {
      placement: 'start',
      avatar: <Avatar icon={<RobotOutlined />} style={{ background: '#f5f5f5', color: '#555' }} />,
      contentRender: (content) => <XMarkdown>{String(content)}</XMarkdown>,
      typing: sending, // 控制打字动画
    },
  }

  const sidebarProps = { convCollapsed, onConvCollapse: setConvCollapsed }
  const approvalTaskID = session?.tasks?.[0]?.id || ''
  const handleGoToApproval = useCallback(async () => {
    if (!session?.session.id) return
    setResolvingApproval(true)
    try {
      const runs = await fetchPipelineRuns()
      const sessionRuns = runs.filter(item => item.sourceSessionId === session.session.id)
      if (sessionRuns.length === 0) {
        message.warning('当前会话还没有对应的审批流程')
        return
      }
      const targetRun = sessionRuns.find(item => item.status === 'waiting_approval') || sessionRuns[0]
      window.location.assign(`/monitoring?runId=${encodeURIComponent(targetRun.id)}&action=approve`)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '获取审批上下文失败')
    } finally {
      setResolvingApproval(false)
    }
  }, [session?.session.id])

  if (loading) {
    return (
      <div className="min-h-screen bg-background">
        <Sidebar {...sidebarProps} />
        <main className="h-screen flex items-center justify-center" style={{ marginLeft: `${sidebarWidth}px` }}>
          <span className="material-symbols-outlined text-primary text-2xl animate-spin">progress_activity</span>
        </main>
      </div>
    )
  }

  if (!session) {
    return (
      <div className="min-h-screen bg-background">
        <Sidebar {...sidebarProps} />
        <main className="h-screen flex items-center justify-center" style={{ marginLeft: `${sidebarWidth}px` }}>
          <p className="text-on-surface-variant">会话未找到</p>
        </main>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Sidebar {...sidebarProps} />
      <main
        className="h-screen flex flex-col overflow-hidden transition-all duration-300"
        style={{ marginLeft: `${sidebarWidth}px` }}
      >
        {/* Header */}
        <div className="h-14 px-6 flex items-center border-b border-slate-100 bg-white/80 backdrop-blur flex-shrink-0">
          <div className="flex w-full items-center justify-between gap-3">
            <h1 className="text-base font-semibold text-slate-800 truncate">{session.session.title}</h1>
            {approvalTaskID ? (
              <Button
                size="small"
                loading={resolvingApproval}
                onClick={() => void handleGoToApproval()}
              >
                进入审批
              </Button>
            ) : null}
          </div>
        </div>

        {/* 消息区域 */}
        <div className="flex-1 overflow-hidden px-4 md:px-16 py-4">
          <Bubble.List
            items={bubbleItems}
            role={roleConfig}
            style={{ height: '100%' }}
          />
        </div>

        {/* 输入区域 */}
        <div className="px-4 md:px-16 pb-6 flex-shrink-0">
          <Sender
            value={input}
            onChange={setInput}
            onSubmit={() => { const c = input; setInput(''); sendMessage(c) }}
            loading={sending}
            onCancel={() => {
              // 用户点击 stop 时清除 sending 状态和消息
              sendingRef.current = false
              setSending(false)
              setSession(prev => {
                if (!prev) return prev
                return { ...prev, messages: prev.messages.filter(m => m.id !== LOADING_MSG_ID && m.id !== STREAMING_MSG_ID) }
              })
            }}
            placeholder="描述你的需求，Shift+Enter 换行，Enter 发送..."
          />
          <p className="text-center text-xs text-slate-400 mt-2">内容由 AI 生成，请仔细甄别</p>
        </div>
      </main>
    </div>
  )
}

