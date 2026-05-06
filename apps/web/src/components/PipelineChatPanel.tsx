import { useEffect, useRef, useState } from 'react'
import { Button, Input, Space, Spin, Tag, Typography, message } from 'antd'
import { CloseOutlined, DownOutlined, LeftOutlined, RightOutlined, SendOutlined, UpOutlined } from '@ant-design/icons'
import { useRouterState } from '@tanstack/react-router'
import {
  approveCheckpoint,
  fetchCodeDiff,
  fetchSessionDetail,
  fetchPipelineAgentRuns,
  fetchPipelineCurrent,
  fetchPipelineRuns,
  rejectCheckpoint,
  sendSessionMessage,
  stageLabel,
  type AgentRun,
  type ChangeItem,
  type CodeDiffResponse,
  type PipelineRunCurrent,
  type SessionMessage,
} from '../lib/pipeline'

type ChatRole = 'user' | 'assistant'

interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  createdAt: string
}

const QUICK_ACTIONS = ['解释一下这次变更', '优化这段代码', '补充测试用例']
const DEMO_MESSAGES: ChatMessage[] = [
  {
    id: 'demo_ai_1',
    role: 'assistant',
    content: '我已完成代码生成，主要补充了错误处理和参数校验。你可以先看下面 diff，再决定是否通过。',
    createdAt: new Date().toISOString(),
  },
  {
    id: 'demo_user_1',
    role: 'user',
    content: '为什么这个函数里没有处理空指针？',
    createdAt: new Date().toISOString(),
  },
  {
    id: 'demo_ai_2',
    role: 'assistant',
    content: '你说得对，我在新版本里补了 nil 判断，并在失败时返回明确错误信息。',
    createdAt: new Date().toISOString(),
  },
]

function extractRunId(pathname: string, search: string): string {
  const match = pathname.match(/^\/approvals\/([^/?#]+)/)
  if (match?.[1]) return decodeURIComponent(match[1])
  const queryRunId = new URLSearchParams(search).get('runId')?.trim() || ''
  return queryRunId
}

function extractDiffPreview(agentRun?: AgentRun): { before: string; after: string } {
  if (!agentRun) return { before: '', after: '' }
  return {
    before: agentRun.inputJson || '',
    after: agentRun.outputJson || '',
  }
}

function mapSessionMessages(messages: SessionMessage[]): ChatMessage[] {
  return messages
    .filter(item => item.role === 'user' || item.role === 'assistant')
    .map(item => ({
      id: item.id,
      role: item.role as ChatRole,
      content: item.content,
      createdAt: item.createdAt,
    }))
}

export type PipelineChatPanelProps = {
  /** 嵌入工作台时传入当前 Run，优先于 URL */
  runId?: string
  embedded?: boolean
  onRequestClose?: () => void
  /** 审批通过/驳回后通知父级刷新（如工作台 timeline） */
  onTimelineDirty?: () => void
}

type DiffLine = {
  text: string
  kind: 'context' | 'add' | 'remove'
}

function buildUnifiedDiff(before: string, after: string): DiffLine[] {
  const a = (before || '').split('\n')
  const b = (after || '').split('\n')
  const n = a.length
  const m = b.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => Array<number>(m + 1).fill(0))

  for (let i = n - 1; i >= 0; i -= 1) {
    for (let j = m - 1; j >= 0; j -= 1) {
      if (a[i] === b[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1])
      }
    }
  }

  const lines: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      lines.push({ text: a[i], kind: 'context' })
      i += 1
      j += 1
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      lines.push({ text: a[i], kind: 'remove' })
      i += 1
    } else {
      lines.push({ text: b[j], kind: 'add' })
      j += 1
    }
  }
  while (i < n) {
    lines.push({ text: a[i], kind: 'remove' })
    i += 1
  }
  while (j < m) {
    lines.push({ text: b[j], kind: 'add' })
    j += 1
  }
  return lines
}

function renderUnifiedDiff(before: string, after: string) {
  const lines = buildUnifiedDiff(before, after)
  return (
    <div className="max-h-48 overflow-auto rounded border border-slate-200 bg-[#0b1020] p-2 font-mono text-[11px] leading-5 text-slate-100">
      {lines.length === 0 ? <div className="text-slate-400">(无)</div> : null}
      {lines.map((line, idx) => {
        const className =
          line.kind === 'add'
            ? 'bg-emerald-500/20 text-emerald-100'
            : line.kind === 'remove'
              ? 'bg-rose-500/20 text-rose-100 line-through decoration-rose-200/70'
              : ''
        return (
          <div
            key={`diff_${idx}`}
            className={`rounded px-1 ${className}`}
          >
            <span className="whitespace-pre-wrap break-all">{line.text}</span>
          </div>
        )
      })}
    </div>
  )
}

function renderGitPatchText(patch: string) {
  const raw = patch || ''
  const lines = raw.split('\n')
  return (
    <div className="max-h-40 overflow-auto rounded border border-slate-800 bg-[#0b1020] p-2 font-mono text-[11px] leading-5 text-slate-200">
      {lines.length === 0 || (lines.length === 1 && lines[0] === '') ? (
        <div className="text-slate-500">(无 diff 文本)</div>
      ) : null}
      {lines.map((line, idx) => {
        let cls = 'text-slate-300'
        if (line.startsWith('+++ ') || line.startsWith('--- ') || line.startsWith('diff --git')) cls = 'text-sky-300/90'
        else if (line.startsWith('@@')) cls = 'text-amber-200/90'
        else if (line.startsWith('+')) cls = 'bg-emerald-500/15 text-emerald-100'
        else if (line.startsWith('-')) cls = 'bg-rose-500/15 text-rose-100'
        return (
          <div key={`gp_${idx}`} className={`whitespace-pre-wrap break-all rounded px-0.5 ${cls}`}>
            {line}
          </div>
        )
      })}
    </div>
  )
}

function normalizeChangeSet(items: unknown): ChangeItem[] {
  if (!Array.isArray(items)) return []
  return items as ChangeItem[]
}

export default function PipelineChatPanel({ runId: runIdProp, embedded, onRequestClose, onTimelineDirty }: PipelineChatPanelProps = {}) {
  const location = useRouterState({ select: state => state.location })
  const [collapsed, setCollapsed] = useState(false)
  const [resolvingData, setResolvingData] = useState(false)
  const [current, setCurrent] = useState<PipelineRunCurrent | null>(null)
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([])
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [deciding, setDeciding] = useState<'approve' | 'reject' | null>(null)
  const [diffExpanded, setDiffExpanded] = useState(true)
  const [codeDiff, setCodeDiff] = useState<CodeDiffResponse | null>(null)
  const [codeDiffLoading, setCodeDiffLoading] = useState(false)
  const messagesRef = useRef<HTMLDivElement | null>(null)

  const runId = (runIdProp && runIdProp.trim()) || extractRunId(location.pathname, location.searchStr)
  const isDemoMode = new URLSearchParams(location.searchStr).get('chatDemo') === '1'
  const sessionId = current?.run?.sourceSessionId || ''
  const checkpointId = current?.checkpoint?.id || ''
  const currentStage = current?.stage?.stageKey || current?.run?.currentStageKey
  const latestAgentRun = agentRuns.length > 0 ? agentRuns[agentRuns.length - 1] : undefined
  const canDecide = Boolean(checkpointId && current?.run?.status === 'waiting_approval' && !deciding)
  const canSend = Boolean(sessionId && input.trim() && !sending)
  const diff = extractDiffPreview(latestAgentRun)
  const changeSet = normalizeChangeSet(codeDiff?.changeSet)
  const hasApiDiff = changeSet.some(item => (item.proposedDiff || '').trim().length > 0) || Boolean((codeDiff?.summary || '').trim())
  const shouldShowDecision = Boolean(canDecide && latestAgentRun)
  const lastAssistantMessageId = [...chatMessages].reverse().find(item => item.role === 'assistant')?.id

  const reloadRunContext = async (targetRunId: string) => {
    setCodeDiffLoading(true)
    try {
      const [currentData, agentRunData, diffResult] = await Promise.all([
        fetchPipelineCurrent(targetRunId),
        fetchPipelineAgentRuns(targetRunId),
        fetchCodeDiff(targetRunId).catch(() => null),
      ])
      setCurrent(currentData)
      setAgentRuns(agentRunData)
      setCodeDiff(diffResult)
      if (currentData.run.sourceSessionId) {
        const sessionDetail = await fetchSessionDetail(currentData.run.sourceSessionId)
        setChatMessages(mapSessionMessages(sessionDetail.messages))
      } else {
        setChatMessages([])
      }
    } finally {
      setCodeDiffLoading(false)
    }
  }

  useEffect(() => {
    messagesRef.current?.scrollTo({ top: messagesRef.current.scrollHeight, behavior: 'smooth' })
  }, [chatMessages, agentRuns])

  useEffect(() => {
    if (isDemoMode) {
      setCodeDiff(null)
      setCodeDiffLoading(false)
      setCurrent({
        run: {
          id: 'run_demo',
          templateId: 'tpl_default',
          title: '演示：优化审批聊天面板',
          requirementText: '在审批页加入 Cursor 风格聊天面板',
          sourceSessionId: 'session_demo',
          targetRepo: 'self',
          targetBranch: 'main',
          workBranch: 'devflow/demo',
          status: 'waiting_approval',
          currentStageKey: 'code_generation',
          createdBy: 'demo',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        stage: {
          id: 'stage_demo',
          pipelineRunId: 'run_demo',
          stageKey: 'code_generation',
          stageType: 'codegen',
          status: 'waiting_approval',
          attempt: 1,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        checkpoint: {
          id: 'checkpoint_demo',
          pipelineRunId: 'run_demo',
          stageRunId: 'stage_demo',
          checkpointType: 'code_review',
          status: 'pending',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        nextAction: 'approve_checkpoint',
      })
      setAgentRuns([{
        id: 'agent_demo_1',
        pipelineRunId: 'run_demo',
        stageRunId: 'stage_demo',
        agentKey: 'code_generation',
        provider: 'demo',
        model: 'mock',
        inputJson: 'func UpdateUser(user *User) error {\n  return repo.Save(user)\n}',
        outputJson: 'func UpdateUser(user *User) error {\n  if user == nil {\n    return errors.New("user is nil")\n  }\n  if err := repo.Save(user); err != nil {\n    return fmt.Errorf("save user failed: %w", err)\n  }\n  return nil\n}',
        tokenUsageJson: '{"prompt":128,"completion":196}',
        latencyMs: 420,
        status: 'succeeded',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      }])
      setChatMessages(DEMO_MESSAGES)
      setResolvingData(false)
      return
    }
    let cancelled = false
    const resolveRunContext = async () => {
      setResolvingData(true)
      try {
        let targetRunId = runId
        if (!targetRunId) {
          const runs = await fetchPipelineRuns()
          targetRunId = runs.find(item => item.status === 'waiting_approval')?.id || ''
        }
        if (!targetRunId) {
          if (!cancelled) {
            setCurrent(null)
            setAgentRuns([])
            setCodeDiff(null)
          }
          return
        }
        await reloadRunContext(targetRunId)
        if (cancelled) return
      } catch (err) {
        if (!cancelled) {
          message.error(err instanceof Error ? err.message : '加载聊天面板上下文失败')
        }
      } finally {
        if (!cancelled) setResolvingData(false)
      }
    }
    void resolveRunContext()
    return () => {
      cancelled = true
    }
  }, [isDemoMode, runId])

  const handleQuickAction = (text: string) => {
    setInput(text)
  }

  const handleSend = async () => {
    if (isDemoMode) {
      if (!input.trim()) return
      const now = new Date().toISOString()
      const payload = input.trim()
      setChatMessages(prev => [
        ...prev,
        { id: `demo_user_${Date.now()}`, role: 'user', content: payload, createdAt: now },
      ])
      setInput('')
      setSending(true)
      window.setTimeout(() => {
        setChatMessages(prev => [
          ...prev,
          {
            id: `demo_ai_${Date.now()}`,
            role: 'assistant',
            content: `收到修改意见：${payload}\n我已更新建议并生成新的 diff 版本，请继续查看后决定。`,
            createdAt: new Date().toISOString(),
          },
        ])
        setSending(false)
      }, 500)
      return
    }
    if (!sessionId || !input.trim() || sending) return
    const userMessage: ChatMessage = {
      id: `local_user_${Date.now()}`,
      role: 'user',
      content: input.trim(),
      createdAt: new Date().toISOString(),
    }
    const payload = input.trim()
    setInput('')
    setSending(true)
    setChatMessages(prev => [...prev, userMessage])
    try {
      const session = await sendSessionMessage(sessionId, payload)
      setChatMessages(mapSessionMessages(session.messages))
      if (current?.run?.id) {
        await reloadRunContext(current.run.id)
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '发送消息失败')
    } finally {
      setSending(false)
    }
  }

  const handleApprove = async () => {
    if (isDemoMode) {
      message.success('演示模式：已 Resolve（未调用后端）')
      setCollapsed(true)
      return
    }
    if (!checkpointId || deciding) return
    setDeciding('approve')
    try {
      await approveCheckpoint(checkpointId, '通过右侧聊天面板审批通过')
      message.success('已通过审批，流水线继续执行')
      if (current?.run?.id) {
        await reloadRunContext(current.run.id)
      }
      onTimelineDirty?.()
      setCollapsed(true)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '审批失败')
    } finally {
      setDeciding(null)
    }
  }

  const handleReject = async () => {
    if (isDemoMode) {
      message.success('演示模式：已 Reject（未调用后端）')
      setCollapsed(true)
      return
    }
    if (!checkpointId || deciding) return
    setDeciding('reject')
    try {
      await rejectCheckpoint(checkpointId, '')
      message.success('已驳回审批，流水线将回退重做')
      if (current?.run?.id) {
        await reloadRunContext(current.run.id)
      }
      onTimelineDirty?.()
      setCollapsed(true)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '驳回失败')
    } finally {
      setDeciding(null)
    }
  }

  const renderCodeDiffBody = () => {
    if (codeDiffLoading) {
      return (
        <div className="flex items-center gap-2 text-slate-500">
          <Spin size="small" />
          同步流水线 diff...
        </div>
      )
    }
    if (hasApiDiff) {
      return (
        <div className="space-y-2">
          {codeDiff?.summary?.trim() ? (
            <pre className="max-h-28 overflow-auto whitespace-pre-wrap rounded border border-slate-200 bg-white p-2 text-[11px] leading-relaxed text-slate-600">
              {codeDiff.summary}
            </pre>
          ) : null}
          {changeSet.length > 0
            ? changeSet.map((item, i) => (
                <div key={`${item.filePath || 'f'}_${i}`} className="space-y-1">
                  <div className="text-[11px] font-medium text-slate-600">
                    {item.filePath || `变更 ${i + 1}`}
                    {item.changeType ? <span className="ml-2 font-normal text-slate-400">[{item.changeType}]</span> : null}
                  </div>
                  {renderGitPatchText(item.proposedDiff || '')}
                </div>
              ))
            : null}
          {codeDiff?.updatedAt ? (
            <div className="text-[10px] text-slate-400">更新于 {new Date(codeDiff.updatedAt).toLocaleString('zh-CN')}</div>
          ) : null}
        </div>
      )
    }
    return renderUnifiedDiff(diff.before || '', diff.after || '')
  }

  return (
    <div className={`fixed right-0 top-0 z-40 h-screen border-l border-slate-200 bg-white shadow-xl transition-all duration-200 ${collapsed ? 'w-12' : 'w-[420px]'}`}>
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b border-slate-100 px-3 py-3">
          {!collapsed ? <Typography.Text strong>代码审批</Typography.Text> : null}
          <div className="flex shrink-0 items-center gap-0">
            {embedded && onRequestClose ? (
              <Button
                type="text"
                icon={<CloseOutlined />}
                onClick={() => onRequestClose()}
                aria-label="关闭面板"
              />
            ) : null}
            <Button
              type="text"
              icon={collapsed ? <LeftOutlined /> : <RightOutlined />}
              onClick={() => setCollapsed(prev => !prev)}
              aria-label={collapsed ? '展开聊天面板' : '折叠聊天面板'}
            />
          </div>
        </div>

        {collapsed ? null : (
          <div className="flex min-h-0 flex-1 flex-col p-3">
            <div className="mb-2 flex items-center justify-between gap-2">
              <Typography.Text type="secondary" className="text-xs">阶段：{stageLabel(currentStage)}</Typography.Text>
              <Tag color={current?.run?.status === 'waiting_approval' ? 'warning' : 'default'}>
                {current?.run?.status || '未运行'}
              </Tag>
            </div>

            <div ref={messagesRef} className="min-h-0 flex-1 space-y-3 overflow-y-auto rounded border border-slate-100 bg-slate-50 p-3">
              {resolvingData ? (
                <div className="flex items-center gap-2 text-slate-500"><Spin size="small" /> 加载上下文...</div>
              ) : null}
              {!resolvingData && agentRuns.length === 0 && hasApiDiff ? (
                <div className="flex justify-start">
                  <div className="max-w-[92%] rounded-2xl rounded-bl-md bg-white px-3 py-2 text-xs shadow-sm">
                    <div className="mb-1 font-medium text-slate-600">代码变更（流水线）</div>
                    <div className="space-y-2 rounded-lg border border-slate-200 bg-slate-50 p-2">
                      <button
                        type="button"
                        className="flex w-full items-center justify-between text-left text-[11px] font-medium text-slate-600"
                        onClick={() => setDiffExpanded(prev => !prev)}
                      >
                        <span>代码 diff · GET /code-diff</span>
                        {diffExpanded ? <UpOutlined /> : <DownOutlined />}
                      </button>
                      {diffExpanded ? <div className="space-y-2">{renderCodeDiffBody()}</div> : null}
                    </div>
                  </div>
                </div>
              ) : null}
              {agentRuns.map(run => (
                <div key={run.id} className="flex justify-start">
                  <div className="max-w-[92%] rounded-2xl rounded-bl-md bg-white px-3 py-2 text-xs shadow-sm">
                    <div className="mb-1 font-medium text-slate-600">AI · {run.agentKey}</div>
                    <pre className="max-h-40 overflow-auto whitespace-pre-wrap">{run.outputJson || '(无输出)'}</pre>
                    {latestAgentRun?.id === run.id ? (
                      <div className="mt-3 space-y-2 rounded-lg border border-slate-200 bg-slate-50 p-2">
                        <button
                          type="button"
                          className="flex w-full items-center justify-between text-left text-[11px] font-medium text-slate-600"
                          onClick={() => setDiffExpanded(prev => !prev)}
                        >
                          <span>{hasApiDiff ? '代码 diff（流水线 /code-diff）' : '代码 diff（Agent 输入/输出对比）'}</span>
                          {diffExpanded ? <UpOutlined /> : <DownOutlined />}
                        </button>
                        {diffExpanded ? (
                          <div className="space-y-2">
                            {renderCodeDiffBody()}
                          </div>
                        ) : null}
                      </div>
                    ) : null}
                    {shouldShowDecision && latestAgentRun?.id === run.id ? (
                      <div className="mt-2 flex gap-2">
                        <Button type="primary" size="small" disabled={!canDecide} loading={deciding === 'approve'} onClick={() => void handleApprove()}>
                          Resolve
                        </Button>
                        <Button danger size="small" disabled={!canDecide} loading={deciding === 'reject'} onClick={() => void handleReject()}>
                          Reject
                        </Button>
                      </div>
                    ) : null}
                  </div>
                </div>
              ))}
              {chatMessages.map(msg => (
                <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  <div className={`max-w-[92%] ${msg.role === 'user' ? 'items-end' : 'items-start'} flex flex-col`}>
                    <div className={`w-full rounded-2xl px-3 py-2 text-xs shadow-sm ${msg.role === 'user' ? 'rounded-br-md bg-blue-500 text-white' : 'rounded-bl-md bg-white text-slate-800'}`}>
                      <div className={`mb-1 font-medium ${msg.role === 'user' ? 'text-blue-100' : 'text-slate-600'}`}>{msg.role === 'user' ? '你' : 'AI'}</div>
                      <div className="whitespace-pre-wrap">{msg.content}</div>
                    </div>
                    {msg.role === 'assistant' && msg.id === lastAssistantMessageId && canDecide ? (
                      <div className="mt-2 flex gap-2">
                        <Button type="primary" size="small" disabled={!canDecide} loading={deciding === 'approve'} onClick={() => void handleApprove()}>
                          Resolve
                        </Button>
                        <Button danger size="small" disabled={!canDecide} loading={deciding === 'reject'} onClick={() => void handleReject()}>
                          Reject
                        </Button>
                      </div>
                    ) : null}
                  </div>
                </div>
              ))}
              {!resolvingData && agentRuns.length === 0 && chatMessages.length === 0 && !hasApiDiff ? (
                <div className="text-xs text-slate-400">暂无可展示消息，进入审批节点后会显示 Agent 记录。</div>
              ) : null}
            </div>

            <Space wrap className="mt-3">
              {QUICK_ACTIONS.map(item => (
                <Button key={item} size="small" onClick={() => handleQuickAction(item)}>
                  {item}
                </Button>
              ))}
            </Space>

            <div className="mt-3 flex gap-2">
              <Input.TextArea
                rows={2}
                value={input}
                disabled={!sessionId || sending}
                onChange={(event) => setInput(event.target.value)}
                placeholder={sessionId ? '输入你的问题或修改要求...' : '当前 run 未绑定 session，无法发消息'}
              />
              <Button type="primary" icon={<SendOutlined />} disabled={!canSend} loading={sending} onClick={() => void handleSend()} />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
