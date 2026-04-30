import { useEffect, useState } from 'react'
import { Card, Empty, List, Tag, Button, message, Space, Typography, Collapse, Table, Modal, Input, Checkbox, Pagination } from 'antd'
import { ArrowLeftOutlined, FolderOutlined, LinkOutlined, FileTextOutlined, ApiOutlined, PlusOutlined, EditOutlined, GithubOutlined } from '@ant-design/icons'
import Sidebar from '../components/Sidebar'
import { useNavigate } from '@tanstack/react-router'

const { Title, Text, Paragraph } = Typography
const { Panel } = Collapse

const mainMarginLeft = 336
const API_BASE = ''

interface Project {
  id: string
  title: string
  description: string
  swaggerUrl: string
  docUrls: string[]
  githubRepo: string
  githubRepos: string[]  // 多个仓库
  createdAt: string
  updatedAt: string
}

interface GitHubRepo {
  id: number
  name: string
  full_name: string
  private: boolean
  html_url: string
  description: string
}

interface OpenAPISpec {
  id: string
  projectId: string
  title: string
  description: string
  specJson: string
  swaggerUrl: string
  docUrls: string[]
  createdAt: string
}

interface ParsedSpec {
  openapi?: string
  info?: {
    title?: string
    description?: string
    version?: string
  }
  servers?: Array<{ url: string; description?: string }>
  paths?: Record<string, {
    get?: { summary?: string; description?: string; tags?: string[] }
    post?: { summary?: string; description?: string; tags?: string[] }
    put?: { summary?: string; description?: string; tags?: string[] }
    delete?: { summary?: string; description?: string; tags?: string[] }
    patch?: { summary?: string; description?: string; tags?: string[] }
  }>
  components?: {
    schemas?: Record<string, {
      type?: string
      description?: string
      properties?: Record<string, { type?: string; description?: string }>
    }>
  }
}

export default function Assets() {
  const navigate = useNavigate()
  
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedProject, setSelectedProject] = useState<Project | null>(null)
  const [specs, setSpecs] = useState<OpenAPISpec[]>([])
  const [createModalVisible, setCreateModalVisible] = useState(false)
  const [newProjectTitle, setNewProjectTitle] = useState('')
  const [editingProject, setEditingProject] = useState(false)
  const [editingTitle, setEditingTitle] = useState('')
  const [editingDesc, setEditingDesc] = useState('')
  const [editingGitHubRepos, setEditingGitHubRepos] = useState<string[]>([])
  const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([])
  const [githubReposLoading, setGithubReposLoading] = useState(false)
  const [githubReposModalVisible, setGithubReposModalVisible] = useState(false)
  const [githubReposPage, setGithubReposPage] = useState(1)
  const githubReposPageSize = 10

  // 加载 GitHub 仓库列表
  const loadGitHubRepos = async () => {
    setGithubReposLoading(true)
    try {
      const res = await fetch('/api/github/repos', { credentials: 'include' })
      if (res.ok) {
        const data = await res.json()
        setGithubRepos(data.data || [])
      } else {
        message.error('获取 GitHub 仓库失败，请确保已绑定 GitHub 账号')
      }
    } catch (err) {
      console.error('加载 GitHub 仓库失败:', err)
      message.error('获取 GitHub 仓库失败')
    } finally {
      setGithubReposLoading(false)
    }
  }

  // 打开 GitHub 仓库选择弹窗
  const openGitHubRepoSelector = async () => {
    setGithubReposPage(1)
    await loadGitHubRepos()
    setGithubReposModalVisible(true)
  }

  // 打开 GitHub 仓库编辑器（从关联仓库卡片）
  const openGitHubRepoEditor = async () => {
    if (selectedProject) {
      const repos = selectedProject.githubRepo ? selectedProject.githubRepo.split(',').filter(Boolean) : []
      setEditingGitHubRepos(repos)
    }
    await openGitHubRepoSelector()
  }

  // 切换仓库选择
  const toggleGitHubRepo = (repoFullName: string) => {
    setEditingGitHubRepos(prev =>
      prev.includes(repoFullName)
        ? prev.filter(r => r !== repoFullName)
        : [...prev, repoFullName]
    )
  }

  // 确认选择
  const confirmGitHubRepos = async () => {
    if (!selectedProject) return
    
    const newGitHubRepo = editingGitHubRepos.join(',')
    try {
      const res = await fetch(`/api/projects/${selectedProject.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ github_repo: newGitHubRepo })
      })
      if (res.ok) {
        setSelectedProject({ ...selectedProject, githubRepo: newGitHubRepo })
        setGithubReposModalVisible(false)
        message.success('仓库绑定已更新')
      }
    } catch (err) {
      console.error('保存失败:', err)
      message.error('保存失败')
    }
  }

  // 进入编辑模式
  const startEdit = () => {
    if (selectedProject) {
      setEditingTitle(selectedProject.title)
      setEditingDesc(selectedProject.description)
      setEditingProject(true)
    }
  }

  // 保存编辑
  const saveEdit = async () => {
    if (!selectedProject || !editingTitle.trim()) return
    try {
      const res = await fetch(`${API_BASE}/api/projects/${selectedProject.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: editingTitle, description: editingDesc })
      })
      if (res.ok) {
        setSelectedProject({ ...selectedProject, title: editingTitle, description: editingDesc })
        setEditingProject(false)
        loadProjects()
      }
    } catch (err) {
      console.error('保存失败:', err)
    }
  }

  // 取消编辑
  const cancelEdit = () => setEditingProject(false)

  // 跳转到创建需求
  const goToNewRequirement = () => {
    if (selectedProject) {
      navigate({ to: '/new-requirement', search: { projectId: selectedProject.id } })
    }
  }

  // 加载项目列表
  const loadProjects = async () => {
    setLoading(true)
    try {
      const res = await fetch(`${API_BASE}/api/projects`)
      if (res.ok) {
        const data = await res.json()
        if (data.data) {
          const projectList: Project[] = data.data.map((p: any) => ({
            id: p.ID || p.id,
            title: p.Title || p.title || '未命名项目',
            description: p.Description || p.description || '',
            swaggerUrl: p.SwaggerURL || p.swaggerUrl || `${window.location.origin}/swagger?projectId=${p.ID || p.id}`,
            docUrls: p.DocUrls || p.docUrls ? JSON.parse(p.DocUrls || p.docUrls) : [],
            githubRepo: p.GitHubRepo || p.githubRepo || '',
            createdAt: p.CreatedAt || p.createdAt,
            updatedAt: p.UpdatedAt || p.updatedAt,
          }))
          setProjects(projectList)
        }
      }
    } catch (err) {
      console.error('加载项目失败:', err)
      message.error('加载项目失败')
    } finally {
      setLoading(false)
    }
  }

  // 加载项目下的 API 文档
  const loadProjectSpecs = async (projectId: string) => {
    try {
      const res = await fetch(`${API_BASE}/api/projects/${projectId}/specs`)
      if (res.ok) {
        const data = await res.json()
        if (data.data) {
          const specList: OpenAPISpec[] = data.data.map((s: any) => ({
            id: s.id,
            projectId: s.projectId || '',
            title: s.title || '未命名API',
            description: s.description || '',
            specJson: s.specJson || '{}',
            swaggerUrl: s.swaggerUrl || `${window.location.origin}/swagger?specId=${s.id}`,
            docUrls: s.docUrls ? JSON.parse(s.docUrls) : [],
            createdAt: s.createdAt,
          }))
          setSpecs(specList)
        }
      }
    } catch (err) {
      console.error('加载 API 文档失败:', err)
      message.error('加载 API 文档失败')
    }
  }

  // 创建项目
  const handleCreateProject = async () => {
    if (!newProjectTitle.trim()) {
      message.warning('请输入项目名称')
      return
    }
    try {
      const res = await fetch(`${API_BASE}/api/projects`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: newProjectTitle }),
      })
      if (res.ok) {
        message.success('项目创建成功')
        setCreateModalVisible(false)
        setNewProjectTitle('')
        loadProjects()
      } else {
        message.error('创建失败')
      }
    } catch (err) {
      console.error('创建项目失败:', err)
      message.error('创建失败')
    }
  }

  useEffect(() => {
    loadProjects()
  }, [])

  const handleSelectProject = async (project: Project) => {
    setSelectedProject(project)
    await loadProjectSpecs(project.id)
  }

  const handleBack = () => {
    setSelectedProject(null)
    setSpecs([])
  }

  // 解析 OpenAPI 规范
  const parseSpec = (specJson: string): ParsedSpec => {
    try {
      return JSON.parse(specJson)
    } catch {
      return {}
    }
  }

  // 渲染 API 端点表格
  const renderEndpointsTable = (specJson: string) => {
    const spec = parseSpec(specJson)
    if (!spec.paths) return <Empty description="暂无接口" />

    const endpoints: Array<{
      method: string
      path: string
      summary: string
      tags: string[]
    }> = []

    Object.entries(spec.paths).forEach(([path, methods]) => {
      ;(['get', 'post', 'put', 'delete', 'patch'] as const).forEach(method => {
        const operation = methods[method]
        if (operation) {
          endpoints.push({
            method: method.toUpperCase(),
            path,
            summary: operation.summary || '',
            tags: operation.tags || [],
          })
        }
      })
    })

    const columns = [
      { title: '方法', dataIndex: 'method', key: 'method', width: 80,
        render: (method: string) => <Tag color={method === 'GET' ? 'green' : method === 'POST' ? 'blue' : method === 'PUT' ? 'orange' : method === 'DELETE' ? 'red' : 'purple'}>{method}</Tag> },
      { title: '路径', dataIndex: 'path', key: 'path' },
      { title: '描述', dataIndex: 'summary', key: 'summary' },
    ]

    return <Table dataSource={endpoints} columns={columns} rowKey="path" size="small" pagination={{ pageSize: 10 }} />
  }

  // 渲染数据模型
  const renderSchemas = (specJson: string) => {
    const spec = parseSpec(specJson)
    if (!spec.components?.schemas) return <Empty description="暂无数据模型" />

    return (
      <Collapse>
        {Object.entries(spec.components.schemas).map(([name, schema]) => (
          <Panel key={name} header={<Tag>{name}</Tag>}>
            <Text type="secondary">{schema.description || '无描述'}</Text>
            {schema.properties && (
              <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                {Object.entries(schema.properties).map(([prop, info]) => (
                  <li key={prop}><Text code>{prop}</Text>: {info.type} {info.description && `- ${info.description}`}</li>
                ))}
              </ul>
            )}
          </Panel>
        ))}
      </Collapse>
    )
  }

  // 渲染文档列表（统一列表，用标签区分类型）
  const renderDocUrls = (docUrls: string[], swaggerUrls?: string[]) => {
    const allDocs: Array<{ url: string; type: 'feishu' | 'swagger' }> = []
    
    // 添加飞书文档
    if (docUrls && docUrls.length > 0) {
      docUrls.forEach(url => allDocs.push({ url, type: 'feishu' }))
    }
    
    // 添加 Swagger UI
    if (swaggerUrls && swaggerUrls.length > 0) {
      swaggerUrls.forEach(url => allDocs.push({ url, type: 'swagger' }))
    }
    
    if (allDocs.length === 0) return <Empty description="暂无关联文档" />

    return (
      <List
        bordered
        dataSource={allDocs}
        renderItem={(item) => (
          <List.Item 
            actions={[
              <Button 
                key="open" 
                icon={<LinkOutlined />} 
                href={item.url} 
                target="_blank"
              >
                打开
              </Button>
            ]}
          >
            <List.Item.Meta
              avatar={
                item.type === 'swagger' 
                  ? <ApiOutlined style={{ fontSize: 20, color: '#52c41a' }} />
                  : <FileTextOutlined style={{ fontSize: 20, color: '#1890ff' }} />
              }
              title={
                <Space>
                  <a href={item.url} target="_blank">
                    {item.type === 'swagger' ? 'Swagger UI' : (item.url.split('/').pop() || '文档')}
                  </a>
                  <Tag color={item.type === 'swagger' ? 'green' : 'blue'}>
                    {item.type === 'swagger' ? 'API' : '飞书'}
                  </Tag>
                </Space>
              }
              description={<Text ellipsis>{item.url}</Text>}
            />
          </List.Item>
        )}
      />
    )
  }

  // 项目列表视图
  const renderProjectList = () => (
    <div className="flex-1 overflow-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <Title level={4}>项目列表</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalVisible(true)}>
          新建项目
        </Button>
      </div>

      <List
        loading={loading}
        dataSource={projects}
        renderItem={(project: Project) => (
          <Card
            hoverable
            className="mb-4"
            onClick={() => handleSelectProject(project)}
            extra={<Tag color="blue">API 文档</Tag>}
          >
            <Card.Meta
              avatar={<FolderOutlined style={{ fontSize: 32, color: '#1890ff' }} />}
              title={<Text strong>{project.title}</Text>}
              description={project.description || '暂无描述'}
            />
            <div className="mt-4">
              <Space>
                {project.docUrls?.length > 0 && (
                  <Tag>{project.docUrls.length} 个文档</Tag>
                )}
              </Space>
            </div>
          </Card>
        )}
      />
    </div>
  )

  // 项目详情视图
  const renderProjectDetail = () => (
    <div className="flex-1 overflow-auto p-6">
      <Button icon={<ArrowLeftOutlined />} onClick={handleBack} className="mb-4">
        返回项目列表
      </Button>

      <Card className="mb-4">
        {editingProject ? (
          <div>
            <Input
              value={editingTitle}
              onChange={(e) => setEditingTitle(e.target.value)}
              placeholder="项目名称"
              className="mb-2"
            />
            <Input.TextArea
              value={editingDesc}
              onChange={(e) => setEditingDesc(e.target.value)}
              placeholder="项目描述"
              rows={2}
              className="mb-2"
            />
            <Space>
              <Button type="primary" onClick={saveEdit}>保存</Button>
              <Button onClick={cancelEdit}>取消</Button>
            </Space>
          </div>
        ) : (
          <div>
            <Space align="center">
              <Title level={4} className="m-0">{selectedProject?.title}</Title>
              <Button icon={<EditOutlined />} type="text" onClick={startEdit} />
            </Space>
            <Paragraph type="secondary">{selectedProject?.description || '暂无描述'}</Paragraph>
            <Button type="primary" icon={<PlusOutlined />} onClick={goToNewRequirement}>
              创建需求
            </Button>
          </div>
        )}
      </Card>

      {/* 关联文档（飞书文档 + Swagger UI）*/}
      <Card title="关联文档" className="mb-4">
        {renderDocUrls(
          selectedProject?.docUrls || [], 
          [
            ...(selectedProject?.swaggerUrl ? [selectedProject.swaggerUrl] : []),
            ...specs.map(s => s.swaggerUrl).filter(Boolean)
          ]
        )}
      </Card>

      {/* 关联仓库 */}
      <Card 
        title="关联仓库" 
        className="mb-4"
        extra={
          <Button icon={<EditOutlined />} size="small" onClick={openGitHubRepoEditor}>
            编辑
          </Button>
        }
      >
        {selectedProject?.githubRepo ? (
          <List
            dataSource={selectedProject.githubRepo.split(',').filter(Boolean)}
            renderItem={(repo: string) => (
              <List.Item 
                actions={[
                  <Button 
                    key="open" 
                    icon={<GithubOutlined />} 
                    href={`https://github.com/${repo}`} 
                    target="_blank"
                  >
                    打开
                  </Button>
                ]}
              >
                <List.Item.Meta
                  avatar={<GithubOutlined style={{ fontSize: 20, color: '#333' }} />}
                  title={
                    <a href={`https://github.com/${repo}`} target="_blank">
                      {repo}
                    </a>
                  }
                  description={`https://github.com/${repo}`}
                />
              </List.Item>
            )}
          />
        ) : (
          <Empty description="暂无关联仓库" />
        )}
      </Card>

      {/* 选中的API 详情 */}
      {specs.length > 0 && (
        <Card title="接口详情" className="mb-4">
          {renderEndpointsTable(specs[0]?.specJson || '{}')}
        </Card>
      )}

      {specs.length > 0 && (
        <Card title="数据模型">
          {renderSchemas(specs[0]?.specJson || '{}')}
        </Card>
      )}
    </div>
  )

  return (
    <div className="flex h-screen bg-gray-50">
      <Sidebar />
      <main className="h-screen flex-1 flex flex-col relative overflow-hidden transition-all duration-300" style={{ marginLeft: mainMarginLeft }}>
        <header className="bg-white border-b px-6 py-4">
          <Title level={4} className="m-0">资产管理</Title>
        </header>
        {selectedProject ? renderProjectDetail() : renderProjectList()}
      </main>

      {/* 创建项目弹窗 */}
      <Modal
        title="新建项目"
        open={createModalVisible}
        onOk={handleCreateProject}
        onCancel={() => { setCreateModalVisible(false); setNewProjectTitle('') }}
        okText="创建"
        cancelText="取消"
      >
        <div style={{ marginTop: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>项目名称</label>
          <input
            type="text"
            value={newProjectTitle}
            onChange={(e) => setNewProjectTitle(e.target.value)}
            placeholder="请输入项目名称"
            style={{ width: '100%', padding: '8px 12px', border: '1px solid #d9d9d9', borderRadius: 4 }}
          />
        </div>
      </Modal>

      {/* GitHub 仓库选择弹窗 */}
      <Modal
        title="选择 GitHub 仓库"
        open={githubReposModalVisible}
        onCancel={() => setGithubReposModalVisible(false)}
        footer={null}
        width={600}
      >
        {githubRepos.length === 0 ? (
          <Empty description={githubReposLoading ? '加载中...' : '暂无仓库或尚未绑定 GitHub 账号'} />
        ) : (
          <div>
            <List
              dataSource={githubRepos.slice((githubReposPage - 1) * githubReposPageSize, githubReposPage * githubReposPageSize)}
              renderItem={(repo: GitHubRepo) => (
                <List.Item>
                  <Checkbox
                    checked={editingGitHubRepos.includes(repo.full_name)}
                    onChange={() => toggleGitHubRepo(repo.full_name)}
                    className="w-full"
                  >
                    <div className="flex items-center gap-2">
                      <GithubOutlined />
                      <span className="font-medium">{repo.full_name}</span>
                      {repo.private && <Tag color="orange">私有</Tag>}
                    </div>
                    {repo.description && (
                      <div className="text-xs text-gray-500 mt-1">{repo.description}</div>
                    )}
                  </Checkbox>
                </List.Item>
              )}
            />
            {githubRepos.length > githubReposPageSize && (
              <div className="flex justify-center mt-4">
                <Pagination
                  current={githubReposPage}
                  pageSize={githubReposPageSize}
                  total={githubRepos.length}
                  onChange={(page: number) => setGithubReposPage(page)}
                  size="small"
                />
              </div>
            )}
            <div className="flex justify-end gap-2 mt-4">
              <Button onClick={() => setGithubReposModalVisible(false)}>取消</Button>
              <Button type="primary" onClick={confirmGitHubRepos}>确定</Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}



