import { useState, useEffect } from 'react'

// TS 后端 API 地址
const API_BASE = 'http://localhost:3001'
// Go 后端 API 地址
const GO_API_BASE = 'http://localhost:8080'
const USER_TOKEN_KEY = 'feishu_user_token'
const USER_TOKEN_EXPIRES_KEY = 'feishu_user_token_expires'
const USER_OPEN_ID_KEY = 'feishu_user_open_id'

interface ApiResult {
  success?: boolean
  data?: unknown
  error?: string
  code?: number
  msg?: string
  message?: string
}

export default function Debug() {
  const [results, setResults] = useState<{ api: string; result: ApiResult; time: string }[]>([])
  const [loading, setLoading] = useState(false)

  // Token 状态
  const [userToken, setUserToken] = useState(() => localStorage.getItem(USER_TOKEN_KEY) || '')
  const [tokenExpires, setTokenExpires] = useState(() => localStorage.getItem(USER_TOKEN_EXPIRES_KEY) || '')
  const [openId, setOpenId] = useState(() => localStorage.getItem(USER_OPEN_ID_KEY) || '')
  const [folderToken, setFolderToken] = useState('')
  const [documentId, setDocumentId] = useState('')
  const [manualCode, setManualCode] = useState('')

  // 发送消息状态
  const [receiveId, setReceiveId] = useState('')
  const [receiveIdType, setReceiveIdType] = useState<'open_id' | 'user_id' | 'union_id' | 'email' | 'chat_id'>('open_id')
  const [msgType, setMsgType] = useState<'text' | 'post' | 'interactive'>('text')
  const [messageContent, setMessageContent] = useState('')

  // 卡片测试状态
  const [cardOpenId, setCardOpenId] = useState(() => localStorage.getItem(USER_OPEN_ID_KEY) || '')
  const [cardTitle, setCardTitle] = useState('测试需求卡片')
  const [cardSummary, setCardSummary] = useState('这是一个测试需求摘要，用于验证卡片功能是否正常。')
  const [cardRequirement, setCardRequirement] = useState('详细需求描述：\n1. 功能点A\n2. 功能点B\n3. 功能点C')
  const [cardSessionId, setCardSessionId] = useState('test_session_' + Date.now())

  // AI 生成状态
  const [aiDocUrl, setAiDocUrl] = useState('')

  // AI 生成代码
  const handleAIGenerate = async () => {
    if (!aiDocUrl) {
      alert('请输入飞书文档 URL')
      return
    }

    setLoading(true)
    const start = Date.now()

    try {
      // 从 URL 中提取 wiki token
      const cleanUrl = aiDocUrl.trim().replace(/\n/g, '')
      const wikiMatch = cleanUrl.match(/\/wiki\/([a-zA-Z0-9]+)/)
      
      if (!wikiMatch) {
        addResult('AI 生成', { error: `无法识别的 URL 格式。当前输入：${cleanUrl}` }, `${Date.now() - start}ms`)
        setLoading(false)
        return
      }
      
      const documentId = wikiMatch[1]

      // 1. 先获取 Wiki 节点信息
      addResult('1. 获取 Wiki 节点', { message: '正在获取节点信息...' }, '0ms')

      let docData: any

      // wiki 知识库：先获取节点信息
      let docRes = await fetch(`${API_BASE}/api/feishu/wiki-node?token=${documentId}`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
      })
      docData = await docRes.json()

      if (!docData.success || !docData.data) {
        addResult('1. 获取 Wiki 节点', { error: docData.error || '获取 wiki 节点失败' }, `${Date.now() - start}ms`)
        setLoading(false)
        return
      }

      // 从 wiki 节点获取实际文档 ID
      const wikiDocId = docData.data?.obj_token
      if (!wikiDocId) {
        addResult('1. 获取 Wiki 节点', { error: `无法获取文档 ID，返回数据：${JSON.stringify(docData.data)}` }, `${Date.now() - start}ms`)
        setLoading(false)
        return
      }

      // 2. 获取文档内容
      addResult('2. 获取文档内容', { message: '正在获取文档内容...' }, '')

      docRes = await fetch(`${API_BASE}/api/ai/get-document-content`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ document_id: wikiDocId }),
      })
      docData = await docRes.json()

      if (docData.code !== 200) {
        addResult('2. 获取文档内容', { error: docData.msg || '获取失败' }, `${Date.now() - start}ms`)
        setLoading(false)
        return
      }

      const docContent = docData.data?.content || ''
      addResult('2. 获取文档内容', { success: true, message: `获取成功，文档长度：${docContent.length} 字符` }, `${Date.now() - start}ms`)

      // 3. 调用 AI 生成代码
      addResult('3. AI 生成代码', { message: '正在调用 AI 生成...' }, '')

      // 调用 TS 后端的 AI 接口
      const aiRes = await fetch(`${API_BASE}/api/ai/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          message: '请根据文档内容生成代码实现',
          document_content: docContent,
          user_token: userToken,
          open_id: openId
        }),
      })
      const aiData = await aiRes.json()

      addResult('3. AI 生成代码', aiData, `${Date.now() - start}ms`)

    } catch (err) {
      addResult('AI 生成', { error: String(err) }, `${Date.now() - start}ms`)
    }

    setLoading(false)
  }

  // 检查 URL 中的 token 参数（OAuth 回调）
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')
    const expiresAt = params.get('expires_at')
    const openIdParam = params.get('open_id')
    const error = params.get('error')

    if (token) {
      setUserToken(token)
      localStorage.setItem(USER_TOKEN_KEY, token)
      if (expiresAt) {
        setTokenExpires(expiresAt)
        localStorage.setItem(USER_TOKEN_EXPIRES_KEY, expiresAt)
      }
      if (openIdParam) {
        setOpenId(openIdParam)
        localStorage.setItem(USER_OPEN_ID_KEY, openIdParam)
        setCardOpenId(openIdParam)
      }
      // 清除 URL 参数
      window.history.replaceState({}, '', '/debug')
      addResult('OAuth 回调', { success: true, data: { message: 'Token 已获取并保存', open_id: openIdParam } }, '自动')
    } else if (error) {
      addResult('OAuth 回调', { success: false, error: `错误: ${error}` }, '自动')
      window.history.replaceState({}, '', '/debug')
    }
  }, [])

  const addResult = (api: string, result: ApiResult, time: string) => {
    setResults(prev => [...prev, { api, result, time }])
  }

  // 获取飞书 OAuth URL
  const getOAuthUrl = async () => {
    setLoading(true)
    const start = Date.now()
    try {
      const res = await fetch(`${API_BASE}/api/feishu/oauth-url`)
      const data = await res.json()
      addResult('/api/feishu/oauth-url', data, `${Date.now() - start}ms`)

      if (data.success && data.data?.oauthUrl) {
        // 跳转到飞书授权页面
        window.location.href = data.data.oauthUrl
      }
    } catch (err) {
      addResult('/api/feishu/oauth-url', { error: String(err) }, `${Date.now() - start}ms`)
    }
    setLoading(false)
  }

  // 手动兑换 token
  const exchangeToken = async () => {
    if (!manualCode.trim()) {
      alert('请输入 code')
      return
    }
    setLoading(true)
    const start = Date.now()
    try {
      const res = await fetch(`${API_BASE}/api/feishu/exchange-token`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: manualCode }),
      })
      const data = await res.json()
      addResult('/api/feishu/exchange-token', data, `${Date.now() - start}ms`)

      if (data.success && data.data?.user_token) {
        setUserToken(data.data.user_token)
        setTokenExpires(data.data.expires_at || '')
        setOpenId(data.data.open_id || '')
        localStorage.setItem(USER_TOKEN_KEY, data.data.user_token)
        if (data.data.expires_at) {
          localStorage.setItem(USER_TOKEN_EXPIRES_KEY, data.data.expires_at)
        }
        if (data.data.open_id) {
          localStorage.setItem(USER_OPEN_ID_KEY, data.data.open_id)
          setCardOpenId(data.data.open_id)
        }
        setManualCode('')
      }
    } catch (err) {
      addResult('/api/feishu/exchange-token', { error: String(err) }, `${Date.now() - start}ms`)
    }
    setLoading(false)
  }

  // 调用 POST API
  const callApi = async (api: string, body: object) => {
    setLoading(true)
    const start = Date.now()
    try {
      const res = await fetch(`${API_BASE}${api}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await res.json()
      addResult(api, data, `${Date.now() - start}ms`)
    } catch (err) {
      addResult(api, { error: String(err) }, `${Date.now() - start}ms`)
    }
    setLoading(false)
  }

  // 调用 GET API
  const callGetApi = async (api: string, params: Record<string, string>) => {
    setLoading(true)
    const start = Date.now()
    try {
      const query = new URLSearchParams(params).toString()
      const res = await fetch(`${API_BASE}${api}?${query}`)
      const data = await res.json()
      addResult(api + '?' + query, data, `${Date.now() - start}ms`)
    } catch (err) {
      addResult(api, { error: String(err) }, `${Date.now() - start}ms`)
    }
    setLoading(false)
  }

  // 测试列表
  const [docTitle, setDocTitle] = useState('测试文档-' + new Date().toLocaleString())
  
  const tests = [
    {
      name: '健康检查',
      api: '/api/health2',
      body: {},
    },
    {
      name: '获取根目录文件',
      api: '/api/feishu/list-files',
      body: { order_by: 'EditedTime', direction: 'DESC' },
    },
    {
      name: '获取文件夹文件',
      api: '/api/feishu/list-files',
      body: { folder_token: folderToken, page_size: 20 },
    },
    {
      name: '创建文档',
      api: '/api/feishu/create-document',
      body: { folder_token: folderToken || undefined, title: docTitle },
    },
  ]

  // 模拟创建完整文档
  const [createdDocUrl, setCreatedDocUrl] = useState('')
  const createFullDocument = async () => {
    if (!userToken) {
      alert('请先获取 Token')
      return
    }
    setLoading(true)
    const start = Date.now()

    try {
      // 1. 创建文档
      const title = 'AI 生成报告 - ' + new Date().toLocaleString()
      addResult('1. 创建文档', { message: '开始创建文档...' }, '0ms')

      const createRes = await fetch(`${API_BASE}/api/feishu/create-document`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, user_token: userToken }),
      })
      const createData = await createRes.json()
      addResult('1. 创建文档', createData, `${Date.now() - start}ms`)

      if (!createData.success || !createData.data?.data?.document?.document_id) {
        alert('创建文档失败')
        setLoading(false)
        return
      }

      const documentId = createData.data.data.document.document_id
      const docUrl = `https://feishu.cn/docx/${documentId}`
      setCreatedDocUrl(docUrl)

      // 等待一小段时间让文档创建完成
      await new Promise(r => setTimeout(r, 500))

      // 简化的块结构测试 - 使用飞书正确的 block 结构
      const allBlocks = [
        {
          block_type: 3, // heading1
          heading1: {
            elements: [{ text_run: { content: '📊 测试报告摘要' } }],
          },
        },
        {
          block_type: 2, // text
          text: {
            elements: [{ text_run: { content: '这是一个由 AI 自动生成的测试文档。' } }],
          },
        },
        {
          block_type: 2, // text
          text: {
            elements: [{ text_run: { content: '文档创建时间：' + new Date().toLocaleString() } }],
          },
        },
      ]

      addResult('2. 添加块', { message: `正在添加 ${allBlocks.length} 个块...` }, '')

      const blocksRes = await fetch(`${API_BASE}/api/feishu/create-nested-blocks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          document_id: documentId,
          user_token: userToken,
          blocks: allBlocks,
        }),
      })
      const blocksData = await blocksRes.json()
      addResult('2. 添加块', blocksData, `${Date.now() - start}ms`)

      // 完成
      addResult('完成', {
        success: true,
        data: {
          message: '文档创建成功！',
          url: docUrl,
          document_id: documentId,
        },
      }, `${Date.now() - start}ms`)

    } catch (err) {
      addResult('错误', { error: String(err) }, `${Date.now() - start}ms`)
    }

    setLoading(false)
  }

  // 发送消息
  const sendMessage = async () => {
    if (!receiveId) {
      alert('请输入接收者 ID')
      return
    }
    if (!messageContent) {
      alert('请输入消息内容')
      return
    }
    if (!userToken) {
      alert('请先获取 Token')
      return
    }

    setLoading(true)
    const start = Date.now()

    try {
      let content: string
      if (msgType === 'text') {
        content = JSON.stringify({ text: messageContent })
      } else if (msgType === 'post') {
        // 富文本消息格式
        content = JSON.stringify({
          zh_cn: {
            title: '飞书通知',
            content: [[{
              tag: 'text',
              text: messageContent,
            }]]
          }
        })
      } else {
        content = JSON.stringify({ text: messageContent })
      }

      const uuid = crypto.randomUUID()

      await callApi('/api/feishu/send-message', {
        receive_id: receiveId,
        receive_id_type: receiveIdType,
        msg_type: msgType,
        content,
        uuid,
      })
    } catch (err) {
      addResult('/api/feishu/send-message', { error: String(err) }, `${Date.now() - start}ms`)
    }

    setLoading(false)
  }

  // 测试发送需求确认卡片
  const sendApprovalCard = async () => {
    if (!cardOpenId) {
      alert('请输入接收者的 open_id')
      return
    }
    if (!cardTitle) {
      alert('请输入卡片标题')
      return
    }

    setLoading(true)
    const start = Date.now()

    try {
      // 获取 Cookie
      const cookies = document.cookie.split(';').reduce((acc, cookie) => {
        const [key, value] = cookie.trim().split('=')
        acc[key] = value
        return acc
      }, {} as Record<string, string>)

      const res = await fetch(`${GO_API_BASE}/api/admin/test-approval-card`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Cookie': `session=${cookies['session'] || ''}`,
        },
        credentials: 'include',
        body: JSON.stringify({
          open_id: cardOpenId,
          title: cardTitle,
          summary: cardSummary,
          requirement: cardRequirement,
          session_id: cardSessionId,
          run_id: '',
        }),
      })

      const data = await res.json()
      addResult('/api/admin/test-approval-card', data, `${Date.now() - start}ms`)
    } catch (err) {
      addResult('/api/admin/test-approval-card', { error: String(err) }, `${Date.now() - start}ms`)
    }

    setLoading(false)
  }

  return (
    <div className="bg-gray-50 p-6" style={{ minHeight: '100vh', overflowY: 'auto' }}>
      <div className="max-w-6xl mx-auto">
        <h1 className="text-2xl font-bold text-gray-800 mb-6">🔧 TS 后端接口调试</h1>

        {/* Token 获取 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">🔑 Token 获取</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-gray-600 mb-1">方法1: OAuth 授权登录</label>
              <button
                onClick={getOAuthUrl}
                disabled={loading}
                className="w-full px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
              >
                一键登录飞书获取 Token
              </button>
              <p className="text-xs text-gray-500 mt-1">点击后会跳转到飞书授权，授权后自动返回</p>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">方法2: 手动兑换 Code</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={manualCode}
                  onChange={e => setManualCode(e.target.value)}
                  placeholder="输入飞书授权码"
                  className="flex-1 px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <button
                  onClick={exchangeToken}
                  disabled={loading}
                  className="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50"
                >
                  兑换
                </button>
              </div>
              <p className="text-xs text-gray-500 mt-1">从飞书 API 调试台获取的临时 code</p>
            </div>
          </div>
        </div>

        {/* 模拟创建完整文档 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">🤖 模拟创建完整文档</h2>
          <p className="text-sm text-gray-600 mb-3">自动创建文档并添加标题、段落、列表、引用、代码块等</p>
          <div className="flex items-center gap-4">
            <button
              onClick={createFullDocument}
              disabled={loading || !userToken}
              className="px-6 py-3 bg-purple-500 text-white rounded-lg hover:bg-purple-600 disabled:opacity-50 font-medium"
            >
              {loading ? '创建中...' : '🚀 一键创建飞书文档'}
            </button>
            {createdDocUrl && (
              <a
                href={createdDocUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="px-4 py-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 font-medium"
              >
                📄 查看文档
              </a>
            )}
          </div>
          {createdDocUrl && (
            <div className="mt-3 p-3 bg-gray-50 rounded">
              <label className="block text-sm text-gray-600 mb-1">文档链接：</label>
              <a
                href={createdDocUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-blue-600 hover:text-blue-800 break-all"
              >
                {createdDocUrl}
              </a>
            </div>
          )}
        </div>

        {/* 发送消息测试 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">📨 发送消息测试</h2>
          <p className="text-sm text-gray-600 mb-3">通过机器人向用户或群组发送消息</p>
          
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div>
              <label className="block text-sm text-gray-600 mb-1">接收者 ID 类型</label>
              <select
                value={receiveIdType}
                onChange={e => setReceiveIdType(e.target.value as any)}
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="open_id">open_id</option>
                <option value="user_id">user_id</option>
                <option value="union_id">union_id</option>
                <option value="email">email</option>
                <option value="chat_id">chat_id（群组）</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">消息类型</label>
              <select
                value={msgType}
                onChange={e => setMsgType(e.target.value as any)}
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="text">文本消息</option>
                <option value="post">富文本消息</option>
                <option value="interactive">卡片消息</option>
              </select>
            </div>
          </div>

          <div className="mb-4">
            <label className="block text-sm text-gray-600 mb-1">接收者 ID（open_id / user_id / chat_id）</label>
            <input
              type="text"
              value={receiveId}
              onChange={e => setReceiveId(e.target.value)}
              placeholder="例如：ou_c2c620cfd86e5c67267919847143b696"
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="mb-4">
            <label className="block text-sm text-gray-600 mb-1">消息内容</label>
            <textarea
              value={messageContent}
              onChange={e => setMessageContent(e.target.value)}
              placeholder="输入要发送的消息内容"
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <button
            onClick={sendMessage}
            disabled={loading || !userToken || !receiveId || !messageContent}
            className="px-6 py-3 bg-orange-500 text-white rounded-lg hover:bg-orange-600 disabled:opacity-50 font-medium"
          >
            {loading ? '发送中...' : '📤 发送消息'}
          </button>
          <p className="text-xs text-gray-500 mt-2">提示：接收者需要在机器人的可用范围内，向群组发送时机器人需在群内</p>
        </div>

        {/* 需求确认卡片测试 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">📋 需求确认卡片测试</h2>
          <p className="text-sm text-gray-600 mb-3">测试发送需求确认卡片给用户，卡片包含 Approve/Reject 按钮</p>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div>
              <label className="block text-sm text-gray-600 mb-1">接收者 Open ID</label>
              <input
                type="text"
                value={cardOpenId}
                onChange={e => setCardOpenId(e.target.value)}
                placeholder="例如：ou_c2c620cfd86e5c67267919847143b696"
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-gray-500 mt-1">从上方 Token 状态中的 Open ID 复制</p>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">卡片标题</label>
              <input
                type="text"
                value={cardTitle}
                onChange={e => setCardTitle(e.target.value)}
                placeholder="需求标题"
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <div className="mb-4">
            <label className="block text-sm text-gray-600 mb-1">需求摘要</label>
            <input
              type="text"
              value={cardSummary}
              onChange={e => setCardSummary(e.target.value)}
              placeholder="简短的需求摘要"
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="mb-4">
            <label className="block text-sm text-gray-600 mb-1">详细需求</label>
            <textarea
              value={cardRequirement}
              onChange={e => setCardRequirement(e.target.value)}
              placeholder="详细的需求描述..."
              rows={4}
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="mb-4">
            <label className="block text-sm text-gray-600 mb-1">Session ID（用于跳转链接）</label>
            <input
              type="text"
              value={cardSessionId}
              onChange={e => setCardSessionId(e.target.value)}
              placeholder="会话ID"
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <button
            onClick={sendApprovalCard}
            disabled={loading || !cardOpenId || !cardTitle}
            className="px-6 py-3 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600 disabled:opacity-50 font-medium"
          >
            {loading ? '发送中...' : '📋 发送需求确认卡片'}
          </button>
          <p className="text-xs text-gray-500 mt-2">注意：需要先登录 Go 后端并获取 Admin 权限</p>
        </div>

        {/* Token 状态 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold text-gray-700">📋 Token 状态</h2>
            <button
              onClick={() => {
                setUserToken('')
                setTokenExpires('')
                localStorage.removeItem(USER_TOKEN_KEY)
                localStorage.removeItem(USER_TOKEN_EXPIRES_KEY)
              }}
              className="px-3 py-1 text-sm bg-red-100 text-red-600 rounded hover:bg-red-200"
            >
              清空 Token
            </button>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm text-gray-600 mb-1">User Token</label>
              <div className="text-xs text-gray-800 break-all bg-gray-50 p-2 rounded max-h-20 overflow-y-auto">
                {userToken || '未获取'}
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">过期时间</label>
              <div className="text-xs text-gray-800 bg-gray-50 p-2 rounded">
                {tokenExpires ? new Date(tokenExpires).toLocaleString() : '未获取或无过期时间'}
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">Folder Token</label>
              <input
                type="text"
                value={folderToken}
                onChange={e => setFolderToken(e.target.value)}
                placeholder="云盘文件夹 token"
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 text-xs"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">Document ID</label>
              <input
                type="text"
                value={documentId}
                onChange={e => setDocumentId(e.target.value)}
                placeholder="文档 ID (docx...)"
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 text-xs"
              />
            </div>
          </div>
        </div>

        {/* 测试按钮 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">🧪 接口测试</h2>
          
          {/* 文档标题输入 */}
          <div className="mb-3">
            <label className="block text-sm text-gray-600 mb-1">创建文档标题</label>
            <input
              type="text"
              value={docTitle}
              onChange={e => setDocTitle(e.target.value)}
              placeholder="输入文档标题"
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
            />
          </div>
          
          <div className="flex flex-wrap gap-2">
            {tests.map((test, i) => (
              <button
                key={i}
                onClick={() => {
                  const body = test.api === '/api/health2'
                    ? {}
                    : { ...test.body, user_token: userToken }
                  callApi(test.api, body)
                }}
                disabled={loading || (test.api !== '/api/health2' && !userToken)}
                className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 transition"
                title={!userToken && test.api !== '/api/health2' ? '请先获取 Token' : ''}
              >
                {test.name}
              </button>
            ))}
            <button
              onClick={() => setResults([])}
              className="px-4 py-2 bg-gray-500 text-white rounded hover:bg-gray-600 transition"
            >
              清空结果
            </button>
          </div>
        </div>

        {/* GET 接口测试 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">📖 文档内容获取</h2>
          <button
            onClick={() => {
              if (!documentId) {
                alert('请输入 Document ID')
                return
              }
              callGetApi('/api/feishu/document-content', {
                document_id: documentId,
                user_token: userToken,
              })
            }}
            disabled={loading || !userToken || !documentId}
            className="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:opacity-50 transition"
            title={!documentId ? '请输入 Document ID' : !userToken ? '请先获取 Token' : ''}
          >
            获取文档纯文本内容
          </button>
        </div>

        {/* 文件夹元数据 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">📁 文件夹元数据</h2>
          <p className="text-sm text-gray-600 mb-3">根据文件夹 token 获取文件夹的元数据（ID、名称、创建者等）</p>
          <div className="flex items-center gap-4 mb-3">
            <input
              type="text"
              value={folderToken}
              onChange={e => setFolderToken(e.target.value)}
              placeholder="输入文件夹 token"
              className="flex-1 px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
            />
            <button
              onClick={() => {
                if (!folderToken) {
                  alert('请输入文件夹 token')
                  return
                }
                callGetApi('/api/feishu/folder-meta', {
                  folder_token: folderToken,
                  user_token: userToken,
                })
              }}
              disabled={loading || !folderToken}
              className="px-6 py-2 bg-teal-500 text-white rounded hover:bg-teal-600 disabled:opacity-50 transition"
              title={!folderToken ? '请输入文件夹 token' : ''}
            >
              {loading ? '获取中...' : '获取文件夹元数据'}
            </button>
          </div>
          <p className="text-xs text-gray-500">提示：folder_token 从云盘文件夹 URL 中获取，格式为 https://feishu.cn/drive/folder/{'{folder_token}'}</p>
        </div>

        {/* AI 文档内容获取 */}
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">🤖 AI 文档生成代码</h2>
          <p className="text-sm text-gray-600 mb-3">输入飞书文档 URL，AI 自动读取文档并根据文档需求生成代码</p>
          <div className="mb-3">
            <textarea
              value={aiDocUrl}
              onChange={e => setAiDocUrl(e.target.value)}
              placeholder="输入飞书文档 URL，例如：https://feishu.cn/docx/doxbcmEtbxxx"
              rows={2}
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
            />
          </div>
          <button
            onClick={handleAIGenerate}
            disabled={loading || !aiDocUrl}
            className="px-6 py-2 bg-indigo-500 text-white rounded hover:bg-indigo-600 disabled:opacity-50 transition"
          >
            {loading ? '生成中...' : '🚀 开始生成'}
          </button>
        </div>

        {/* 结果展示 */}
        <div className="bg-white rounded-lg shadow">
          <div className="p-4 border-b border-gray-200">
            <h2 className="font-semibold text-gray-700">📊 测试结果</h2>
          </div>
          <div className="divide-y divide-gray-100 max-h-[600px] overflow-y-auto">
            {results.length === 0 ? (
              <p className="p-4 text-gray-500 text-center">暂无测试结果</p>
            ) : (
              results.map((r, i) => (
                <div key={i} className="p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-mono text-sm text-blue-600">{r.api}</span>
                    <span className="text-xs text-gray-400">{r.time}</span>
                  </div>
                  <pre className="bg-gray-50 p-3 rounded text-xs overflow-x-auto max-h-64">
                    {JSON.stringify(r.result, null, 2)}
                  </pre>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
