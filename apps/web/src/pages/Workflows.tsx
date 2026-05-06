import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useRouterState, useSearch } from '@tanstack/react-router'
import { Alert, Button, Card, Checkbox, Empty, Form, Input, Modal, Progress, Select, Skeleton, Space, Tag, Tooltip, message, Drawer, Spin } from 'antd'
import { PlusOutlined, PauseCircleOutlined, PlayCircleOutlined, ReloadOutlined, StopOutlined, SyncOutlined, FileTextOutlined, CodeOutlined } from '@ant-design/icons'
import Sidebar from '../components/Sidebar'
import PipelineChatPanel from '../components/PipelineChatPanel'
import {
  type AgentRun,
  type Artifact,
  type PipelineRun,
  type PipelineRunTimeline,
  fetchPipelineRuns,
  fetchPipelineTimeline,
  formatDateTime,
  formatDuration,
  isCodeApprovalStage,
  isLiveRun,
  latestArtifact,
  nextActionLabel,
  pausePipelineRun,
  resumePipelineRun,
  runStatusMeta,
  stageLabel,
  stageStatusMeta,
  startPipelineRun,
  terminatePipelineRun,
} from '../lib/pipeline'

const sidebarWidth = 80

/** 地址栏与 Router 解析有时不同步，两处任一命中 ?chatDemo=1 即视为 Mock */
function useWorkflowChatDemoFlag(): boolean {
  return useRouterState({
    select: state => {
      if (typeof window !== 'undefined') {
        if (new URLSearchParams(window.location.search).get('chatDemo') === '1') return true
      }
      const s = state.location.search as Record<string, unknown> | undefined
      const v = s?.chatDemo
      return v === '1' || v === 1 || String(v ?? '') === '1'
    },
  })
}

export default function Workflows() {
  const navigate = useNavigate()
  const { runId: runIdFromSearch } = useSearch({ from: '/workflows' })
  const workflowChatDemo = useWorkflowChatDemoFlag()

  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [selectedRunId, setSelectedRunId] = useState('')
  const [timeline, setTimeline] = useState<PipelineRunTimeline | null>(null)
  const [loadingRuns, setLoadingRuns] = useState(true)
  const [loadingTimeline, setLoadingTimeline] = useState(false)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [createModalVisible, setCreateModalVisible] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [githubRepos, setGithubRepos] = useState<{full_name: string; html_url: string; private: boolean}[]>([])
  const [loadingRepos, setLoadingRepos] = useState(false)
  const [githubBranches, setGithubBranches] = useState<string[]>([])
  const [loadingBranches, setLoadingBranches] = useState(false)
  const [showCreateRepoModal, setShowCreateRepoModal] = useState(false)
  const [createRepoLoading, setCreateRepoLoading] = useState(false)
  // 飞书文档选择相关状态
  const [docDrawerVisible, setDocDrawerVisible] = useState(false)
  const [wikiSpaces, setWikiSpaces] = useState<any[]>([])
  const [loadingSpaces, setLoadingSpaces] = useState(false)
  const [currentFolder, setCurrentFolder] = useState('')
  const [folderHistory, setFolderHistory] = useState<{token: string; name: string}[]>([])
  const [selectedDocUrls, setSelectedDocUrls] = useState<string[]>([])
  const [form] = Form.useForm<{
    title: string
    requirementText: string
    targetRepo: string
    targetBranch: string
  }>()
  const [approvalChatOpen, setApprovalChatOpen] = useState(false)

  const selectedRun = useMemo(
    () => runs.find(run => run.id === selectedRunId) ?? timeline?.run,
    [runs, selectedRunId, timeline],
  )
  const currentArtifact = latestArtifact(timeline ?? undefined)
  const completionPercent = timeline?.summary.totalStages
    ? Math.round((timeline.summary.completedStages / timeline.summary.totalStages) * 100)
    : 0
  const recentAgentRuns = useMemo(
    () => [...(timeline?.agentRuns ?? [])].slice(-4).reverse(),
    [timeline?.agentRuns],
  )

  const loadRuns = async () => {
    setLoadingRuns(true)
    setError('')
    try {
      const items = await fetchPipelineRuns()
      setRuns(items)
<<<<<<< HEAD
      const queryRunId = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('runId')?.trim() : ''
      const firstId = queryRunId && items.some(item => item.id === queryRunId) ? queryRunId : items[0]?.id
=======
      const fromRouter = runIdFromSearch?.trim() || ''
      const fromWindow =
        typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('runId')?.trim() || '' : ''
      const prefer = fromRouter || fromWindow
      if (prefer && prefer !== 'undefined' && prefer !== 'null') {
        setSelectedRunId(prefer)
        return
      }
      const firstId = items[0]?.id
>>>>>>> afa286698b5abaf48b72d3c492bc7b0ab40399ab
      if (firstId && firstId !== 'undefined') {
        setSelectedRunId(firstId)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载流水线失败')
      setRuns([])
    } finally {
      setLoadingRuns(false)
    }
  }

  const loadTimeline = async (runId: string) => {
    // 防御性检查：确保 runId 是有效的非空字符串
    const safeId = String(runId || '').trim()
    if (!safeId || safeId === 'undefined' || safeId === 'null') {
      console.warn('[loadTimeline] Invalid runId, skipping:', runId)
      setTimeline(null)
      return
    }
    setLoadingTimeline(true)
    setError('')
    try {
      setTimeline(await fetchPipelineTimeline(safeId))
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载工作台失败')
      setTimeline(null)
    } finally {
      setLoadingTimeline(false)
    }
  }

  useEffect(() => {
    void loadRuns()
  }, [])

  useEffect(() => {
    const id = runIdFromSearch?.trim()
    if (id && id !== 'undefined' && id !== 'null') {
      setSelectedRunId(id)
    }
  }, [runIdFromSearch])

  useEffect(() => {
    void loadTimeline(selectedRunId)
  }, [selectedRunId])

  // 防止 selectedRunId 为 undefined 或空字符串时发送请求
  useEffect(() => {
    if (!selectedRunId || selectedRunId === 'undefined' || selectedRunId === '') {
      return
    }
    void loadTimeline(selectedRunId)
  }, [])

  useEffect(() => {
    if (!timeline || !isLiveRun(timeline.run.status)) return
    const timer = window.setInterval(() => {
      void loadTimeline(timeline.run.id)
    }, 8000)
    return () => window.clearInterval(timer)
  }, [timeline?.run.id, timeline?.run.status])

  const showCodeApprovalFab = isCodeApprovalStage(timeline) || workflowChatDemo

  useEffect(() => {
    if (!showCodeApprovalFab) setApprovalChatOpen(false)
  }, [showCodeApprovalFab])

  const refreshAll = async () => {
    await loadRuns()
    if (selectedRunId) await loadTimeline(selectedRunId)
  }

  const openMockApprovalPreview = () => {
    navigate({ to: '/workflows', search: { chatDemo: '1' } })
  }

  const exitMockApprovalPreview = () => {
    navigate({ to: '/workflows', search: {} })
  }

  const handleCreate = async (values: CreateFormValues) => {
    setCreateLoading(true)
    try {
      const res = await fetch('/api/pipeline-runs', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: values.title,
          requirementText: values.requirementText,
          targetRepo: values.targetRepo || 'self',
          targetBranch: values.targetBranch || 'main',
          selectedDocUrls: selectedDocUrls, // 飞书文档 URL
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || '创建失败')
      }
      message.success('Pipeline 创建成功')
      setCreateModalVisible(false)
      setSelectedDocUrls([])
      form.resetFields()
      await loadRuns()
      if (data.data?.id) {
        setSelectedRunId(data.data.id)
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '创建失败')
    } finally {
      setCreateLoading(false)
    }
  }

  const loadGithubRepos = async () => {
    setLoadingRepos(true)
    try {
      const res = await fetch('/api/github/repos', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        setGithubRepos(data.data || [])
      }
    } catch {
      // 忽略错误，用户可以手动输入
    } finally {
      setLoadingRepos(false)
    }
  }

  // 加载仓库的分支列表
  const loadGithubBranches = async (repoFullName: string) => {
    if (repoFullName === 'self') {
      setGithubBranches([])
      return
    }
    const [owner, repo] = repoFullName.split('/')
    if (!owner || !repo) return

    setLoadingBranches(true)
    try {
      const res = await fetch(`/api/github/repos/${owner}/${repo}/branches`, { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        const branches = (data.data || []).map((b: { name: string }) => b.name)
        setGithubBranches(branches)
        // 如果有 main 分支，自动选中
        if (branches.includes('main')) {
          form.setFieldValue('targetBranch', 'main')
        }
      }
    } catch {
      // 忽略错误
    } finally {
      setLoadingBranches(false)
    }
  }

  // 打开创建弹窗时加载 GitHub 仓库列表
  const handleOpenCreateModal = () => {
    setCreateModalVisible(true)
    setGithubBranches([])
    void loadGithubRepos()
  }

  // 打开文档选择抽屉
  const openDocPicker = async () => {
    setDocDrawerVisible(true)
    setLoadingSpaces(true)
    setWikiSpaces([])
    setCurrentFolder('')
    setFolderHistory([])
    try {
      const res = await fetch('/api/feishu/documents')
      if (res.ok) {
        const data = await res.json()
        setWikiSpaces(data.data || [])
      } else {
        message.error('获取文档列表失败，请先绑定飞书')
      }
    } catch {
      message.error('获取文档列表失败')
    } finally {
      setLoadingSpaces(false)
    }
  }

  // 进入文件夹
  const enterFolder = async (folderToken: string) => {
    setLoadingSpaces(true)
    try {
      const res = await fetch(`/api/feishu/documents?folder_token=${folderToken}`)
      if (res.ok) {
        const data = await res.json()
        // 保存当前文件夹到历史
        const currentFiles = wikiSpaces.find(f => f.type === 'folder')
        if (currentFiles) {
          setFolderHistory(prev => [...prev, { token: currentFolder, name: '当前文件夹' }])
        }
        setCurrentFolder(folderToken)
        setWikiSpaces(data.data || [])
      }
    } catch {
      message.error('加载文件夹失败')
    } finally {
      setLoadingSpaces(false)
    }
  }

  // 返回上级文件夹
  const goBack = async () => {
    const history = [...folderHistory]
    const prev = history.pop()
    if (!prev) return
    setFolderHistory(history)
    setCurrentFolder(prev.token)
    setLoadingSpaces(true)
    try {
      const url = prev.token ? `/api/feishu/documents?folder_token=${prev.token}` : '/api/feishu/documents'
      const res = await fetch(url)
      if (res.ok) {
        const data = await res.json()
        setWikiSpaces(data.data || [])
      }
    } catch {
      message.error('加载文件夹失败')
    } finally {
      setLoadingSpaces(false)
    }
  }

  // 选择/取消选择文档
  const toggleSelectDocument = (doc: any) => {
    if (doc.type === 'folder') {
      void enterFolder(doc.token)
      return
    }
    // 只处理文档类型，跳过 bitable 等
    if (doc.type !== 'docx' && doc.type !== 'sheet' && doc.type !== 'mindnote' && doc.type !== 'slides') {
      return
    }
    const url = doc.url?.replace('lanshanteam.feishu.cn', 'feishu.cn')
    setSelectedDocUrls(prev => {
      if (prev.includes(url)) {
        return prev.filter(u => u !== url)
      }
      return [...prev, url]
    })
  }

  // 仓库选择变化时加载分支
  const handleRepoChange = (value: string) => {
    form.setFieldValue('targetBranch', value === 'self' ? 'main' : '')
    void loadGithubBranches(value)
  }

  const runAction = async (action: string, fn: () => Promise<void>) => {
    setActionLoading(action)
    try {
      await fn()
      message.success('操作已提交')
      await refreshAll()
    } catch (err) {
      message.error(err instanceof Error ? err.message : '操作失败')
    } finally {
      setActionLoading(null)
    }
  }

  const renderRunActions = () => {
    if (!selectedRun) return null
    const status = selectedRun.status
    return (
      <Space>
        {status === 'draft' || status === 'failed' ? (
          <Tooltip title="启动">
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={actionLoading === 'start'}
              onClick={() => runAction('start', () => startPipelineRun(selectedRun.id))}
            />
          </Tooltip>
        ) : null}
        {status === 'queued' || status === 'running' ? (
          <Tooltip title="暂停">
            <Button
              icon={<PauseCircleOutlined />}
              loading={actionLoading === 'pause'}
              onClick={() => runAction('pause', () => pausePipelineRun(selectedRun.id))}
            />
          </Tooltip>
        ) : null}
        {status === 'paused' || status === 'failed' ? (
          <Tooltip title="恢复">
            <Button
              icon={<PlayCircleOutlined />}
              loading={actionLoading === 'resume'}
              onClick={() => runAction('resume', () => resumePipelineRun(selectedRun.id))}
            />
          </Tooltip>
        ) : null}
        {['draft', 'queued', 'running', 'waiting_approval', 'paused', 'failed'].includes(status) ? (
          <Tooltip title="终止">
            <Button
              danger
              icon={<StopOutlined />}
              loading={actionLoading === 'terminate'}
              onClick={() => runAction('terminate', () => terminatePipelineRun(selectedRun.id))}
            />
          </Tooltip>
        ) : null}
        <Tooltip title="刷新">
          <Button icon={<ReloadOutlined />} onClick={refreshAll} loading={loadingTimeline} />
        </Tooltip>
      </Space>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Sidebar />
      <main className="h-screen overflow-hidden p-5 transition-all duration-300" style={{ marginLeft: `${sidebarWidth}px` }}>
        <div className="flex h-full gap-4">
          <aside className="w-80 shrink-0 overflow-y-auto rounded-lg border border-outline-variant bg-surface-container-lowest">
            <div className="sticky top-0 z-10 flex items-center justify-between border-b border-outline-variant bg-surface-container-lowest px-4 py-3">
              <div>
                <h1 className="m-0 text-lg font-bold text-on-surface">Pipeline 工作台</h1>
                <div className="text-xs text-on-surface-variant">{runs.length} 个运行记录</div>
              </div>
              <Space wrap size="small">
                <Tooltip title="无需后端流水线：展示右侧圆形代码按钮与审批面板演示数据">
                  <Button
                    type={workflowChatDemo ? 'default' : 'dashed'}
                    icon={<CodeOutlined />}
                    onClick={() => (workflowChatDemo ? exitMockApprovalPreview() : openMockApprovalPreview())}
                  >
                    {workflowChatDemo ? '退出预览' : '审批预览'}
                  </Button>
                </Tooltip>
                <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreateModal}>新建</Button>
                <Button type="text" icon={<ReloadOutlined />} onClick={refreshAll} loading={loadingRuns} />
              </Space>
            </div>
            <div className="p-3">
              {loadingRuns ? <Skeleton active paragraph={{ rows: 8 }} /> : null}
              {!loadingRuns && runs.length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <span className="text-on-surface-variant">
                      暂无运行记录。可点上方「审批预览」加载 Mock，或新建 Pipeline；真实环境下仅在流水线进入「评审确认」待审批时会出现右侧图标。
                    </span>
                  }
                />
              ) : null}
              <div className="space-y-2">
                {runs.map(run => {
                  const meta = runStatusMeta(run.status)
                  const active = run.id === selectedRunId
                  return (
                    <button
                      key={run.id}
                      type="button"
                      onClick={() => setSelectedRunId(run.id)}
                      className={`w-full rounded-lg border p-3 text-left transition ${
                        active ? 'border-primary bg-primary/5' : 'border-outline-variant bg-white hover:border-primary/50'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="truncate text-sm font-semibold text-on-surface">{run.title}</div>
                          <div className="mt-1 truncate text-xs text-on-surface-variant">{run.targetRepo} · {run.targetBranch}</div>
                        </div>
                        <Tag color={meta.color}>{meta.label}</Tag>
                      </div>
                      <div className="mt-3 flex items-center justify-between text-xs text-on-surface-variant">
                        <span>{stageLabel(run.currentStageKey)}</span>
                        <span>{formatDateTime(run.updatedAt)}</span>
                      </div>
                    </button>
                  )
                })}
              </div>
            </div>
          </aside>

          <section className="flex min-w-0 flex-1 flex-col overflow-hidden">
            <div className="mb-4 flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="mb-1 flex items-center gap-2">
                  {selectedRun ? <Tag color={runStatusMeta(selectedRun.status).color}>{runStatusMeta(selectedRun.status).label}</Tag> : null}
                  {timeline?.current?.nextAction ? <Tag color="blue">{nextActionLabel(timeline.current.nextAction)}</Tag> : null}
                </div>
                <h2 className="m-0 truncate text-2xl font-bold text-on-surface">{selectedRun?.title || '选择 PipelineRun'}</h2>
                <p className="m-0 mt-1 truncate text-sm text-on-surface-variant">{selectedRun?.requirementText || '暂无运行记录'}</p>
              </div>
              {renderRunActions()}
            </div>

            {error ? <Alert className="mb-4" type="error" showIcon message={error} /> : null}

            {workflowChatDemo ? (
              <Alert
                className="mb-4"
                type="info"
                showIcon
                message="Mock 预览模式（chatDemo=1）"
                description="左侧为真实工作台布局；右侧圆形按钮打开的面板为演示数据，便于联调 UI。"
              />
            ) : null}

            <div className="grid grid-cols-4 gap-3">
              <Metric title="完成阶段" value={`${timeline?.summary.completedStages ?? 0}/${timeline?.summary.totalStages ?? 0}`} />
              <Metric title="当前阶段" value={stageLabel(timeline?.summary.currentStageKey)} />
              <Metric title="运行耗时" value={formatDuration(timeline?.summary.durationMs)} />
              <Metric title="AgentRun" value={`${timeline?.agentRuns.length ?? 0}`} />
            </div>

            <div className="mt-4 grid min-h-0 flex-1 grid-cols-[1fr_360px] gap-4 overflow-hidden">
              <div className="min-w-0 overflow-y-auto">
                <Card className="!rounded-lg">
                  <div className="mb-3 flex items-center justify-between">
                    <span className="font-semibold text-on-surface">阶段进度</span>
                    <span className="text-sm text-on-surface-variant">{completionPercent}%</span>
                  </div>
                  <Progress percent={completionPercent} showInfo={false} />
                  <div className="mt-4 grid grid-cols-1 gap-2">
                    {(timeline?.stages ?? []).map(stage => (
                      <StageRow key={stage.id} stage={stage} active={stage.stageKey === timeline?.run.currentStageKey} />
                    ))}
                    {!timeline && !loadingTimeline ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : null}
                    {loadingTimeline ? <Skeleton active paragraph={{ rows: 6 }} /> : null}
                  </div>
                </Card>

                <Card className="!mt-4 !rounded-lg" title="阶段产物">
                  <div className="space-y-2">
                    {(timeline?.artifacts ?? []).slice(-5).reverse().map(artifact => (
                      <ArtifactItem key={artifact.id} artifact={artifact} />
                    ))}
                    {timeline?.artifacts.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : null}
                  </div>
                </Card>
              </div>

              <aside className="min-w-0 overflow-y-auto">
                <Card className="!rounded-lg" title="当前上下文">
                  <div className="space-y-4">
                    <Field label="当前阶段" value={stageLabel(timeline?.current?.stage?.stageKey)} />
                    <Field label="下一动作" value={nextActionLabel(timeline?.current?.nextAction)} />
                    <Field label="最新产物" value={currentArtifact?.title || '-'} />
                    <Field label="交付草稿" value={timeline?.current?.delivery?.prmrTitle || '-'} />
                  </div>
                </Card>

                <Card className="!mt-4 !rounded-lg" title="Agent 观测">
                  <div className="space-y-3">
                    {recentAgentRuns.map(agentRun => <AgentRunItem key={agentRun.id} agentRun={agentRun} />)}
                    {recentAgentRuns.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : null}
                  </div>
                </Card>
              </aside>
            </div>
          </section>
        </div>
      </main>

      {showCodeApprovalFab && !approvalChatOpen ? (
        <Tooltip title="代码审批：查看 Diff 与对话" placement="left">
          <Button
            type="primary"
            shape="circle"
            size="large"
            icon={<CodeOutlined />}
            className="fixed right-5 top-1/2 z-[100] !flex h-12 w-12 !items-center !justify-center -translate-y-1/2 shadow-lg"
            onClick={() => setApprovalChatOpen(true)}
            aria-label="打开代码审批面板"
          />
        </Tooltip>
      ) : null}
      {showCodeApprovalFab && approvalChatOpen ? (
        <PipelineChatPanel
          runId={timeline?.run?.id ?? selectedRunId}
          embedded
          onRequestClose={() => setApprovalChatOpen(false)}
          onTimelineDirty={() => void refreshAll()}
        />
      ) : null}

      <CreatePipelineModal
        open={createModalVisible}
        onClose={() => setCreateModalVisible(false)}
        onSubmit={handleCreate}
        loading={createLoading}
        form={form}
        githubRepos={githubRepos.map(r => ({ fullName: r.full_name, htmlUrl: r.html_url, isPrivate: r.private }))}
        loadingRepos={loadingRepos}
        githubBranches={githubBranches}
        loadingBranches={loadingBranches}
        onRepoChange={handleRepoChange}
        onOpenCreateRepo={() => setShowCreateRepoModal(true)}
        selectedDocUrls={selectedDocUrls}
        onDocPicker={openDocPicker}
        onRemoveDoc={(i) => setSelectedDocUrls(prev => prev.filter((_, idx) => idx !== i))}
      />

      {/* 飞书文档选择抽屉 */}
      <FeishuDocDrawer
        open={docDrawerVisible}
        onClose={() => setDocDrawerVisible(false)}
        wikiSpaces={wikiSpaces}
        loadingSpaces={loadingSpaces}
        currentFolder={currentFolder}
        onBack={goBack}
        onToggleSelect={toggleSelectDocument}
        selectedDocUrls={selectedDocUrls}
      />

      {/* 新建仓库弹窗 */}
      <Modal
        title="新建 GitHub 仓库"
        open={showCreateRepoModal}
        onCancel={() => setShowCreateRepoModal(false)}
        footer={null}
        width={400}
      >
        <Form
          layout="vertical"
          onFinish={async (values) => {
            setCreateRepoLoading(true)
            try {
              const res = await fetch('/api/github/repos', {
                method: 'POST',
                credentials: 'include',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  name: values.name,
                  description: values.description || '',
                  private: values.private || false,
                }),
              })
              const data = await res.json()
              if (!res.ok) throw new Error(data.error || '创建失败')
              message.success('仓库创建成功')
              setShowCreateRepoModal(false)
              // 刷新仓库列表并选中新建的仓库
              await loadGithubRepos()
              form.setFieldValue('targetRepo', data.data.full_name)
              void loadGithubBranches(data.data.full_name)
            } catch (err) {
              message.error(err instanceof Error ? err.message : '创建失败')
            } finally {
              setCreateRepoLoading(false)
            }
          }}
        >
          <Form.Item name="name" label="仓库名称" rules={[{ required: true, message: '请输入仓库名称' }]}>
            <Input placeholder="my-awesome-repo" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="可选，仓库描述..." rows={2} />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>设为私有仓库</Checkbox>
          </Form.Item>
          <div className="flex justify-end gap-3 mt-4">
            <Button onClick={() => setShowCreateRepoModal(false)}>取消</Button>
            <Button type="primary" htmlType="submit" loading={createRepoLoading}>
              创建
            </Button>
          </div>
        </Form>
      </Modal>
    </div>
  )
}

function Metric({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-lg border border-outline-variant bg-white px-4 py-3">
      <div className="text-xs font-semibold text-on-surface-variant">{title}</div>
      <div className="mt-1 truncate text-xl font-bold text-on-surface">{value}</div>
    </div>
  )
}

function StageRow({ stage, active }: { stage: PipelineRunTimeline['stages'][number]; active: boolean }) {
  const meta = stageStatusMeta(stage.status)
  return (
    <div className={`flex items-center gap-3 rounded-lg border px-3 py-2 ${active ? 'border-primary bg-primary/5' : 'border-outline-variant bg-white'}`}>
      <div className={`flex h-8 w-8 items-center justify-center rounded-full ${stage.status === 'succeeded' ? 'bg-green-50 text-green-600' : 'bg-surface-container-high text-on-surface-variant'}`}>
        {stage.status === 'running' ? <SyncOutlined spin /> : stage.attempt}
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold text-on-surface">{stageLabel(stage.stageKey)}</div>
        <div className="truncate text-xs text-on-surface-variant">{stage.stageKey} · attempt {stage.attempt}</div>
      </div>
      <Tag color={meta.color}>{meta.label}</Tag>
    </div>
  )
}

function ArtifactItem({ artifact }: { artifact: Artifact }) {
  return (
    <div className="rounded-lg border border-outline-variant bg-white p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-on-surface">{artifact.title}</div>
          <div className="mt-1 line-clamp-2 text-xs text-on-surface-variant">{artifact.contentText || artifact.artifactType}</div>
        </div>
        <Tag>{artifact.artifactType}</Tag>
      </div>
    </div>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs font-semibold text-on-surface-variant">{label}</div>
      <div className="mt-1 break-words text-sm font-medium text-on-surface">{value}</div>
    </div>
  )
}

function AgentRunItem({ agentRun }: { agentRun: AgentRun }) {
  const ok = agentRun.status === 'succeeded'
  return (
    <div className="rounded-lg border border-outline-variant bg-white p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-on-surface">{agentRun.agentKey}</div>
          <div className="mt-1 truncate text-xs text-on-surface-variant">{agentRun.provider || '-'} · {agentRun.model || '-'}</div>
        </div>
        <Tag color={ok ? 'success' : 'error'}>{ok ? '成功' : '失败'}</Tag>
      </div>
      <div className="mt-2 text-xs text-on-surface-variant">{formatDuration(agentRun.latencyMs)}</div>
    </div>
  )
}

// 新建 Pipeline Modal
interface CreateFormValues {
  title: string
  requirementText: string
  targetRepo: string
  targetBranch: string
}

function CreatePipelineModal({
  open,
  onClose,
  onSubmit,
  loading,
  form,
  githubRepos,
  loadingRepos,
  githubBranches,
  loadingBranches,
  onRepoChange,
  onOpenCreateRepo,
  selectedDocUrls,
  onDocPicker,
  onRemoveDoc,
}: {
  open: boolean
  onClose: () => void
  onSubmit: (values: CreateFormValues) => void
  loading: boolean
  form: ReturnType<typeof Form.useForm<CreateFormValues>>[0]
  githubRepos: {fullName: string; htmlUrl: string; isPrivate?: boolean}[]
  loadingRepos: boolean
  githubBranches: string[]
  loadingBranches: boolean
  onRepoChange: (value: string) => void
  onOpenCreateRepo: () => void
  selectedDocUrls: string[]
  onDocPicker: () => void
  onRemoveDoc: (index: number) => void
}) {
  return (
    <Modal
      title="新建 Pipeline"
      open={open}
      onCancel={onClose}
      footer={null}
      width={520}
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        initialValues={{
          targetRepo: 'self',
          targetBranch: 'main',
        }}
      >
        <Form.Item
          name="title"
          label="流水线名称"
          rules={[{ required: true, message: '请输入流水线名称' }]}
        >
          <Input placeholder="例如：用户信息查询接口" />
        </Form.Item>
        <Form.Item
          name="requirementText"
          label="需求描述"
          tooltip="直接输入需求描述，或选取飞书文档，两者至少选一"
        >
          <Input.TextArea
            rows={4}
            placeholder="详细描述你的需求，例如：创建一个用户信息查询接口，支持根据 userId 查询用户名、头像、年龄等信息..."
          />
        </Form.Item>
        {/* 飞书文档选择 */}
        <Form.Item label="关联飞书文档">
          <Button icon={<FileTextOutlined />} onClick={onDocPicker} size="small">
            选取飞书文档
          </Button>
          {selectedDocUrls.length > 0 && (
            <div className="mt-2">
              <span className="text-xs text-gray-500">已选 {selectedDocUrls.length} 个文档</span>
              <div className="flex flex-wrap gap-1 mt-1">
                {selectedDocUrls.map((url, i) => (
                  <Tag
                    key={i}
                    closable
                    onClose={() => {
                      onRemoveDoc(i)
                    }}
                    color="blue"
                  >
                    {url.split('/').pop()?.slice(0, 20) || '文档'}
                  </Tag>
                ))}
              </div>
            </div>
          )}
        </Form.Item>
        <Form.Item name="targetRepo" label="目标仓库" tooltip="选择已有仓库或输入 owner/repo 格式的新仓库名">
          <div className="flex gap-2">
            <Select
              className="flex-1"
              showSearch
              allowClear
              filterOption={(_input, _option) => true}
              placeholder="选择 GitHub 仓库或输入 owner/repo"
              loading={loadingRepos}
              onChange={onRepoChange}
            >
              <Select.Option value="self">自测 (self)</Select.Option>
              {githubRepos.map(repo => (
                <Select.Option key={repo.fullName} value={repo.fullName}>
                  <span>
                    {repo.fullName}
                    {repo.isPrivate && <Tag className="ml-2" color="orange" style={{ fontSize: '10px', padding: '0 4px' }}>private</Tag>}
                  </span>
                </Select.Option>
              ))}
            </Select>
            <Button onClick={onOpenCreateRepo}>+ 新建</Button>
          </div>
        </Form.Item>
        <Form.Item name="targetBranch" label="目标分支">
          <Select
            showSearch
            allowClear
            placeholder={loadingBranches ? "加载中..." : githubBranches.length > 0 ? "选择已有分支或输入新分支名" : "输入分支名，如 main、develop"}
            loading={loadingBranches}
            disabled={loadingBranches}
            mode={undefined}
          >
            {githubBranches.map(branch => (
              <Select.Option key={branch} value={branch}>
                {branch}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>
        <div className="flex justify-end gap-3 mt-6">
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" htmlType="submit" loading={loading}>
            创建
          </Button>
        </div>
      </Form>
    </Modal>
  )
}

// 飞书文档选择抽屉
function FeishuDocDrawer({
  open,
  onClose,
  wikiSpaces,
  loadingSpaces,
  currentFolder,
  onBack,
  onToggleSelect,
  selectedDocUrls,
}: {
  open: boolean
  onClose: () => void
  wikiSpaces: any[]
  loadingSpaces: boolean
  currentFolder: string
  onBack: () => void
  onToggleSelect: (doc: any) => void
  selectedDocUrls: string[]
}) {
  return (
    <Drawer
      title="选择飞书文档"
      placement="right"
      width={400}
      onClose={onClose}
      open={open}
      footer={
        <div className="flex justify-end">
          <Button type="primary" onClick={onClose}>确定</Button>
        </div>
      }
    >
      {currentFolder && (
        <Button size="small" onClick={onBack} className="mb-3">
          ← 返回上级
        </Button>
      )}
      {loadingSpaces ? (
        <div className="flex justify-center py-8">
          <Spin />
        </div>
      ) : wikiSpaces.length === 0 ? (
        <div className="text-center py-8 text-gray-400">暂无文档</div>
      ) : (
        <div className="space-y-1">
          {wikiSpaces.map((item: any) => {
            const isFolder = item.type === 'folder'
            const url = item.url?.replace('lanshanteam.feishu.cn', 'feishu.cn')
            const isSelected = selectedDocUrls.includes(url)
            return (
              <div
                key={item.token}
                className={`flex items-center gap-2 p-2 rounded cursor-pointer hover:bg-gray-50 ${isSelected ? 'bg-blue-50' : ''}`}
                onClick={() => onToggleSelect(item)}
              >
                <span>{isFolder ? '📁' : '📄'}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm truncate">{item.name}</div>
                  <div className="text-xs text-gray-400">{isFolder ? '文件夹' : '文档'}</div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Drawer>
  )
}
