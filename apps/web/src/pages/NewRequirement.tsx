import { Card, Form, Input, Select, DatePicker, Button, Space, Tag, Timeline, Drawer, List, Spin } from 'antd'
import { useState, useEffect } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import Sidebar from '../components/Sidebar'

// TS 后端 API 地址（飞书相关接口）
const API_BASE_TS = '/api2'
// Go 后端 API 地址（OpenAPI 相关接口）
const API_BASE_GO = ''
const USER_TOKEN_KEY = 'feishu_user_token'
const USER_OPEN_ID_KEY = 'feishu_user_open_id'

// 部门信息接口
interface DepartmentInfo {
  id: string
  label: string
  leaderUserId?: string
}

export default function NewRequirement() {
  const navigate = useNavigate()
  const searchParams = useSearch({ from: '/new-requirement' })
  const [form] = Form.useForm()
  const [teams, setTeams] = useState<{ value: string; label: string }[]>([])
  const [selectedTeams, setSelectedTeams] = useState<string[]>([])
  const [loadingTeams, setLoadingTeams] = useState(false)
  const [departmentMap, setDepartmentMap] = useState<Map<string, DepartmentInfo>>(new Map())
  const [leaderNames, setLeaderNames] = useState<Map<string, string>>(new Map())
  const [loadingLeaders, setLoadingLeaders] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [generatingDoc, setGeneratingDoc] = useState(false)
  
  // 加载项目列表
  const loadProjects = async () => {
    setLoadingProjects(true)
    try {
      const res = await fetch(`${API_BASE_GO}/api/projects`)
      if (res.ok) {
        const data = await res.json()
        if (data.data) {
          setProjects(data.data.map((p: any) => ({ id: p.ID || p.id, title: p.Title || p.title || '未命名项目' })))
        }
      }
    } catch (err) {
      console.error('加载项目列表失败:', err)
    } finally {
      setLoadingProjects(false)
    }
  }

  // 项目相关状态
  const [projects, setProjects] = useState<{ id: string; title: string }[]>([])
  const [loadingProjects, setLoadingProjects] = useState(false)
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)

  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search)
    const projectId = urlParams.get('projectId')
    if (projectId) {
      setSelectedProjectId(projectId)
    }
  }, [])

  useEffect(() => {
    loadProjects()
  }, [])
  
  // AI 步骤项类型
  interface AiChainItem {
    key: string
    title: string
    description?: string
    status: 'loading' | 'success' | 'error'
    children?: React.ReactNode
  }
  
  // AI 生成状态
  const [aiChainItems, setAiChainItems] = useState<AiChainItem[]>([])
  const [isAiGenerating, setIsAiGenerating] = useState(false)
  const [selectedDocUrls, setSelectedDocUrls] = useState<string[]>([])
  const [wikiSpaces, setWikiSpaces] = useState<any[]>([])
  const [drawerVisible, setDrawerVisible] = useState(false)
  const [loadingSpaces, setLoadingSpaces] = useState(false)
  const [currentFolder, setCurrentFolder] = useState<string>('')
  const [folderHistory, setFolderHistory] = useState<{token: string, name: string}[]>([])
  
  const sidebarWidth = 80
  
  // 提交需求
  const handleSubmit = async () => {
    const leaders = getLeaderTags()
    if (leaders.length === 0) {
      alert('请先选择至少一个团队')
      return
    }

    try {
      await form.validateFields()
    } catch {
      return 
    }

    console.log('提交的 Leaders:', leaders)

    // 获取表单数据
    const formData = form.getFieldsValue()
    const priority = formData['priority'] || ''
    const businessType = formData['business_type'] || ''
    const targetDate = formData['target_date']
    const deadlineStr = targetDate ? `，截止时间：${targetDate.format('YYYY-MM-DD')}` : ''
    const autoTitle = priority && businessType ? `[${priority}]${businessType}需求` : ''
    const requirementTitle = autoTitle || formData['requirement_title'] || '未命名需求'
    console.log('需求标题:', requirementTitle)

    setSubmitting(true)
    try {
      let docUrl = ''
      let swaggerUrl = ''
      let specId = ''
      let testResult: {summary?: {total: number, passed: number, failed: number}, results?: any[]} | null = null
      const description = formData['description'] || ''
      const isInterfaceIntegration = businessType === '接口对接'
      
      if (isInterfaceIntegration && (description || selectedDocUrls.length > 0)) {
        setGeneratingDoc(true)
        setIsAiGenerating(true)
        setAiChainItems([])
        
        // 初始思考节点
        setAiChainItems([{
          key: 'start',
          title: '🚀 开始分析需求',
          description: '正在理解用户输入...',
          status: 'loading',
        } as any])
        
        try {
          const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
          const openId = localStorage.getItem(USER_OPEN_ID_KEY) || ''
          
          let aiMessage = `请根据以下需求描述生成 API 设计文档：\n\n需求标题：${requirementTitle}`
          if (deadlineStr) {
            aiMessage += `\n\n截止时间：${targetDate.format('YYYY-MM-DD')}`
          }
          
          if (description) {
            aiMessage += `\n\n详细描述：${description}`
          }
          
          if (selectedDocUrls.length > 0) {
            aiMessage += `\n\n## 重要：用户提供了飞书文档参考\n请先调用 extractContentFromUrls 工具读取以下文档内容，然后基于文档内容生成 API 设计：\n${selectedDocUrls.join('\n')}`
          }
          
          // 调用 SSE AI 生成文档
          const aiRes = await fetch(`${API_BASE_TS}/api/ai/chat/stream`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
              message: aiMessage,
              user_token: userToken,
              open_id: openId
            }),
          })
          
          if (!aiRes.ok || !aiRes.body) {
            throw new Error('SSE 请求失败')
          }
          
          const reader = aiRes.body.getReader()
          const decoder = new TextDecoder()
          let buffer = ''
          
          while (true) {
            const { done, value } = await reader.read()
            if (done) break
            
            buffer += decoder.decode(value, { stream: true })
            const lines = buffer.split('\n')
            buffer = lines.pop() || ''
            
            for (const line of lines) {
              if (!line.startsWith('data: ')) continue
              try {
                const data = JSON.parse(line.slice(6))
                
                switch (data.event) {
                  case 'start':
                    // 只添加一个分析节点
                    setAiChainItems([{
                      key: 'step_0',
                      title: '🤖 AI 正在生成文档...',
                      description: data.content || '正在分析需求并生成文档',
                      status: 'loading' as const,
                    }])
                    break
                    
                  case 'done': {
                    // 完成所有
                    if (data.content && !docUrl) {
                      const docMatch = data.content.match(/https:\/\/feishu\.cn\/docx\/[a-zA-Z0-9_-]+/)
                      if (docMatch) {
                        docUrl = docMatch[0]
                      }
                    }
                    // 构建展示节点
                    const chainItems: any[] = []
                    if (docUrl || swaggerUrl) {
                      chainItems.push({
                        key: 'step_doc',
                        title: '✅ 文档生成完成',
                        description: docUrl ? '文档链接: ' + docUrl : 'Swagger: ' + swaggerUrl,
                        status: 'success',
                      })
                    }
                    if (testResult?.summary) {
                      const s = testResult.summary
                      const icon = s.failed > 0 ? '⚠️' : '✅'
                      chainItems.push({
                        key: 'step_test',
                        title: icon + ' 接口测试: ' + s.total + ' 个用例',
                        description: '通过 ' + s.passed + ' 个，失败 ' + s.failed + ' 个',
                        status: s.failed > 0 ? 'warning' : 'success',
                      })
                    }
                    if (chainItems.length === 0) {
                      chainItems.push({
                        key: 'step_done',
                        title: '📝 处理完成',
                        description: 'AI 处理已完成',
                        status: 'success',
                      })
                    }
                    setAiChainItems(chainItems)
                    break
                  }
                    
                  case 'error':
                    console.error('AI Stream Error:', data.message)
                    setAiChainItems([{
                      key: 'step_0',
                      title: '❌ 发生错误',
                      description: data.message,
                      status: 'error' as const,
                    }])
                    break
                    
                  case 'tool_result':
                    console.log('[DEBUG] tool_result event:', JSON.stringify(data))
                    // 提取 swaggerUrl（Swagger UI 页面链接）
                    if (data.name === 'saveOpenApiSpec' && data.result?.swaggerUrl) {
                      swaggerUrl = data.result.swaggerUrl
                      // 从 swaggerUrl 中解析 specId，格式: /swagger?specId=spec_xxx
                      const match = swaggerUrl.match(/specId=([^&]+)/)
                      if (match) {
                        specId = match[1]
                      }
                      console.log('[DEBUG] swaggerUrl extracted:', swaggerUrl, 'specId:', specId)
                    }
                    // 提取飞书文档链接
                    if (data.name === 'createFeishuDocument' && data.result?.url) {
                      docUrl = data.result.url
                    }
                    // 提取接口测试结果
                    if (data.name === 'testApi' && data.result) {
                      testResult = data.result
                      console.log('[DEBUG] testApi result:', JSON.stringify(testResult))
                    }
                    break
                }
              } catch (e) {
                // 忽略 JSON 解析错误
              }
            }
          }
        } catch (aiErr) {
          console.error('AI 生成文档失败:', aiErr)
          // 添加错误节点
          const errorKey = `error_${Date.now()}`
          setAiChainItems(prev => [...(prev || []), {
            key: errorKey,
            title: '❌ 请求失败',
            description: String(aiErr),
            status: 'error' as any,
          }])
        }
        setGeneratingDoc(false)
        setIsAiGenerating(false)
      }

      // 2. 发送消息给每个 leader
      for (const leader of leaders) {
        // 构建消息内容
        let messageContent = `${leader.name}，您好！您收到了一条新的开发需求，请及时查看处理。\n\n`
        messageContent += `📋 需求模块：${requirementTitle}\n`
        if (targetDate) {
          messageContent += `📅 截止时间：${targetDate.format('YYYY-MM-DD')}\n`
        }
        if (priority) {
          const priorityDesc: Record<string, string> = {
            'P0': '紧急阻断',
            'P1': '高优',
            'P2': '常规需求',
            'P3': '低优'
          }
          messageContent += `🔥 优先级：${priority} ${priorityDesc[priority] || ''}\n`
        }
        
        if (businessType) {
          messageContent += `📂 业务类型：${businessType}\n`
        }
        
        // 如果生成了文档，添加文档链接
        if (docUrl) {
          messageContent += `\n📄 API 设计文档：${docUrl}\n`
        }
        
        // 如果生成了 Swagger UI，添加链接
        if (swaggerUrl) {
          messageContent += '\n🔧 Swagger UI 接口调试：' + swaggerUrl + '\n'
        }
                
        // 如果有测试结果，添加测试摘要
        if (testResult?.summary) {
          const s = testResult.summary
          const icon = s.failed > 0 ? '⚠️' : '✅'
          messageContent += '\n' + icon + ' 接口测试结果：共 ' + s.total + ' 个用例，通过 ' + s.passed + ' 个，失败 ' + s.failed + ' 个\n'
        }
                
        console.log('[DEBUG] 发送飞书消息前检查 - docUrl:', docUrl, 'swaggerUrl:', swaggerUrl, 'testResult:', !!testResult)
        console.log('[DEBUG] 消息内容:', messageContent)
        
        messageContent += `\n请及时查看并处理！`
        
        const content = JSON.stringify({ text: messageContent })
        const uuid = crypto.randomUUID()

        await fetch(`${API_BASE_TS}/api/feishu/send-message`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            receive_id: leader.id,
            receive_id_type: 'open_id',
            msg_type: 'text',
            content,
            uuid,
          }),
        })
      }
      
      // 3. 更新资产的关联信息（如果生成了 swaggerUrl 且有选中的飞书文档）
      if (specId) {
        try {
          const updateData: any = {
            title: requirementTitle,
            description: description,
          }
          // 关联项目
          if (selectedProjectId) {
            updateData.projectId = selectedProjectId
          }
          // 关联飞书文档
          if (selectedDocUrls.length > 0) {
            updateData.doc_urls = selectedDocUrls
          }
          await fetch(`${API_BASE_GO}/api/openapi/${specId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updateData),
          })
          console.log('[DEBUG] 资产关联信息已更新')
        } catch (err) {
          console.error('[DEBUG] 更新资产关联信息失败:', err)
        }
      }
      
      // 4. 如果选择了项目，创建项目-需求关联
      if (selectedProjectId) {
        try {
          await fetch(`${API_BASE_GO}/api/projects/${selectedProjectId}/requirements`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              title: requirementTitle,
              description: description,
            }),
          })
          console.log('[DEBUG] 项目-需求关联已创建')
        } catch (err) {
          console.error('[DEBUG] 创建项目-需求关联失败:', err)
        }
      }
      
      let alertMsg = `需求已提交！\n已通知 ${leaders.length} 位团队负责人`
      if (selectedProjectId) {
        alertMsg += `\n\n📁 已关联到项目`
      }
      if (docUrl) {
        alertMsg += `\n\n📄 AI 已自动生成 API 设计文档：${docUrl}`
      }
      if (swaggerUrl) {
        alertMsg += `\n\n🔧 Swagger UI：${swaggerUrl}`
      }
      alert(alertMsg)
      
      // 跳转到首页
      navigate({ to: '/' })
    } catch (err) {
      console.error('提交失败:', err)
      alert('提交失败，请重试')
    } finally {
      setSubmitting(false)
    }
  }
  
  const openDocPicker = async () => {
    const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
    if (!userToken) {
      alert('请先登录飞书账号')
      return
    }
    
    setDrawerVisible(true)
    setCurrentFolder('')
    setFolderHistory([])
    await fetchFiles(userToken, '')
  }
  
  const fetchFiles = async (_token: string, folderToken: string) => {
    setLoadingSpaces(true)
    try {
      const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
      const url = folderToken
        ? `${API_BASE_TS}/api/feishu/wiki-spaces?user_token=${encodeURIComponent(userToken)}&folder_token=${encodeURIComponent(folderToken)}`
        : `${API_BASE_TS}/api/feishu/wiki-spaces?user_token=${encodeURIComponent(userToken)}`
      const res = await fetch(url)
      const data = await res.json()
      if (data.success && data.data?.data?.files) {
        setWikiSpaces(data.data.data.files)
      } else if (data.code === 401 || data.error?.includes('expired')) {
        alert('登录已过期，请重新登录')
        setDrawerVisible(false)
      } else {
        setWikiSpaces([])
      }
    } catch (err) {
      console.error('获取文件失败:', err)
      setWikiSpaces([])
    } finally {
      setLoadingSpaces(false)
    }
  }
  
  // 进入文件夹
  const enterFolder = (folder: any) => {
    setFolderHistory(prev => [...prev, { token: currentFolder, name: currentFolder ? '当前目录' : '根目录' }])
    setCurrentFolder(folder.token)
    const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
    fetchFiles(userToken, folder.token)
  }
  
  const goBack = () => {
    const history = [...folderHistory]
    const prev = history.pop()
    setFolderHistory(history)
    setCurrentFolder(prev?.token || '')
    const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
    fetchFiles(userToken, prev?.token || '')
  }
  
  const toggleSelectDocument = (file: any) => {
    if (file.url) {
      const docUrl = file.url.replace('lanshanteam.feishu.cn', 'feishu.cn')
      setSelectedDocUrls(prev => {
        if (prev.includes(docUrl)) {
          return prev.filter(url => url !== docUrl)
        } else {
          return [...prev, docUrl]
        }
      })
    }
  }

  // 获取部门列表（递归获取所有子部门）
  useEffect(() => {
    const fetchAllDepartments = async () => {
      const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
      
      if (!userToken) {
        return
      }

      setLoadingTeams(true)
      try {
        // 第一步：递归获取所有部门（处理分页 has_more）
        const allItems: any[] = []
        let pageToken: string | undefined = undefined
        
        do {
          const params = new URLSearchParams({
            department_id: '0',
            fetch_child: 'true',
            page_size: '50',
            user_token: userToken,
          })
          if (pageToken) {
            params.set('page_token', pageToken)
          }
          
          const res = await fetch(`/api/feishu/department-children?${params.toString()}`)
          const data = await res.json()
          
          if (data.success && data.data?.data?.items) {
            allItems.push(...data.data.data.items)
            pageToken = data.data.data.page_token
            console.log(`获取到 ${data.data.data.items.length} 个部门, has_more: ${data.data.data.has_more}`)
          } else {
            break
          }
        } while (pageToken)
        
        console.log('获取到的所有部门数量:', allItems.length)
        
        if (allItems.length === 0) {
          setLoadingTeams(false)
          return
        }
        
        // 第二步：获取所有部门的完整信息（包括 leader）
        // 提取所有部门的 open_department_id
        const allDeptIds = allItems
          .map((dept: any) => dept.open_department_id || dept.department_id)
          .filter((id: string) => id)
        
        console.log('部门 IDs:', allDeptIds)
        
        // 分批调用批量获取部门信息接口（最多 50 个一批）
        const batchSize = 50
        const allDepartments: any[] = []
        
        for (let i = 0; i < allDeptIds.length; i += batchSize) {
          const batchIds = allDeptIds.slice(i, i + batchSize)
          const batchRes = await fetch(`${API_BASE_TS}/api/feishu/batch-departments`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
              department_ids: batchIds, 
              user_token: userToken,
              user_id_type: 'open_id'  // 明确要求返回 open_id
            }),
          })
          const batchData = await batchRes.json()
          
          if (batchData.success && batchData.data?.data?.items) {
            allDepartments.push(...batchData.data.data.items)
          }
        }
        
        console.log('批量获取的部门详情数量:', allDepartments.length)
        
        // 处理部门数据，提取名称和 leader
        const departments: DepartmentInfo[] = allDepartments.map((dept: any) => {
          const id = dept.open_department_id || dept.department_id || ''
          const zhName = dept.i18n_name?.zh_cn?.trim()
          const enName = dept.i18n_name?.en_us?.trim()
          const rawName = dept.name?.trim()
          const label = zhName || enName || rawName || `部门-${id.slice(0, 8)}`
          // 获取主负责人（leaderType === 1）
          const leaderUserId = dept.leaders?.find((l: any) => l.leaderType === 1)?.leaderID || dept.leader_user_id
          return { id, label, leaderUserId }
        })
        
        console.log('处理的部门数据:', departments)
        
        const deptMap = new Map<string, DepartmentInfo>()
        departments.forEach(dept => deptMap.set(dept.id, dept))
        setDepartmentMap(deptMap)
        
        const teamOptions = departments.map(d => ({ value: d.id, label: d.label }))
        if (teamOptions.length > 0) {
          setTeams(teamOptions)
        }
      } catch (err) {
        console.error('Failed to fetch departments:', err)
      } finally {
        setLoadingTeams(false)
      }
    }
    fetchAllDepartments()
  }, [])

  // 当选择的团队变化时，获取 leader 姓名
  useEffect(() => {
    const fetchLeaderNames = async () => {
      if (selectedTeams.length === 0) {
        setLeaderNames(new Map())
        return
      }

      setLoadingLeaders(true)
      try {
        // 从 departmentMap 中获取所有 leader user_id
        const leaderIds = selectedTeams
          .map(teamId => departmentMap.get(teamId)?.leaderUserId)
          .filter((id): id is string => !!id)
          .filter((id, index, arr) => arr.indexOf(id) === index) // 去重

        if (leaderIds.length === 0) {
          setLeaderNames(new Map())
          return
        }

        const userToken = localStorage.getItem(USER_TOKEN_KEY) || ''
        const res = await fetch(`${API_BASE_TS}/api/feishu/batch-user-names`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            user_ids: leaderIds,
            user_token: userToken,
          }),
        })
        const data = await res.json()

        if (data.success && data.data?.data?.users) {
          const nameMap = new Map<string, string>()
          data.data.data.users.forEach((user: any) => {
            // 优先使用外层 name 字段，其次使用 i18n_name.zh_cn
            const name = user.name || user.i18n_name?.zh_cn || user.user_id
            nameMap.set(user.user_id, name)
          })
          setLeaderNames(nameMap)
        }
      } catch (err) {
        console.error('Failed to fetch leader names:', err)
      } finally {
        setLoadingLeaders(false)
      }
    }

    fetchLeaderNames()
  }, [selectedTeams, departmentMap])

  // 获取当前选中团队的 leader 列表
  const getLeaderTags = () => {
    const leaders = selectedTeams
      .map(teamId => departmentMap.get(teamId)?.leaderUserId)
      .filter((id): id is string => !!id)
      .filter((id, index, arr) => arr.indexOf(id) === index) // 去重

    return leaders.map(leaderId => ({
      id: leaderId,
      name: leaderNames.get(leaderId) || leaderId,
    }))
  }

  return (
    <div className="min-h-screen bg-background">
      <Sidebar />
      <main className="h-screen overflow-y-auto p-6 transition-all duration-300" style={{ marginLeft: `${sidebarWidth}px` }}>
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm mb-3">
            <span className="text-on-surface-variant">创建</span>
            <span className="text-on-surface/30">›</span>
            <span className="text-primary font-medium">新建需求</span>
          </div>
          <div className="flex justify-between items-start gap-5">
            <div>
              <h1 className="text-2xl font-bold text-on-surface mb-1">创建新需求</h1>
              <p className="text-sm text-on-surface-variant max-w-xl leading-relaxed">
                精确描述功能需求和技术需求，可从飞书文档导入现有草稿。
              </p>
            </div>
            <Button icon={<span className="material-symbols-outlined text-sm">upload_file</span>} className="flex items-center gap-2 px-3 py-2 !rounded-lg !border-outline-variant bg-white text-primary text-sm font-medium hover:bg-surface-container-low whitespace-nowrap">
              从飞书文档导入
            </Button>
          </div>
        </div>

        <div className="mb-8">
          <Card className="!rounded-xl !shadow-sm !p-6" bordered={false}>
          <Form form={form} layout="vertical">
            {/* 需求元数据 */}
            <div className="grid grid-cols-3 gap-2 mb-4">
              <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">优先级</span>} name="priority" className="mb-0" rules={[{ required: true, message: '请选择优先级' }]}>
                <Select placeholder="优先级" className="!rounded-lg" size="small">
                  <Select.Option value="P0">P0</Select.Option>
                  <Select.Option value="P1">P1</Select.Option>
                  <Select.Option value="P2">P2</Select.Option>
                  <Select.Option value="P3">P3</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">业务需求</span>} name="business_type" className="mb-0" rules={[{ required: true, message: '请选择业务需求' }]}>
                <Select placeholder="业务类型" className="!rounded-lg" size="small">
                  <Select.Option value="新功能开发">新功能</Select.Option>
                  <Select.Option value="迭代优化">迭代优化</Select.Option>
                  <Select.Option value="BUG修复">BUG修复</Select.Option>
                  <Select.Option value="性能优化">性能优化</Select.Option>
                  <Select.Option value="兼容性适配">兼容性</Select.Option>
                  <Select.Option value="数据相关">数据相关</Select.Option>
                  <Select.Option value="接口对接">接口对接</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">截止时间</span>} name="target_date" className="mb-0" rules={[{ required: true, message: '请选择截止时间' }]}>
                <DatePicker className="w-full !rounded-lg" placeholder="截止日期" size="small" />
              </Form.Item>
              <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">关联项目</span>} className="mb-0">
                <Select
                  placeholder="选择项目（可选）"
                  allowClear
                  value={selectedProjectId}
                  onChange={setSelectedProjectId}
                  options={projects.map(p => ({ value: p.id, label: p.title }))}
                  loading={loadingProjects}
                  className="!rounded-lg"
                  size="small"
                />
              </Form.Item>
            </div>

            <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">需求标题</span>} name="requirement_title">
              <Input placeholder="选填，例如：实时数据分析模块" className="!rounded-lg" />
            </Form.Item>
            <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">详细描述</span>} name="description">
              <Input.TextArea
                placeholder="描述核心目标、功能边界和关键约束"
                rows={8}
                className="!rounded-lg"
              />
            </Form.Item>

            {/* 选取飞书文档按钮 */}
            <div className="mb-4">
              <Button onClick={openDocPicker} className="!rounded-lg">
                📄 选取飞书文档
              </Button>
              {selectedDocUrls.length > 0 && (
                <div className="mt-2">
                  <div className="text-sm text-green-600">已选择 {selectedDocUrls.length} 个文档</div>
                  <div className="flex flex-wrap gap-1 mt-1">
                    {selectedDocUrls.map((url, idx) => (
                      <Tag key={idx} closable onClose={() => setSelectedDocUrls(prev => prev.filter(u => u !== url))}>
                        {url.split('/').pop()?.substring(0, 20)}...
                      </Tag>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">团队</span>} className="mb-0">
                <Select
                  mode="multiple"
                  placeholder="选择团队（可多选）"
                  value={selectedTeams}
                  onChange={setSelectedTeams}
                  options={teams}
                  loading={loadingTeams}
                  className="w-full"
                  maxTagCount={2}
                  allowClear
                />
              </Form.Item>

            <Form.Item label={<span className="text-xs font-semibold text-on-surface-variant tracking-wider">团队Leader</span>} className="mb-4">
              <div className="min-h-[32px] p-2 bg-white rounded border border-gray-200">
                {loadingLeaders ? (
                  <span className="text-gray-400 text-sm">加载中...</span>
                ) : getLeaderTags().length === 0 ? (
                  <span className="text-gray-400 text-sm">选择团队后自动显示</span>
                ) : (
                  <Space wrap>
                    {getLeaderTags().map(leader => (
                      <Tag key={leader.id} color="blue" className="!m-0">
                        {leader.name}
                      </Tag>
                    ))}
                  </Space>
                )}
              </div>
              <div className="text-xs text-gray-400 mt-1">选择团队后自动通知负责人</div>
            </Form.Item>
          </Form>
        </Card>
        </div>

        {/* AI 生成状态展示 */}
        {(generatingDoc || (aiChainItems && aiChainItems.length > 0)) && (
          <Card className="!rounded-xl !shadow-sm !p-4 mb-4" bordered={false}>
            <div className="flex items-center gap-2 mb-4 pb-3 border-b border-gray-100">
              <span className="material-symbols-outlined text-primary" style={{ fontSize: 20 }}>
                {isAiGenerating ? 'psychology' : 'check_circle'}
              </span>
              <span className="font-medium">
                {isAiGenerating ? 'AI 正在处理...' : 'AI 处理完成'}
              </span>
            </div>
            <Timeline
              items={aiChainItems.map(item => ({
                color: item.status === 'loading' ? 'blue' : item.status === 'success' ? 'green' : 'red',
                content: (
                  <div>
                    <div className="font-medium text-sm">{item.title}</div>
                    {item.description && (
                      <div className="text-xs text-gray-500 mt-1">{item.description}</div>
                    )}
                    {item.children}
                  </div>
                ),
              }))}
            />
          </Card>
        )}

        <Button 
          type="primary" 
          icon={<span className="material-symbols-outlined text-sm">send</span>} 
          size="large" 
          className="!rounded-xl !font-semibold"
          onClick={handleSubmit}
          loading={submitting || generatingDoc}
        >
          {generatingDoc ? 'AI 生成文档...' : submitting ? '通知中...' : '提交需求'}
        </Button>

        {/* 飞书文档选择抽屉 */}
        <Drawer
          title="选择飞书文档"
          placement="right"
          width={500}
          onClose={() => setDrawerVisible(false)}
          open={drawerVisible}
          footer={
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500">已选择 {selectedDocUrls.length} 个文件</span>
              <Space>
                <Button onClick={() => setDrawerVisible(false)}>取消</Button>
                <Button type="primary" onClick={() => setDrawerVisible(false)}>确认 ({selectedDocUrls.length})</Button>
              </Space>
            </div>
          }
        >
          {currentFolder !== '' && (
            <Button 
              icon={<span className="material-symbols-outlined text-sm">arrow_back</span>}
              onClick={goBack}
              className="mb-3 !rounded-lg"
            >
              返回上级
            </Button>
          )}
          {loadingSpaces ? (
            <div className="flex justify-center items-center py-10">
              <Spin />
            </div>
          ) : wikiSpaces.length === 0 ? (
            <div className="text-center text-gray-500 py-10">暂无可用文档</div>
          ) : (
            <List
              dataSource={wikiSpaces}
              renderItem={(file: any) => {
                const docUrl = file.url?.replace('lanshanteam.feishu.cn', 'feishu.cn')
                const isSelected = docUrl && selectedDocUrls.includes(docUrl)
                return (
                <List.Item 
                  onClick={() => file.type === 'folder' ? enterFolder(file) : toggleSelectDocument(file)}
                  className={`cursor-pointer hover:bg-gray-50 px-2 py-3 rounded-lg ${isSelected ? 'bg-blue-50' : ''}`}
                  style={isSelected ? { borderLeft: '3px solid #1890ff' } : {}}
                >
                  <List.Item.Meta
                    avatar={
                      <span className="material-symbols-outlined" style={{ color: isSelected ? '#1890ff' : '#9ca3af' }}>
                        {file.type === 'folder' ? 'folder' : 'description'}
                      </span>
                    }
                    title={
                      <span style={{ color: isSelected ? '#1890ff' : 'inherit' }}>
                        {file.name || '未命名文件'}
                        {isSelected && <span className="ml-2 text-xs">✓</span>}
                      </span>
                    }
                    description={
                      <span className="text-xs text-gray-400">
                        {file.type === 'folder' ? '文件夹' : file.mime_type || '文件'}
                      </span>
                    }
                  />
                </List.Item>
                )
              }}
            />
          )}
        </Drawer>
      </main>
    </div>
  )
}
