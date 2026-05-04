import { useState, useEffect } from 'react'
import { useSearch } from '@tanstack/react-router'
import Sidebar from '../components/Sidebar'
import XMarkdown from '@ant-design/x-markdown'
import {
  Card,
  Progress,
  Tag,
  Button,
  Space,
  Spin,
  message,
  Modal,
  Form,
  Input,
  Select,
  Table,
} from 'antd'
import {
  PlusOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  StopOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  PlayCircleOutlined as ExecuteIcon,
} from '@ant-design/icons'

// API Base
const API_BASE = '/api'

// Pipeline Run 类型 (与 API 响应一致，使用 camelCase)
interface PipelineRun {
  id: string
  title: string
  templateId: string
  status: string
  currentStageKey: string
  targetRepo: string
  targetBranch: string
  createdAt: string
  startedAt?: string
  finishedAt?: string
}

// Stage 类型
interface StageRun {
  ID: string
  pipelineRunID: string
  stageKey: string
  stageName: string
  stageType: string
  status: string
  attempt: number
  startedAt?: string
  finishedAt?: string
  error?: string
}

// Pipeline Timeline 类型
interface PipelineTimeline {
  run: PipelineRun
  current?: {
    checkpoint?: { id: string; checkpointType: string; status: string; createdAt: string }
    artifact?: { title: string; contentText: string; contentJson?: string }
  }
  stages: StageRun[]
  artifacts?: { artifactType: string; contentJson?: string }[]
  summary: {
    totalStages: number
    completedStages: number
    failedStages: number
    waitingApproval: boolean
    currentStageKey: string
    startedAt?: string
    finishedAt?: string
    durationMS?: number
  }
}

// 变更项类型
interface ChangeSetItem {
  filePath: string
  changeType?: string
  reason?: string
  proposedPatch?: string
  contextIncluded?: boolean
  originalContent?: string
  proposedDiff?: string
}

// 阶段状态映射
const statusConfig: Record<string, { color: string; icon: string; label: string }> = {
  draft: { color: '#a0aec0', icon: 'more_horiz', label: '待启动' },
  queued: { color: '#3182ce', icon: 'schedule', label: '排队中' },
  running: { color: '#0066ff', icon: 'progress_activity', label: '运行中' },
  paused: { color: '#ed8936', icon: 'pause', label: '已暂停' },
  waiting_approval: { color: '#f6ad55', icon: 'pending', label: '待审批' },
  succeeded: { color: '#48bb78', icon: 'check_circle', label: '已完成' },
  failed: { color: '#fc8181', icon: 'error', label: '失败' },
  terminated: { color: '#a0aec0', icon: 'cancel', label: '已终止' },
}

// 阶段中文名映射
const stageNameMap: Record<string, string> = {
  requirement: '需求分析',
  solution: '方案设计',
  codegen: '代码生成',
  test_generation: '测试生成',
  review: '评审',
  delivery: '交付',
}

export default function Monitoring() {
  const searchParams = useSearch({ from: '/monitoring' })
  const [loading, setLoading] = useState(false)
  const [pipelines, setPipelines] = useState<PipelineRun[]>([])
  const [selectedPipeline, setSelectedPipeline] = useState<PipelineTimeline | null>(null)
  const [_loadingDetail, setLoadingDetail] = useState(false)
  const [createModalVisible, setCreateModalVisible] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [templates, setTemplates] = useState<{ID: string, name: string}[]>([])
  const [form] = Form.useForm()
  
  // 审批相关状态
  const [approvalModalVisible, setApprovalModalVisible] = useState(false)
  const [approvalComment, setApprovalComment] = useState('')
  const [approvalSubmitting, setApprovalSubmitting] = useState(false)
  const [approvalData, setApprovalData] = useState<{
    checkpointId: string
    artifact: { title: string; contentText: string; contentJson?: string } | null
    changeSet: ChangeSetItem[]
  } | null>(null)
  const [showChangeSet, setShowChangeSet] = useState(false)
  const [executingChanges, setExecutingChanges] = useState(false)
  
  // 处理 URL 参数自动打开审批弹窗
  useEffect(() => {
    if (searchParams.runId && searchParams.action === 'approve') {
      openApprovalModal(searchParams.runId)
    }
  }, [])
  
  const sidebarWidth = 80

  // 加载流水线模板列表
  const loadTemplates = async () => {
    try {
      const res = await fetch(`${API_BASE}/pipeline-templates`)
      if (res.ok) {
        const data = await res.json()
        setTemplates(data.data || [])
      }
    } catch (err) {
      console.error('加载模板列表失败:', err)
    }
  }

  // 加载流水线列表
  const loadPipelines = async () => {
    setLoading(true)
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs`)
      if (res.ok) {
        const data = await res.json()
        setPipelines(data.data || [])
      }
    } catch (err) {
      console.error('加载流水线列表失败:', err)
      message.error('加载流水线列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 加载流水线详情
  const loadPipelineDetail = async (id: string) => {
    // 防御性检查：确保 id 是有效的非空字符串
    const safeId = String(id || '').trim()
    if (!safeId || safeId === 'undefined' || safeId === 'null') {
      console.warn('[loadPipelineDetail] Invalid id:', id)
      setSelectedPipeline(null)
      return
    }
    setLoadingDetail(true)
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs/${safeId}/timeline`)
      if (res.ok) {
        const data = await res.json()
        setSelectedPipeline(data.data)
      }
    } catch (err) {
      console.error('加载流水线详情失败:', err)
    } finally {
      setLoadingDetail(false)
    }
  }

  // 启动流水线
  const startPipeline = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs/${id}/start`, { method: 'POST' })
      if (res.ok) {
        message.success('流水线已启动')
        loadPipelineDetail(id)
      } else {
        message.error('启动失败')
      }
    } catch (err) {
      message.error('启动失败')
    }
  }

  // 暂停流水线
  const pausePipeline = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs/${id}/pause`, { method: 'POST' })
      if (res.ok) {
        message.success('流水线已暂停')
        loadPipelineDetail(id)
      } else {
        message.error('暂停失败')
      }
    } catch (err) {
      message.error('暂停失败')
    }
  }

  // 终止流水线
  const terminatePipeline = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs/${id}/terminate`, { method: 'POST' })
      if (res.ok) {
        message.success('流水线已终止')
        loadPipelineDetail(id)
      } else {
        message.error('终止失败')
      }
    } catch (err) {
      message.error('终止失败')
    }
  }

  // 打开审批弹窗
  const openApprovalModal = async (pipelineId: string) => {
    // 先加载流水线详情，确保 selectedPipeline 有值
    await loadPipelineDetail(pipelineId)
    try {
      // 使用 /timeline 接口获取完整数据，包括 checkpoint 和 artifact
      const res = await fetch(`${API_BASE}/pipeline-runs/${pipelineId}/timeline`)
      if (res.ok) {
        const data = await res.json()
        const timeline = data.data
        
        // 查找当前等待审批的 checkpoint (status=pending)
        const waitingCheckpoint = timeline?.checkpoints?.find(
          (cp: any) => cp.status === 'pending'
        )
        
        if (waitingCheckpoint) {
          // 从 timeline 的 artifacts 中找到对应的 artifact
          const checkpointArtifact = timeline?.artifacts?.find(
            (a: any) => a.stageRunId === waitingCheckpoint.stageRunId
          )
          
          let changeSet: ChangeSetItem[] = []
          if (checkpointArtifact?.contentJson) {
            try {
              const jsonData = JSON.parse(checkpointArtifact.contentJson)
              if (Array.isArray(jsonData.changeSet)) {
                changeSet = jsonData.changeSet
              }
            } catch {}
          }
          
          setApprovalData({
            checkpointId: waitingCheckpoint.id,
            artifact: checkpointArtifact ? {
              title: checkpointArtifact.title,
              contentText: checkpointArtifact.contentText,
              contentJson: checkpointArtifact.contentJson,
            } : null,
            changeSet,
          })
          setApprovalModalVisible(true)
        } else {
          message.warning('当前没有待审批的检查点')
        }
      }
    } catch (err) {
      message.error('加载审批数据失败')
    }
  }

  // 审批通过
  const handleApprove = async () => {
    if (!approvalData) return
    setApprovalSubmitting(true)
    try {
      const res = await fetch(`${API_BASE}/checkpoints/${approvalData.checkpointId}/approve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ comment: approvalComment }),
      })
      if (res.ok) {
        message.success('审批已通过')
        setApprovalModalVisible(false)
        setApprovalComment('')
        loadPipelineDetail(selectedPipeline?.run.id || '')
      } else {
        message.error('审批失败')
      }
    } catch {
      message.error('审批失败')
    } finally {
      setApprovalSubmitting(false)
    }
  }

  // 审批驳回
  const handleReject = async () => {
    if (!approvalData) return
    setApprovalSubmitting(true)
    try {
      const res = await fetch(`${API_BASE}/checkpoints/${approvalData.checkpointId}/reject`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ comment: approvalComment }),
      })
      if (res.ok) {
        message.success('已驳回')
        setApprovalModalVisible(false)
        setApprovalComment('')
        loadPipelineDetail(selectedPipeline?.run.id || '')
      } else {
        message.error('驳回失败')
      }
    } catch {
      message.error('驳回失败')
    } finally {
      setApprovalSubmitting(false)
    }
  }

  const handleExecuteChanges = async () => {
    if (!approvalData || !selectedPipeline) return
    setApprovalSubmitting(true)
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs/${selectedPipeline.run.id}/execute-changes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ changeSet: approvalData.changeSet }),
      })
      if (res.ok) {
        const result = await res.json()
        message.success(result.data?.summary || '变更执行完成')
        setApprovalModalVisible(false)
      } else {
        message.error('执行变更失败')
      }
    } catch {
      message.error('执行变更失败')
    } finally {
      setApprovalSubmitting(false)
    }
  }

  // 直接执行变更（无需先审批）
  const executeChangesDirectly = async (pipelineId: string) => {
    try {
      setExecutingChanges(true)
      // 获取流水线当前的变更计划
      const res = await fetch(`${API_BASE}/pipeline-runs/${pipelineId}/timeline`)
      if (!res.ok) {
        message.error('获取变更计划失败')
        return
      }
      const data = await res.json()
      const timeline = data.data
      
      // 找到 codegen 阶段的 artifact 获取 changeSet
      const codegenArtifact = timeline?.artifacts?.find(
        (a: any) => a.stageKey === 'codegen' || a.artifactType === 'code_diff'
      )
      
      let changeSet: ChangeSetItem[] = []
      if (codegenArtifact?.contentJson) {
        try {
          const jsonData = JSON.parse(codegenArtifact.contentJson)
          if (Array.isArray(jsonData.changeSet)) {
            changeSet = jsonData.changeSet
          } else if (Array.isArray(jsonData)) {
            changeSet = jsonData
          }
        } catch {}
      }
      
      if (changeSet.length === 0) {
        message.warning('未找到变更计划')
        return
      }
      
      // 执行变更
      const execRes = await fetch(`${API_BASE}/pipeline-runs/${pipelineId}/execute-changes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ changeSet }),
      })
      
      if (execRes.ok) {
        const result = await execRes.json()
        message.success(result.data?.summary || '变更执行完成')
        loadPipelineDetail(pipelineId)
      } else {
        message.error('执行变更失败')
      }
    } catch {
      message.error('执行变更失败')
    } finally {
      setExecutingChanges(false)
    }
  }

  // 创建流水线
  const createPipeline = async (values: { title: string; templateID: string; requirementText: string; targetRepo?: string; targetBranch?: string }) => {
    setCreateLoading(true)
    try {
      const res = await fetch(`${API_BASE}/pipeline-runs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(values),
      })
      if (res.ok) {
        const data = await res.json()
        message.success('流水线创建成功')
        setCreateModalVisible(false)
        form.resetFields()
        loadPipelines()
        // 自动打开新创建的流水线详情
        if (data.data?.run?.id || data.data?.id) {
          const newId = data.data.run?.id || data.data.id
          loadPipelineDetail(newId)
        }
      } else {
        const errData = await res.json().catch(() => ({}))
        message.error(errData.error || '创建失败')
      }
    } catch (err) {
      message.error('创建失败')
    } finally {
      setCreateLoading(false)
    }
  }

  useEffect(() => {
    loadPipelines()
    loadTemplates()
  }, [])

  // 格式化耗时
  const formatDuration = (ms?: number) => {
    if (!ms) return '--'
    const seconds = Math.floor(ms / 1000)
    if (seconds < 60) return `${seconds}s`
    const minutes = Math.floor(seconds / 60)
    const remainingSeconds = seconds % 60
    if (minutes < 60) return `${minutes}m ${remainingSeconds}s`
    const hours = Math.floor(minutes / 60)
    return `${hours}h ${minutes % 60}m`
  }

  // 获取状态配置
  const getStageStatus = (stage: StageRun) => {
    return statusConfig[stage.status] || statusConfig.draft
  }

  // 获取阶段状态 (done/running/pending)
  const getPipelineStepStatus = (stage: StageRun) => {
    if (stage.status === 'succeeded') return 'done'
    if (stage.status === 'running' || stage.status === 'queued' || stage.status === 'waiting_approval') return 'running'
    return 'pending'
  }

  return (
    <div className="min-h-screen bg-background">
      <Sidebar />
      <main className="h-screen overflow-y-auto p-6 transition-all duration-300" style={{ marginLeft: `${sidebarWidth}px` }}>
        <div className="flex justify-between items-start mb-5">
          <div>
            <div className="text-sm text-on-surface-variant mb-1">
              Pipeline <span className="text-on-surface/30">›</span> 流水线监控
            </div>
            <h1 className="text-2xl font-bold text-on-surface m-0">Pipeline Status Monitor</h1>
          </div>
          <Space>
            <Button 
              type="primary" 
              icon={<PlusOutlined />} 
              size="large" 
              className="!rounded-lg"
              onClick={() => setCreateModalVisible(true)}
            >
              创建流水线
            </Button>
            <Button 
              icon={<ReloadOutlined />} 
              size="large" 
              className="!rounded-lg"
              onClick={loadPipelines}
              loading={loading}
            >
              刷新
            </Button>
          </Space>
        </div>

        <div className="grid grid-cols-[320px_1fr] gap-5">
          {/* 左侧流水线列表 */}
          <Card className="!rounded-xl !shadow-sm" title={<span className="text-xs font-bold text-on-surface-variant tracking-wider">流水线列表</span>}>
            {loading ? (
              <div className="flex justify-center py-8">
                <Spin />
              </div>
            ) : pipelines.length === 0 ? (
              <div className="text-center py-8 text-on-surface-variant">
                暂无流水线
              </div>
            ) : (
              <div className="space-y-2 max-h-[calc(100vh-280px)] overflow-y-auto">
                {pipelines.map((pipeline) => {
                  const config = statusConfig[pipeline.status] || statusConfig.draft
                  const isSelected = selectedPipeline?.run?.id === pipeline.id
                  return (
                    <div
                      key={pipeline.id}
                      className={`p-3 rounded-lg cursor-pointer transition-all ${
                        isSelected 
                          ? 'bg-primary/10 border border-primary' 
                          : 'bg-surface-container-low hover:bg-surface-container-high'
                      }`}
                      onClick={() => loadPipelineDetail(pipeline.id)}
                    >
                      <div className="flex justify-between items-start mb-1">
                        <div className="font-medium text-on-surface text-sm truncate max-w-[180px]" title={pipeline.title}>
                          {pipeline.title || '未命名流水线'}
                        </div>
                        <Tag className="!text-xs" style={{ backgroundColor: config.color + '20', color: config.color }}>
                          {config.label}
                        </Tag>
                      </div>
                      <div className="text-xs text-on-surface-variant">
                        {pipeline.targetRepo}/{pipeline.targetBranch}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>

          {/* 右侧详情 */}
          <div className="space-y-5">
            {selectedPipeline ? (
              <>
                {/* 流水线概览 */}
                <Card className="!rounded-xl !shadow-sm">
                  <div className="flex justify-between items-start mb-5">
                    <div className="flex items-center gap-3">
                      <div className="w-12 h-12 rounded-lg bg-surface-container-high flex items-center justify-center">
                        <span className="material-symbols-outlined text-primary text-2xl">account_tree</span>
                      </div>
                      <div>
                        <div className="font-semibold text-on-surface">{selectedPipeline.run.title}</div>
                        <div className="text-sm text-on-surface-variant">
                          {selectedPipeline.run.targetRepo}/{selectedPipeline.run.targetBranch}
                        </div>
                      </div>
                    </div>
                    <div className="text-right">
                      <Tag 
                        className="!text-xs !font-semibold"
                        style={{ 
                          backgroundColor: (statusConfig[selectedPipeline.run.status]?.color || '#a0aec0') + '20',
                          color: statusConfig[selectedPipeline.run.status]?.color || '#a0aec0'
                        }}
                      >
                        {statusConfig[selectedPipeline.run.status]?.label || selectedPipeline.run.status}
                      </Tag>
                      <div className="text-xs text-on-surface-variant mt-1">
                        耗时: {formatDuration(selectedPipeline.summary.durationMS)}
                      </div>
                    </div>
                  </div>
                  
                  {/* 进度条 */}
                  <div className="mb-5">
                    <div className="flex justify-between items-center mb-2">
                      <span className="text-sm text-on-surface-variant">执行进度</span>
                      <span className="text-sm font-semibold text-on-surface">
                        {selectedPipeline.summary.completedStages}/{selectedPipeline.summary.totalStages} 阶段
                      </span>
                    </div>
                    <Progress 
                      percent={Math.round((selectedPipeline.summary.completedStages / selectedPipeline.summary.totalStages) * 100)} 
                      strokeColor="#0066ff" 
                      trailColor="#dbe8f6" 
                      showInfo={false} 
                    />
                  </div>

                  {/* 操作按钮 */}
                  <div className="flex gap-2">
                    {selectedPipeline.run.status === 'draft' && (
                      <Button 
                        type="primary" 
                        icon={<PlayCircleOutlined />} 
                        onClick={() => startPipeline(selectedPipeline.run.id)}
                      >
                        启动
                      </Button>
                    )}
                    {selectedPipeline.run.status === 'running' && (
                      <Button 
                        icon={<PauseCircleOutlined />} 
                        onClick={() => pausePipeline(selectedPipeline.run.id)}
                      >
                        暂停
                      </Button>
                    )}
                    {/* 交付阶段显示执行变更按钮 */}
                    {selectedPipeline.run.status === 'running' && 
                     selectedPipeline.summary.currentStageKey === 'delivery' && (
                      <Button 
                        type="primary"
                        icon={<PlayCircleOutlined />} 
                        onClick={() => executeChangesDirectly(selectedPipeline.run.id)}
                        loading={executingChanges}
                      >
                        执行变更
                      </Button>
                    )}
                    {['running', 'queued', 'paused'].includes(selectedPipeline.run.status) && (
                      <Button 
                        danger 
                        icon={<StopOutlined />} 
                        onClick={() => terminatePipeline(selectedPipeline.run.id)}
                      >
                        终止
                      </Button>
                    )}
                    {selectedPipeline.run.status === 'waiting_approval' && (
                      <Button 
                        type="primary" 
                        icon={<CheckCircleOutlined />} 
                        onClick={() => openApprovalModal(selectedPipeline.run.id)}
                      >
                        审批
                      </Button>
                    )}
                  </div>
                </Card>

                {/* 执行流水线 */}
                <Card className="!rounded-xl !shadow-sm">
                  <div className="flex items-center gap-2 mb-5">
                    <span className="material-symbols-outlined text-primary text-lg">account_tree</span>
                    <span className="font-semibold text-on-surface">执行 Pipeline</span>
                  </div>
                  <div className="space-y-4">
                    {selectedPipeline.stages.map((stage, idx) => {
                      const stepStatus = getPipelineStepStatus(stage)
                      const stageConfig = getStageStatus(stage)
                      return (
                        <div key={stage.ID} className="flex gap-4">
                          <div className="flex flex-col items-center">
                            <div className={`w-10 h-10 rounded-full flex items-center justify-center ${
                              stepStatus === 'done' ? 'bg-green-100' : 
                              stepStatus === 'running' ? 'bg-primary/10' : 'bg-surface-container-low'
                            }`}>
                              {stepStatus === 'done' ? (
                                <span className="material-symbols-outlined text-green-500 text-xl">check_circle</span>
                              ) : stepStatus === 'running' ? (
                                <span className="material-symbols-outlined text-primary animate-spin" style={{ animationDuration: '2s' }}>progress_activity</span>
                              ) : (
                                <span className="material-symbols-outlined text-on-surface-variant text-xl">more_horiz</span>
                              )}
                            </div>
                            {idx < selectedPipeline.stages.length - 1 && (
                              <div className="w-0.5 flex-1 bg-outline-variant my-1" />
                            )}
                          </div>
                          <div className="flex-1 pb-5">
                            <div className="flex justify-between items-start mb-1">
                              <div>
                                <div className="font-semibold text-on-surface">
                                  {stageNameMap[stage.stageKey] || stage.stageName || stage.stageKey}
                                </div>
                                <div className="text-sm text-on-surface-variant mt-0.5">
                                  {stage.stageType} · 第 {stage.attempt} 次执行
                                </div>
                              </div>
                              <Tag 
                                className="!text-xs"
                                style={{ 
                                  backgroundColor: stageConfig.color + '20',
                                  color: stageConfig.color
                                }}
                              >
                                {stageConfig.label}
                              </Tag>
                            </div>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </Card>
              </>
            ) : (
              <Card className="!rounded-xl !shadow-sm">
                <div className="flex flex-col items-center justify-center py-16 text-on-surface-variant">
                  <span className="material-symbols-outlined text-4xl mb-4 opacity-50">touch_app</span>
                  <div>请选择一个流水线查看详情</div>
                </div>
              </Card>
            )}
          </div>
        </div>

        {/* 创建流水线弹窗 */}
        <Modal
          title="创建新流水线"
          open={createModalVisible}
          onCancel={() => {
            setCreateModalVisible(false)
            form.resetFields()
          }}
          footer={null}
          width={600}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={createPipeline}
            className="mt-4"
          >
            <Form.Item
              name="title"
              label="流水线名称"
              rules={[{ required: true, message: '请输入流水线名称' }]}
            >
              <Input placeholder="请输入流水线名称" />
            </Form.Item>
            <Form.Item
              name="templateID"
              label="选择模板"
              rules={[{ required: true, message: '请选择模板' }]}
            >
              <Select placeholder="请选择模板">
                {templates.map((t, idx) => (
                  <Select.Option key={t?.ID || `tmpl-${idx}`} value={t?.ID || ''}>
                    {t?.name || t?.ID || '未知模板'}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item
              name="requirementText"
              label="需求描述"
              rules={[{ required: true, message: '请输入需求描述' }]}
            >
              <Input.TextArea 
                rows={4} 
                placeholder="请用自然语言描述需求，例如：实现一个用户登录功能，包含用户名密码校验" 
              />
            </Form.Item>
            <Form.Item name="targetRepo" label="目标仓库（可选）">
              <Input placeholder="默认为 self" />
            </Form.Item>
            <Form.Item name="targetBranch" label="目标分支（可选）">
              <Input placeholder="默认为 main" />
            </Form.Item>
            <Form.Item className="!mb-0">
              <div className="flex justify-end gap-2">
                <Button onClick={() => {
                  setCreateModalVisible(false)
                  form.resetFields()
                }}>
                  取消
                </Button>
                <Button type="primary" htmlType="submit" loading={createLoading}>
                  创建
                </Button>
              </div>
            </Form.Item>
          </Form>
        </Modal>

        {/* 审批弹窗 */}
        <Modal
          title={`审批 - ${approvalData?.artifact?.title || '代码变更计划'}`}
          open={approvalModalVisible}
          onCancel={() => {
            setApprovalModalVisible(false)
            setApprovalComment('')
          }}
          width={900}
          footer={null}
        >
          <div className="space-y-4">
            {/* 审批意见 */}
            <div>
              <label className="block text-sm font-medium text-on-surface-variant mb-1">审批意见</label>
              <Input.TextArea
                value={approvalComment}
                onChange={(e) => setApprovalComment(e.target.value)}
                placeholder="输入审批意见..."
                rows={3}
              />
            </div>

            {/* 操作按钮 */}
            <div className="flex justify-between">
              <div>
                {approvalData && approvalData.changeSet.length > 0 && (
                  <Space>
                    <Button
                      icon={<ExecuteIcon />}
                      onClick={() => setShowChangeSet(!showChangeSet)}
                    >
                      {showChangeSet ? '隐藏变更' : '查看变更'}
                    </Button>
                    <Button
                      type="default"
                      icon={<PlayCircleOutlined />}
                      onClick={handleExecuteChanges}
                      loading={approvalSubmitting}
                    >
                      执行变更
                    </Button>
                  </Space>
                )}
              </div>
              <Space>
                <Button
                  danger
                  icon={<CloseCircleOutlined />}
                  onClick={handleReject}
                  loading={approvalSubmitting}
                >
                  驳回
                </Button>
                <Button
                  type="primary"
                  icon={<CheckCircleOutlined />}
                  onClick={handleApprove}
                  loading={approvalSubmitting}
                >
                  通过
                </Button>
              </Space>
            </div>

            {/* 审批报告内容 */}
            {approvalData?.artifact?.contentText && (
              <div className="mt-4">
                <div className="text-sm font-medium text-on-surface-variant mb-2">审批报告</div>
                <div className="border border-outline-variant rounded-lg p-4 bg-surface-container-low max-h-64 overflow-y-auto">
                  <XMarkdown
                    className="prose prose-sm max-w-none"
                    theme={{
                      h1: 'text-lg font-bold text-on-surface mt-3 mb-2',
                      h2: 'text-base font-bold text-on-surface mt-3 mb-2',
                      h3: 'text-sm font-semibold text-on-surface mt-2 mb-1',
                      p: 'text-sm text-on-surface-variant leading-relaxed mb-2',
                      ul: 'list-disc pl-5 mb-2 space-y-1',
                      ol: 'list-decimal pl-5 mb-2 space-y-1',
                      li: 'text-sm text-on-surface-variant',
                      code: 'bg-surface-container-high px-1 py-0.5 rounded text-xs font-mono',
                      pre: 'bg-surface-container-high p-3 rounded-lg overflow-x-auto mb-2',
                      blockquote: 'border-l-4 border-primary pl-4 italic text-on-surface-variant',
                      table: 'w-full border-collapse mb-2',
                      th: 'border border-outline-variant p-2 bg-surface-container-high text-left',
                      td: 'border border-outline-variant p-2',
                    }}
                    {...({ content: approvalData.artifact.contentText } as any)}
                  />
                </div>
              </div>
            )}

            {/* 变更预览表格 */}
            {showChangeSet && approvalData && approvalData.changeSet.length > 0 && (
              <Table
                size="small"
                dataSource={approvalData.changeSet.map((item, idx) => ({ ...item, key: idx }))}
                columns={[
                  { title: '文件', dataIndex: 'filePath', key: 'filePath', width: 200, ellipsis: true },
                  { 
                    title: '类型', 
                    dataIndex: 'changeType', 
                    key: 'changeType',
                    width: 80,
                    render: (type: string) => (
                      <Tag color={type === 'create' ? 'green' : 'blue'}>{type || 'modify'}</Tag>
                    )
                  },
                  { title: '说明', dataIndex: 'reason', key: 'reason', ellipsis: true }
                ]}
                pagination={false}
                scroll={{ y: 300 }}
              />
            )}
          </div>
        </Modal>
      </main>
    </div>
  )
}