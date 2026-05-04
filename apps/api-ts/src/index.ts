import Fastify from 'fastify'
import cors from '@fastify/cors'
import axios from 'axios'
import type { FastifyRequest, FastifyReply } from 'fastify'
import { feishuClient, lark, getDepartmentChildren, batchGetDepartments, batchGetUserNames, sendMessage, createBitableApp, listBitableTables, createBitableTable, listBitableRecords, upsertBitableRecord, batchUpsertBitableRecords } from './lib/feishu.js'
import { getDocumentForAI, runAIChat, runAIChatStream } from './api/test/index.js'

// 注意: .env 由 tsx -r dotenv/config 在启动时加载

const FEISHU_APP_ID = process.env.FEISHU_APP_ID
const FEISHU_APP_SECRET = process.env.FEISHU_APP_SECRET

if (!FEISHU_APP_ID || !FEISHU_APP_SECRET) {
  console.error('缺少必需的环境变量: FEISHU_APP_ID, FEISHU_APP_SECRET')
  process.exit(1)
}

const app = Fastify({
  logger: {
    transport: {
      target: 'pino-pretty',
      options: { colorize: true, translateTime: 'SYS:HH:MM:ss', ignore: 'pid,hostname' },
    },
  },
})

await app.register(cors, {
  origin: process.env.CORS_ORIGIN ?? '*',
  credentials: true,
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'],
  allowedHeaders: ['Content-Type', 'Authorization', 'Accept', 'Origin', 'X-Requested-With'],
  exposedHeaders: ['Content-Length', 'Content-Type'],
  maxAge: 86400,
})

app.get('/api/health2', async () => ({ status: 'ok', service: 'api-ts' }))

import { feishuRoutes } from './routes/feishu.js'
app.register(feishuRoutes)

app.post("/api/feishu/list-files", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { folder_token, page_size, page_token, order_by, direction, user_token } = request.body as any
    if (!user_token) {
      return reply.status(400).send({ success: false, error: 'user_token is required' })
    }
    const result = await feishuClient.drive.v1.file.list(
      {
        params: {
          folder_token: folder_token || undefined,
          page_size: page_size || 50,
          page_token: page_token || undefined,
          order_by: (order_by as any) || undefined,
          direction: (direction as any) || undefined,
        },
      },
      lark.withUserAccessToken(user_token)
    )
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.get("/api/feishu/oauth-url", async (_request: FastifyRequest, reply: FastifyReply) => {
  try {
    const res = await axios.get('http://localhost:8080/api/auth/feishu/config')
    const { appId } = res.data.data || {}

    if (!appId) {
      return reply.status(500).send({ success: false, error: 'Failed to get appId from Go backend' })
    }

    // 需要申请的权限 scope
    const scope = [
      'docx:document:create',
      'docx:document',
      'docx:document:readonly',
      'drive:drive',
      'drive:drive:readonly',
      'space:document:retrieve',
      'contact:contact.base:readonly',
      'contact:department.base:readonly',
      'contact:department.organize:readonly',
      'contact:user.basic_profile:readonly',
      'im:message',
      'im:message:send_as_bot',
    ].join(' ')
    const redirectUri = encodeURIComponent('http://localhost:3001/api/feishu/callback')
    const oauthUrl = `https://open.feishu.cn/open-apis/authen/v1/authorize?app_id=${appId}&redirect_uri=${redirectUri}&state=ts-auth&scope=${encodeURIComponent(scope)}`

    return reply.send({ success: true, data: { oauthUrl, appId } })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.get("/api/feishu/callback", async (request: FastifyRequest, reply: FastifyReply) => {
  const { code } = request.query as { code?: string }

  if (!code) {
    return reply.redirect('http://localhost:5173/debug?error=no_code')
  }

  try {
    const appTokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
      { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
    )
    const appAccessToken = appTokenRes.data?.app_access_token

    if (!appAccessToken) {
      const debugInfo = encodeURIComponent(JSON.stringify(appTokenRes.data))
      return reply.redirect(`http://localhost:5173/debug?error=no_app_token&detail=${debugInfo}`)
    }

    const tokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/authen/v1/oidc/access_token',
      { grant_type: 'authorization_code', code },
      { headers: { Authorization: `Bearer ${appAccessToken}` } }
    )

    const { access_token, refresh_token } = tokenRes.data.data || {}

    if (!access_token) {
      const debugInfo = encodeURIComponent(JSON.stringify(tokenRes.data))
      return reply.redirect(`http://localhost:5173/debug?error=no_token&detail=${debugInfo}`)
    }

    // 获取用户信息
    let openId = ''
    try {
      const userRes = await axios.get(
        'https://open.feishu.cn/open-apis/authen/v1/user_info',
        { headers: { Authorization: `Bearer ${access_token}` } }
      )
      openId = userRes.data?.data?.open_id || ''
    } catch (userErr) {
      console.error('获取用户信息失败:', userErr)
    }

    return reply.redirect(`http://localhost:5173/debug?token=${encodeURIComponent(access_token)}&refresh_token=${encodeURIComponent(refresh_token || '')}&open_id=${encodeURIComponent(openId)}`)
  } catch (err) {
    const error = err as { response?: { data?: unknown } }
    const debugInfo = encodeURIComponent(JSON.stringify(error.response?.data || String(err)))
    return reply.redirect(`http://localhost:5173/debug?error=oauth_failed&detail=${debugInfo}`)
  }
})

app.post("/api/feishu/exchange-token", async (request: FastifyRequest, reply: FastifyReply) => {
  const { code } = request.body as { code?: string }

  if (!code) {
    return reply.status(400).send({ success: false, error: 'code is required' })
  }

  try {
    const appTokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
      { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
    )
    const appAccessToken = appTokenRes.data?.app_access_token

    if (!appAccessToken) {
      return reply.status(400).send({ success: false, error: 'Failed to get app_access_token' })
    }

    const tokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/authen/v1/oidc/access_token',
      { grant_type: 'authorization_code', code },
      { headers: { Authorization: `Bearer ${appAccessToken}` } }
    )

    const { access_token, refresh_token, expires_in } = tokenRes.data.data || {}

    if (!access_token) {
      return reply.status(400).send({ success: false, error: 'Failed to get token' })
    }

    // 获取用户信息
    let userInfo = null
    try {
      const userRes = await axios.get(
        'https://open.feishu.cn/open-apis/authen/v1/user_info',
        { headers: { Authorization: `Bearer ${access_token}` } }
      )
      userInfo = userRes.data?.data
    } catch (userErr) {
      console.error('获取用户信息失败:', userErr)
    }

    return reply.send({
      success: true,
      data: {
        user_token: access_token,
        refresh_token,
        expires_in,
        expires_at: new Date(Date.now() + (expires_in || 7200) * 1000).toISOString(),
        open_id: userInfo?.open_id,
        user_name: userInfo?.name,
      },
    })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/get-drive-files", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { folder_token, page_size, page_token, order_by, direction, user_id_type, user_token } = request.body as any
    const result = await axios.get("https://open.feishu.cn/open-apis/drive/v1/files", {
      headers: { Authorization: `Bearer ${user_token || ''}` },
      params: { folder_token, page_size, page_token, order_by, direction, user_id_type },
    })
    return reply.send(result.data)
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ error: error.message })
  }
})

app.post("/api/feishu/create-document", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { folder_token, title, user_token } = request.body as any
    if (!user_token) {
      return reply.status(400).send({ success: false, error: 'user_token is required' })
    }
    const result = await feishuClient.docx.v1.document.create(
      {
        data: {
          folder_token: folder_token || undefined,
          title: title || '未命名文档',
        },
      },
      lark.withUserAccessToken(user_token)
    )
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.get("/api/feishu/document-content", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, user_token, lang } = request.query as any
    if (!document_id || !user_token) {
      return reply.status(400).send({ success: false, error: 'document_id and user_token are required' })
    }
    const result = await axios.get(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/raw_content`,
      {
        headers: { Authorization: `Bearer ${user_token}` },
        params: { lang: lang || 0 },
      }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 飞书文档列表（代理到 Go 后端）

app.get("/api/feishu/wiki-spaces", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { user_token, folder_token } = request.query as any
    if (!user_token) {
      return reply.status(400).send({ success: false, error: 'user_token is required' })
    }
    const result = await axios.get(
      'https://open.feishu.cn/open-apis/drive/v1/files',
      {
        headers: { Authorization: `Bearer ${user_token}` },
        params: { 
          page_size: 50,
          folder_token: folder_token || undefined,
        },
      }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err: any) {
    const errorCode = err.response?.status || 500
    const errorMsg = err.response?.data?.msg || err.message
    return reply.status(errorCode).send({ 
      success: false, 
      error: errorMsg,
      code: errorCode
    })
  }
})

// DeepSeek 模型列表
app.get("/api/deepseek/models", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const apiKey = process.env.OPENAI_API_KEY
    if (!apiKey) {
      return reply.status(400).send({ success: false, error: 'OPENAI_API_KEY not configured' })
    }
    
    const response = await axios.get('https://api.deepseek.com/models', {
      headers: {
        'Authorization': `Bearer ${apiKey}`
      }
    })
    
    return reply.send({ success: true, data: response.data })
  } catch (err: any) {
    console.error('[DeepSeek models] Error:', err.response?.data || err.message)
    return reply.status(500).send({ 
      success: false, 
      error: err.response?.data || err.message 
    })
  }
})

// 保存 OpenAPI 规范并生成 Swagger UI
// OpenAPI spec 转发到 Go 后端
const GO_API_BASE = process.env.GO_API_BASE || 'http://localhost:8080'

app.post("/api/openapi", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { spec } = request.body as any
    
    if (!spec) {
      return reply.status(400).send({ success: false, error: 'spec is required' })
    }
    
    // 调用 Go 后端保存 spec
    const response = await axios.post(`${GO_API_BASE}/public/openapi/specs`, {
      title: spec?.info?.title || 'API 文档',
      spec_json: JSON.stringify(spec)
    })
    
    if (response.data?.success) {
      return reply.send({
        success: true,
        data: {
          specId: response.data.data.specId,
          swaggerUrl: response.data.data.swaggerUrl
        }
      })
    }
    
    return reply.status(500).send({ success: false, error: response.data?.error || '保存失败' })
  } catch (err: any) {
    console.error('[saveOpenApiSpec] Error:', err.response?.data || err.message)
    return reply.status(500).send({ 
      success: false, 
      error: err.response?.data?.error || err.message 
    })
  }
})

// 获取 OpenAPI 规范列表（供资产页面使用，转发到 Go 后端）
app.get("/api/openapi/specs", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    // 调用 Go 后端获取所有 specs
    const response = await axios.get(`${GO_API_BASE}/public/openapi/specs`, {
      headers: { 'Accept': 'application/json' }
    })
    
    return reply.send(response.data)
  } catch (err: any) {
    console.error('[listOpenApiSpecs] Error:', err.response?.data || err.message)
    return reply.status(500).send({ success: false, error: err.message })
  }
})

// 更新 OpenAPI 规范的关联信息（转发到 Go 后端）
app.put("/api/openapi/:specId", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { specId } = request.params as any
    
    const response = await axios.put(`${GO_API_BASE}/public/openapi/specs/${specId}`, request.body, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' }
    })
    
    return reply.send(response.data)
  } catch (err: any) {
    console.error('[updateOpenApiSpec] Error:', err.response?.data || err.message)
    return reply.status(500).send({ success: false, error: err.message })
  }
})

// 获取 OpenAPI 规范（供 Swagger UI 使用，转发到 Go 后端）
app.get("/api/openapi/:specId", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { specId } = request.params as any
    
    // 调用 Go 后端获取 spec
    const response = await axios.get(`${GO_API_BASE}/public/openapi/specs/${specId}`, {
      headers: { 'Accept': 'application/json' }
    })
    
    // Go 后端直接返回 spec JSON
    return reply.send(response.data)
  } catch (err: any) {
    console.error('[getOpenApiSpec] Error:', err.response?.data || err.message)
    if (err.response?.status === 404) {
      return reply.status(404).send({ error: 'Spec not found' })
    }
    return reply.status(500).send({ error: err.message })
  }
})

// DeepSeek 模型列表

// AI Function Calling
app.post("/api/ai/get-document-content", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, user_token } = request.body as any
    const result = await getDocumentForAI(document_id, user_token)
    
    if (result.code === 200) {
      return reply.send(result)
    } else if (result.code === 400) {
      return reply.status(400).send(result)
    } else if (result.code === 401) {
      return reply.status(401).send(result)
    } else {
      return reply.status(500).send(result)
    }
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ code: 500, msg: error.message })
  }
})

// AI Chat
app.post("/api/ai/chat", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { message, document_content, user_token, open_id } = request.body as any
    
    if (!message) {
      return reply.status(400).send({ error: 'message is required' })
    }
    
    const fullMessage = document_content 
      ? `请根据以下文档内容生成代码实现：\n\n${document_content}`
      : message
    
    const result = await runAIChat(fullMessage, [], true, user_token, open_id)
    return reply.send({ success: true, data: { content: result } })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/ai/chat/stream", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { message, document_content, user_token, open_id } = request.body as any
    
    if (!message) {
      return reply.status(400).send({ error: 'message is required' })
    }
    
    reply.raw.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
      'Access-Control-Allow-Origin': process.env.CORS_ORIGIN ?? '*',
      'Access-Control-Allow-Credentials': 'true',
    })
    
    // 发送初始事件
    reply.raw.write(`data: {\"event\":\"start\",\"content\":\"开始分析需求...\"}\n\n`)
    console.log('[chat/stream] Starting AI processing...')
    
    const fullMessage = document_content
      ? `请根据以下文档内容生成代码实现：\n\n${document_content}`
      : message
    
    // 事件计数器用于调试
    let eventCount = 0
    
    // 使用 runAIChatStream，传入事件回调
    const { finish, completed } = await runAIChatStream(
      fullMessage,
      user_token,
      open_id,
      (event, data) => {
        eventCount++
        console.log(`[chat/stream] Event #${eventCount}: ${event}`, data)
        // 将事件转发到客户端
        reply.raw.write(`data: ${JSON.stringify({ event, ...data })}\n\n`)
      }
    )
    
    console.log(`[chat/stream] runAIChatStream returned, waiting for completion...`)
    
    // 发送心跳保持连接
    const heartbeat = setInterval(() => {
      reply.raw.write(`: ping\n\n`)
    }, 20000)
    
    // 等待 AI 处理完成（最多 120 秒超时）
    try {
      await Promise.race([
        completed,
        new Promise((_, reject) => setTimeout(() => reject(new Error('AI 处理超时')), 120000))
      ])
    } catch (err: any) {
      console.error('[chat/stream] Processing error:', err.message)
    }
    
    console.log(`[chat/stream] Processing complete. Total events: ${eventCount}`)
    clearInterval(heartbeat)
    finish()
    reply.raw.write(`data: {\"event\":\"done\",\"content\":\"处理完成\"}\n\n`)
    reply.raw.end()
    
  } catch (err) {
    const error = err as Error
    console.error('[chat/stream] Error:', error)
    reply.raw.write(`data: {\"event\":\"error\",\"message\":\"${error.message}\"}\n\n`)
    reply.raw.end()
  }
})

app.get("/api/feishu/department-children", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { department_id, user_token, fetch_child, page_size, page_token, user_id_type, department_id_type } = request.query as any
    if (!department_id) {
      return reply.status(400).send({ success: false, error: 'department_id is required' })
    }
    const result = await getDepartmentChildren(department_id, user_token, {
      fetch_child,
      page_size,
      page_token,
      user_id_type,
      department_id_type,
    })
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.get("/api/feishu/batch-departments", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { department_ids, user_token, department_id_type, user_id_type } = request.query as any
    
    if (!department_ids) {
      return reply.status(400).send({ success: false, error: 'department_ids is required (comma-separated or array)' })
    }
    
    // 支持逗号分隔的字符串或数组
    const ids = Array.isArray(department_ids) ? department_ids : department_ids.split(',')
    
    if (ids.length > 50) {
      return reply.status(400).send({ success: false, error: 'Maximum 50 department IDs allowed' })
    }
    
    const result = await batchGetDepartments(ids, user_token, {
      department_id_type,
      user_id_type,
    })
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/batch-departments", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { department_ids, user_token, department_id_type, user_id_type } = request.body as any
    
    if (!department_ids || !Array.isArray(department_ids)) {
      return reply.status(400).send({ success: false, error: 'department_ids is required (array)' })
    }
    
    if (department_ids.length > 50) {
      return reply.status(400).send({ success: false, error: 'Maximum 50 department IDs allowed' })
    }
    
    const result = await batchGetDepartments(department_ids, user_token, {
      department_id_type,
      user_id_type,
    })
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/batch-user-names", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { user_ids, user_token, user_id_type } = request.body as any
    
    if (!user_ids || !Array.isArray(user_ids)) {
      return reply.status(400).send({ success: false, error: 'user_ids is required (array)' })
    }
    
    if (user_ids.length > 10) {
      return reply.status(400).send({ success: false, error: 'Maximum 10 user IDs allowed' })
    }
    
    const result = await batchGetUserNames(user_ids, user_token, {
      user_id_type,
    })
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/send-message", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { receive_id, receive_id_type, msg_type, content, uuid } = request.body as any
    
    if (!receive_id) {
      return reply.status(400).send({ success: false, error: 'receive_id is required' })
    }
    
    if (!msg_type) {
      return reply.status(400).send({ success: false, error: 'msg_type is required' })
    }
    
    if (!content) {
      return reply.status(400).send({ success: false, error: 'content is required' })
    }
    
    // 使用 tenant_access_token（机器人身份）发送消息，不使用 user_token
    const result = await sendMessage(receive_id, receive_id_type || 'open_id', msg_type, content, undefined, uuid)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/create-block", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token, children } = request.body as any
    if (!document_id || !user_token) {
      return reply.status(400).send({ success: false, error: 'document_id and user_token are required' })
    }
    const targetBlockId = block_id || document_id
    const defaultBlock = {
      block_type: 2,
      paragraph: { elements: [{ text_run: { content: '新段落', text_element_style: {} } }], style: {} },
    }
    const result = await axios.post(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${targetBlockId}/children`,
      { children: children || [defaultBlock] },
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.post("/api/feishu/create-nested-blocks", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token, blocks } = request.body as any
    if (!document_id || !user_token || !blocks) {
      return reply.status(400).send({ success: false, error: 'document_id, user_token, and blocks are required' })
    }
    const targetBlockId = block_id || document_id
    const result = await axios.post(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${targetBlockId}/children`,
      { children: blocks },
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as { message?: string; response?: { data?: unknown } }
    return reply.status(500).send({ success: false, error: error.message, detail: error.response?.data })
  }
})

app.get("/api/feishu/get-block", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token } = request.query as any
    if (!document_id || !block_id || !user_token) {
      return reply.status(400).send({ success: false, error: 'document_id, block_id, and user_token are required' })
    }
    const result = await axios.get(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${block_id}`,
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.get("/api/feishu/get-children", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token, page_size, page_token } = request.query as any
    if (!document_id || !block_id || !user_token) {
      return reply.status(400).send({ success: false, error: 'document_id, block_id, and user_token are required' })
    }
    const targetBlockId = block_id || document_id
    const result = await axios.get(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${targetBlockId}/children`,
      {
        headers: { Authorization: `Bearer ${user_token}` },
        params: { page_size: page_size || 500, page_token: page_token || undefined },
      }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.put("/api/feishu/update-block", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token, update_blocks } = request.body as any
    if (!document_id || !block_id || !user_token || !update_blocks) {
      return reply.status(400).send({ success: false, error: 'document_id, block_id, user_token, and update_blocks are required' })
    }
    const result = await axios.patch(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${block_id}`,
      { update_blocks },
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.put("/api/feishu/batch-update-blocks", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, user_token, requests } = request.body as any
    if (!document_id || !user_token || !requests) {
      return reply.status(400).send({ success: false, error: 'document_id, user_token, and requests are required' })
    }
    const result = await axios.post(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/batch_update`,
      { requests },
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

app.delete("/api/feishu/delete-block", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { document_id, block_id, user_token } = request.query as any
    if (!document_id || !block_id || !user_token) {
      return reply.status(400).send({ success: false, error: 'document_id, block_id, and user_token are required' })
    }
    const result = await axios.delete(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${document_id}/blocks/${block_id}`,
      { headers: { Authorization: `Bearer ${user_token}` } }
    )
    return reply.send({ success: true, data: result.data })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 多维表格

// 创建多维表格 app
app.post("/api/bitable/create-app", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { name, folder_token } = request.body as any
    if (!name) {
      return reply.status(400).send({ success: false, error: 'name is required' })
    }
    const result = await createBitableApp(name, folder_token)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 获取数据表列表
app.get("/api/bitable/list-tables", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { app_token } = request.query as any
    if (!app_token) {
      return reply.status(400).send({ success: false, error: 'app_token is required' })
    }
    const result = await listBitableTables(app_token)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 创建数据表
app.post("/api/bitable/create-table", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { app_token, name } = request.body as any
    if (!app_token || !name) {
      return reply.status(400).send({ success: false, error: 'app_token and name are required' })
    }
    const result = await createBitableTable(app_token, name)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 获取记录列表
app.get("/api/bitable/list-records", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { app_token, table_id, page_size, page_token } = request.query as any
    if (!app_token || !table_id) {
      return reply.status(400).send({ success: false, error: 'app_token and table_id are required' })
    }
    const result = await listBitableRecords(app_token, table_id, page_size ? Number(page_size) : 100, page_token)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 创建或更新记录
app.post("/api/bitable/upsert-record", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { app_token, table_id, fields, record_id } = request.body as any
    if (!app_token || !table_id || !fields) {
      return reply.status(400).send({ success: false, error: 'app_token, table_id, and fields are required' })
    }
    const result = await upsertBitableRecord(app_token, table_id, fields, record_id)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 批量创建记录
app.post("/api/bitable/batch-upsert-records", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { app_token, table_id, records } = request.body as any
    if (!app_token || !table_id || !records) {
      return reply.status(400).send({ success: false, error: 'app_token, table_id, and records are required' })
    }
    const result = await batchUpsertBitableRecords(app_token, table_id, records)
    return reply.send({ success: true, data: result })
  } catch (err) {
    const error = err as Error
    return reply.status(500).send({ success: false, error: error.message })
  }
})

// 获取文件夹元数据
app.get("/api/feishu/folder-meta", async (request: FastifyRequest, reply: FastifyReply) => {
  try {
    const { folder_token, user_token } = request.query as any
    if (!folder_token) {
      return reply.status(400).send({ success: false, error: 'folder_token is required' })
    }

    // 获取 access_token
    let accessToken: string
    if (user_token) {
      accessToken = user_token
    } else {
      const tokenRes = await axios.post(
        'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
        { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
      )
      accessToken = tokenRes.data.app_access_token
    }

    const response = await axios.get(
      `https://open.feishu.cn/open-apis/drive/explorer/v2/folder/${folder_token}/meta`,
      {
        headers: {
          Authorization: `Bearer ${accessToken}`,
          'Content-Type': 'application/json; charset=utf-8',
        },
      }
    )

    return reply.send({ success: true, data: response.data })
  } catch (err) {
    const error = err as { message?: string; response?: { data?: unknown } }
    return reply.status(500).send({ success: false, error: error.message, detail: error.response?.data })
  }
})



app.get('/health', async (_request, reply) => {
  reply.send({ status: 'ok', timestamp: new Date().toISOString() })
})

// GitHub OAuth 配置
const GITHUB_CLIENT_ID = process.env.GITHUB_CLIENT_ID || ''
const GITHUB_CLIENT_SECRET = process.env.GITHUB_CLIENT_SECRET || ''
const GITHUB_REDIRECT_URI = process.env.GITHUB_REDIRECT_URI || 'http://localhost:3001/api/auth/github/callback'

// 获取 GitHub OAuth 配置
app.get("/api/auth/github/config", async (_request: FastifyRequest, reply: FastifyReply) => {
  return reply.send({
    success: true,
    data: {
      enabled: !!(GITHUB_CLIENT_ID && GITHUB_CLIENT_SECRET),
      clientId: GITHUB_CLIENT_ID,
      authorizeUrl: 'https://github.com/login/oauth/authorize',
      callbackUrl: GITHUB_REDIRECT_URI,
    }
  })
})

// GitHub OAuth 回调
app.get("/api/auth/github/callback", async (request: FastifyRequest, reply: FastifyReply) => {
  const { code, state } = request.query as { code?: string; state?: string }
  
  if (!code) {
    return reply.redirect(`${process.env.FRONTEND_URL || 'http://localhost:5173'}/auth/callback?error=no_code`)
  }

  try {
    // 1. 用 code 换取 access_token
    const tokenResponse = await axios.post(
      'https://github.com/login/oauth/access_token',
      {
        client_id: GITHUB_CLIENT_ID,
        client_secret: GITHUB_CLIENT_SECRET,
        code,
        redirect_uri: GITHUB_REDIRECT_URI,
      },
      { headers: { Accept: 'application/json' } }
    )

    const accessToken = tokenResponse.data?.access_token
    if (!accessToken) {
      console.error('[GitHub OAuth] No access_token returned:', tokenResponse.data)
      return reply.redirect(`${process.env.FRONTEND_URL || 'http://localhost:5173'}/auth/callback?error=no_token`)
    }

    // 2. 用 access_token 获取用户信息
    const userResponse = await axios.get('https://api.github.com/user', {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        Accept: 'application/vnd.github.v3+json',
      },
    })

    const user = userResponse.data
    console.log('[GitHub OAuth] User logged in:', user.login, user.email)

    // 3. 调用 Go 后端创建会话
    const userID = 'gh_' + user.id
    console.log('[GitHub OAuth] Calling Go backend at http://localhost:8080/api/auth/github/login')
    
    let goResponse
    try {
      goResponse = await axios.post(
        'http://localhost:8080/api/auth/github/login',
        {
          user_id: userID,
          name: user.name || user.login,
          email: user.email || '',
          avatar: user.avatar_url || '',
        },
        { withCredentials: true }
      )
    } catch (err: any) {
      // axios 会把 4xx/5xx 当作异常，err.response 里有详细信息
      console.error('[GitHub OAuth] Go backend error:', err.response?.status, err.response?.data || err.message)
      return reply.redirect(`${process.env.FRONTEND_URL || 'http://localhost:5173'}/auth/callback?error=go_backend_error&detail=${encodeURIComponent(err.message)}`)
    }

    if (goResponse.data?.success) {
      // 登录成功，跳转到首页
      return reply.redirect(`${process.env.FRONTEND_URL || 'http://localhost:5173'}/?github_login=success`)
    } else {
      console.error('[GitHub OAuth] Go backend login failed:', goResponse.data)
      return reply.redirect(`${process.env.FRONTEND_URL || 'http://localhost:5173'}/auth/callback?error=login_failed`)
    }
  } catch (err: any) {
    console.error('[GitHub OAuth] Error:', err.response?.data || err.message)
    return reply.redirect(
      `${process.env.FRONTEND_URL || 'http://localhost:5173'}/auth/callback?error=oauth_failed&detail=${encodeURIComponent(err.message)}`
    )
  }
})

// GitHub OAuth 获取授权 URL（前端使用）
app.get("/api/auth/github/url", async (_request: FastifyRequest, reply: FastifyReply) => {
  if (!GITHUB_CLIENT_ID) {
    return reply.status(500).send({ success: false, error: 'GitHub OAuth not configured' })
  }

  const scope = encodeURIComponent('read:user user:email')
  const state = Math.random().toString(36).substring(7)
  const authorizeUrl = `https://github.com/login/oauth/authorize?client_id=${GITHUB_CLIENT_ID}&redirect_uri=${encodeURIComponent(GITHUB_REDIRECT_URI)}&scope=${scope}&state=${state}`

  return reply.send({ success: true, data: { url: authorizeUrl } })
})

const PORT = Number(process.env.PORT ?? 3001)

try {
  await app.listen({ port: PORT, host: '0.0.0.0' })
} catch (err) {
  app.log.error(err)
  process.exit(1)
}

const shutdown = async () => {
  await app.close()
  process.exit(0)
}
process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
