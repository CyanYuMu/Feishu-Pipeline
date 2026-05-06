export type PipelineRunStatus =
  | 'draft'
  | 'queued'
  | 'running'
  | 'waiting_approval'
  | 'paused'
  | 'failed'
  | 'completed'
  | 'terminated'

export type StageRunStatus =
  | 'pending'
  | 'queued'
  | 'running'
  | 'waiting_approval'
  | 'succeeded'
  | 'failed'
  | 'skipped'

export type CheckpointStatus = 'pending' | 'approved' | 'rejected'
export type AgentRunStatus = 'pending' | 'running' | 'succeeded' | 'failed'
export type GitDeliveryStatus = 'pending' | 'draft' | 'ready' | 'completed' | 'failed'

export interface PipelineRun {
  id: string
  templateId: string
  title: string
  requirementText: string
  sourceSessionId?: string
  targetRepo: string
  targetBranch: string
  workBranch: string
  status: PipelineRunStatus
  currentStageKey: string
  createdBy: string
  startedAt?: string
  finishedAt?: string
  createdAt: string
  updatedAt: string
}

export interface StageRun {
  id: string
  pipelineRunId: string
  stageKey: string
  stageType: string
  status: StageRunStatus
  attempt: number
  inputJson?: string
  outputJson?: string
  errorMessage?: string
  startedAt?: string
  finishedAt?: string
  createdAt: string
  updatedAt: string
}

export interface Artifact {
  id: string
  pipelineRunId: string
  stageRunId?: string
  artifactType: string
  title: string
  contentText?: string
  contentJson?: string
  filePath?: string
  metaJson?: string
  createdAt: string
}

export interface Checkpoint {
  id: string
  pipelineRunId: string
  stageRunId?: string
  checkpointType: string
  status: CheckpointStatus
  approverId?: string
  decision?: string
  comment?: string
  decidedAt?: string
  createdAt: string
  updatedAt: string
}

export interface AgentRun {
  id: string
  pipelineRunId: string
  stageRunId?: string
  agentKey: string
  provider?: string
  model?: string
  promptSnapshot?: string
  inputJson?: string
  outputJson?: string
  tokenUsageJson?: string
  latencyMs: number
  status: AgentRunStatus
  errorMessage?: string
  createdAt: string
  updatedAt: string
}

export interface GitDelivery {
  id: string
  pipelineRunId: string
  provider: string
  repo: string
  baseBranch: string
  headBranch: string
  commitSha?: string
  prmrUrl?: string
  prmrTitle?: string
  prmrBody?: string
  changedFilesJson?: string
  validationJson?: string
  summaryMarkdown?: string
  status: GitDeliveryStatus
  createdAt: string
  updatedAt: string
}

export interface ChangeItem {
  filePath: string
  changeType: 'modify' | 'create' | 'delete'
  reason: string
  originalContent: string
  proposedPatch: string
  proposedDiff: string
  contextIncluded: boolean
}

export interface CodeDiffResponse {
  changeSet: ChangeItem[]
  summary: string
  updatedAt: string
}

export interface PipelineRunSummary {
  totalStages: number
  completedStages: number
  failedStages: number
  waitingApproval: boolean
  currentStageKey: string
  latestArtifactId?: string
  latestDeliveryId?: string
  startedAt?: string
  finishedAt?: string
  durationMs?: number
}

export interface PipelineRunCurrent {
  run: PipelineRun
  stage?: StageRun
  artifact?: Artifact
  checkpoint?: Checkpoint
  agentRun?: AgentRun
  delivery?: GitDelivery
  nextAction: string
}

export interface PipelineRunTimeline {
  run: PipelineRun
  current?: PipelineRunCurrent
  stages: StageRun[]
  artifacts: Artifact[]
  checkpoints: Checkpoint[]
  agentRuns: AgentRun[]
  deliveries: GitDelivery[]
  summary: PipelineRunSummary
}

interface Envelope<T> {
  data?: T
  error?: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  const payload = await res.json().catch(() => ({})) as Envelope<T>
  if (!res.ok) {
    throw new Error(payload.error || `请求失败：${res.status}`)
  }
  if (payload.data === undefined) {
    throw new Error('接口未返回 data')
  }
  return payload.data
}

export async function fetchPipelineRuns(): Promise<PipelineRun[]> {
  return request<PipelineRun[]>('/api/pipeline-runs')
}

export async function fetchPipelineTimeline(runId: string): Promise<PipelineRunTimeline> {
  return request<PipelineRunTimeline>(`/api/pipeline-runs/${runId}/timeline`)
}

export async function fetchPipelineCurrent(runId: string): Promise<PipelineRunCurrent> {
  return request<PipelineRunCurrent>(`/api/pipeline-runs/${runId}/current`)
}

export async function fetchPipelineDeliveries(runId: string): Promise<GitDelivery[]> {
  const data = await request<{ deliveries: GitDelivery[] }>(`/api/pipeline-runs/${runId}/deliveries`)
  return data.deliveries
}


export async function fetchGitDelivery(deliveryId: string): Promise<GitDelivery> {
  return request<GitDelivery>(`/api/git-deliveries/${deliveryId}`)
}

export async function fetchPipelineAgentRuns(runId: string): Promise<AgentRun[]> {
  const data = await request<{ agentRuns: AgentRun[] }>(`/api/pipeline-runs/${runId}/agent-runs`)
  return data.agentRuns
}

export async function fetchCodeDiff(runId: string): Promise<CodeDiffResponse> {
  return request<CodeDiffResponse>(`/api/pipeline-runs/${runId}/code-diff`)
}

export interface SessionMessage {
  id: string
  sessionId: string
  role: string
  content: string
  createdAt: string
  updatedAt: string
}

export interface SessionDetail {
  session: {
    id: string
    title: string
    description: string
    status: string
    createdAt: string
    updatedAt: string
  }
  messages: SessionMessage[]
}

export async function fetchSessionDetail(sessionId: string): Promise<SessionDetail> {
  return request<SessionDetail>(`/api/sessions/${sessionId}`)
}

export async function sendSessionMessage(sessionId: string, content: string): Promise<SessionDetail> {
  return request<SessionDetail>(`/api/sessions/${sessionId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })
}

export async function startPipelineRun(runId: string): Promise<void> {
  await request(`/api/pipeline-runs/${runId}/start`, { method: 'POST' })
}

export async function pausePipelineRun(runId: string): Promise<void> {
  await request(`/api/pipeline-runs/${runId}/pause`, { method: 'POST' })
}

export async function resumePipelineRun(runId: string): Promise<void> {
  await request(`/api/pipeline-runs/${runId}/resume`, { method: 'POST' })
}

export async function terminatePipelineRun(runId: string): Promise<void> {
  await request(`/api/pipeline-runs/${runId}/terminate`, { method: 'POST' })
}

export async function approveCheckpoint(checkpointId: string, comment: string): Promise<Checkpoint> {
  return request<Checkpoint>(`/api/checkpoints/${checkpointId}/approve`, {
    method: 'POST',
    body: JSON.stringify({ comment }),
  })
}

export async function rejectCheckpoint(checkpointId: string, comment: string): Promise<Checkpoint> {
  return request<Checkpoint>(`/api/checkpoints/${checkpointId}/reject`, {
    method: 'POST',
    body: JSON.stringify({ comment }),
  })
}

export interface ChangeSetItem {
  filePath: string
  changeType?: string
  reason?: string
  proposedPatch?: string
  contextIncluded?: boolean
  originalContent?: string
  proposedDiff?: string
}

export interface ExecuteChangesResult {
  appliedFiles: string[]
  failedFiles: { filePath: string; error: string }[]
  summary: string
}

export async function executeChanges(runId: string, changeSet: ChangeSetItem[], token?: string): Promise<ExecuteChangesResult> {
  return request<ExecuteChangesResult>(`/api/pipeline-runs/${runId}/execute-changes`, {
    method: 'POST',
    body: JSON.stringify({ changeSet, token }),
  })
}

export function parseJSON<T>(value?: string): T | null {
  if (!value) return null
  try {
    return JSON.parse(value) as T
  } catch {
    return null
  }
}

export function stageLabel(stageKey?: string): string {
  const labels: Record<string, string> = {
    requirement_analysis: '需求分析',
    solution_design: '方案设计',
    checkpoint_design: '方案审批',
    code_generation: '代码生成',
    test_generation: '测试生成',
    code_review: '代码评审',
    checkpoint_review: '评审确认',
    delivery: '交付集成',
  }
  return stageKey ? labels[stageKey] || stageKey : '未开始'
}

export function checkpointLabel(checkpointType?: string): string {
  const labels: Record<string, string> = {
    design_review: '方案审批',
    code_review: '评审确认',
  }
  return checkpointType ? labels[checkpointType] || checkpointType : '人工审批'
}

export function runStatusMeta(status?: PipelineRunStatus): { label: string; color: string } {
  const meta: Record<PipelineRunStatus, { label: string; color: string }> = {
    draft: { label: '草稿', color: 'default' },
    queued: { label: '排队中', color: 'processing' },
    running: { label: '运行中', color: 'processing' },
    waiting_approval: { label: '待审批', color: 'warning' },
    paused: { label: '已暂停', color: 'default' },
    failed: { label: '失败', color: 'error' },
    completed: { label: '已完成', color: 'success' },
    terminated: { label: '已终止', color: 'default' },
  }
  return status ? meta[status] : { label: '未知', color: 'default' }
}

export function stageStatusMeta(status?: StageRunStatus): { label: string; color: string } {
  const meta: Record<StageRunStatus, { label: string; color: string }> = {
    pending: { label: '待运行', color: 'default' },
    queued: { label: '排队中', color: 'processing' },
    running: { label: '运行中', color: 'processing' },
    waiting_approval: { label: '待审批', color: 'warning' },
    succeeded: { label: '成功', color: 'success' },
    failed: { label: '失败', color: 'error' },
    skipped: { label: '跳过', color: 'default' },
  }
  return status ? meta[status] : { label: '未知', color: 'default' }
}

export function deliveryStatusMeta(status?: GitDeliveryStatus): { label: string; color: string } {
  const meta: Record<GitDeliveryStatus, { label: string; color: string }> = {
    pending: { label: '待生成', color: 'default' },
    draft: { label: '草稿', color: 'default' },
    ready: { label: '可审查', color: 'success' },
    completed: { label: '已交付', color: 'success' },
    failed: { label: '失败', color: 'error' },
  }
  return status ? meta[status] : { label: '未知', color: 'default' }
}

export function nextActionLabel(action?: string): string {
  const labels: Record<string, string> = {
    start_run: '启动流水线',
    wait_execution: '等待执行',
    approve_checkpoint: '处理审批',
    inspect_failure: '查看失败',
    resume_run: '恢复流水线',
    review_delivery: '审查交付',
    completed: '已完成',
    terminated: '已终止',
  }
  return action ? labels[action] || action : '等待数据'
}

/** 待审批且为「评审确认」代码检查点（含代码 Diff），用于工作台右侧审批面板入口 */
export function isCodeApprovalStage(timeline: PipelineRunTimeline | null): boolean {
  if (!timeline || timeline.run.status !== 'waiting_approval') return false
  if (timeline.run.currentStageKey === 'checkpoint_review') return true
  const pending = timeline.checkpoints?.find(c => c.status === 'pending')
  return pending?.checkpointType === 'code_review'
}

export function formatDateTime(value?: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

export function formatDuration(ms?: number): string {
  if (!ms || ms <= 0) return '-'
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const restSeconds = seconds % 60
  if (minutes < 60) return `${minutes}m ${restSeconds}s`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ${minutes % 60}m`
}

export function latestArtifact(timeline?: PipelineRunTimeline): Artifact | undefined {
  if (!timeline?.artifacts.length) return undefined
  return timeline.artifacts[timeline.artifacts.length - 1]
}

export function isLiveRun(status?: PipelineRunStatus): boolean {
  return status === 'queued' || status === 'running'
}
