import axios from 'axios'
import {OpenAI} from 'openai'
import { sendMessage } from '../../lib/feishu.js'

// ============ HTTP API Test Tool ============

interface TestCase {
  name: string
  url: string
  method: string
  headers?: Record<string, string>
  params?: Record<string, unknown>
  json?: Record<string, unknown>
  assertions: Array<{
    type: string
    key?: string
    expect: unknown
  }>
}

interface TestResult {
  name: string
  url: string
  method: string
  status: number
  response: unknown
  passed: boolean
  assertions: Array<{
    type: string
    key?: string
    expect: unknown
    actual: unknown
    passed: boolean
  }>
  error?: string
  duration: number
}

/**
 * 自动批量测试 HTTP 接口
 * @param base_url 接口基础域名
 * @param test_cases 批量接口测试用例列表
 */
export async function testApi(base_url: string, test_cases: TestCase[]): Promise<{
  success: boolean
  summary: {
    total: number
    passed: number
    failed: number
    duration: number
  }
  results: TestResult[]
}> {
  const startTime = Date.now()
  const results: TestResult[] = []

  for (const tc of test_cases) {
    const caseStart = Date.now()
    const fullUrl = `${base_url.replace(/\/$/, '')}${tc.url}`
    let response: any = null
    let status = 0
    let error: string | undefined

    try {
      const config: any = {
        method: tc.method.toLowerCase(),
        url: fullUrl,
        headers: tc.headers || {},
        timeout: 30000,
      }

      if (tc.params) {
        config.params = tc.params
      }
      if (tc.json) {
        config.data = tc.json
      }

      const res = await axios(config)
      response = res.data
      status = res.status
    } catch (err: any) {
      if (err.response) {
        status = err.response.status
        response = err.response.data
      }
      error = err.message
    }

    // 执行断言
    const assertionResults = tc.assertions.map(assertion => {
      let actual: unknown
      let passed = false

      if (assertion.type === 'status_code') {
        actual = status
        passed = status === assertion.expect
      } else if (assertion.type === 'json_key') {
        actual = getJsonValue(response, assertion.key || '')
        passed = JSON.stringify(actual) === JSON.stringify(assertion.expect)
      } else {
        actual = null
        passed = false
      }

      return {
        type: assertion.type,
        key: assertion.key,
        expect: assertion.expect,
        actual,
        passed,
      }
    })

    const allPassed = !error && assertionResults.every(a => a.passed)

    results.push({
      name: tc.name,
      url: fullUrl,
      method: tc.method,
      status,
      response,
      passed: allPassed,
      assertions: assertionResults,
      error,
      duration: Date.now() - caseStart,
    })
  }

  const totalDuration = Date.now() - startTime
  const passed = results.filter(r => r.passed).length

  return {
    success: passed === results.length,
    summary: {
      total: results.length,
      passed,
      failed: results.length - passed,
      duration: totalDuration,
    },
    results,
  }
}

/**
 * 从 JSON 对象中获取嵌套字段值
 * @param obj JSON 对象
 * @param path 字段路径，如 "data.user.name"
 */
function getJsonValue(obj: any, path: string): unknown {
  if (!path) return obj
  const keys = path.split('.')
  let current = obj
  for (const key of keys) {
    if (current === null || current === undefined) return undefined
    current = current[key]
  }
  return current
}

// 飞书应用配置
const FEISHU_APP_ID = process.env.FEISHU_APP_ID ?? ''
const FEISHU_APP_SECRET = process.env.FEISHU_APP_SECRET ?? ''

/**
 * 获取飞书文档纯文本内容（用于 AI Function Calling）
 * 
 * @param documentId - 飞书文档 ID
 * @param userToken - 用户访问令牌（可选，不传则使用 tenant token）
 * 
 * @returns 包含文档纯文本内容的响应
 */
export async function getDocumentForAI(
  documentId: string,
  userToken?: string
): Promise<{
  code: number
  msg: string
  data?: {
    document_id: string
    content: string
  }
}> {
  // 参数校验
  if (!documentId) {
    return {
      code: 400,
      msg: 'document_id is required',
    }
  }

  try {
    let accessToken: string

    if (userToken) {
      // 使用用户 token
      accessToken = userToken
    } else {
      // 获取 tenant token
      const tokenRes = await axios.post(
        'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
        { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
      )
      accessToken = tokenRes.data.app_access_token
    }

    // 调用飞书 API 获取文档纯文本内容
    const response = await axios.get(
      `https://open.feishu.cn/open-apis/docx/v1/documents/${documentId}/raw_content`,
      {
        headers: {
          Authorization: `Bearer ${accessToken}`,
          'Content-Type': 'application/json; charset=utf-8',
        },
        params: { lang: 0 }, // 0 = 中文
      }
    )

    // 检查飞书 API 返回
    if (response.data?.code !== 0) {
      return {
        code: response.data?.code || 400,
        msg: response.data?.msg || 'Failed to get document content',
      }
    }

    // raw_content 直接返回纯文本
    const content = response.data?.data?.content || ''

    return {
      code: 200,
      msg: 'success',
      data: {
        document_id: documentId,
        content,
      },
    }
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { status?: number; data?: unknown } }
    
    // 处理 401 错误
    if (err.response?.status === 401) {
      return {
        code: 401,
        msg: 'Unauthorized: invalid or expired token',
      }
    }

    // 其他错误
    return {
      code: 500,
      msg: err.message || 'Internal server error',
    }
  }
}

/**
 * 从飞书文档块中提取纯文本
 */
function extractTextFromBlocks(blocks: unknown[]): string {
  const texts: string[] = []

  function extract(block: Record<string, unknown>) {
    // 提取文本内容
    if (block.text) {
      const textBlock = block.text as { elements?: Array<{ text_run?: { content?: string } }> }
      if (textBlock.elements) {
        for (const element of textBlock.elements) {
          if (element.text_run?.content) {
            texts.push(element.text_run.content)
          }
        }
      }
    }

    // 递归处理子块
    if (block.children) {
      const children = block.children as unknown[]
      for (const child of children) {
        if (typeof child === 'object' && child !== null) {
          extract(child as Record<string, unknown>)
        }
      }
    }
  }

  for (const block of blocks) {
    if (typeof block === 'object' && block !== null) {
      extract(block as Record<string, unknown>)
    }
  }

  return texts.join('\n')
}


/**
 * 从文本中提取飞书文档链接并获取内容
 */
async function extractContentFromUrls(text: string, userToken?: string): Promise<{ urlsFound: string[]; contents: Record<string, string> }> {
  const urlsFound: string[] = []
  const contents: Record<string, string> = {}
  
  // 匹配飞书文档链接
  const feishuUrlPatterns = [
    /https?:\/\/(?:feishu\.cn|lark\.cc)\/(?:docx|wiki)\/([a-zA-Z0-9_-]+)/gi,
    /https?:\/\/["']?([^"'\s]*feishu[^"'\s]*\/docx\/[^"'\s]+)["']?/gi,
  ]
  
  // 收集所有文档链接
  const allMatches = new Set<string>()
  for (const pattern of feishuUrlPatterns) {
    let match
    while ((match = pattern.exec(text)) !== null) {
      const url = match[0] || match[1]
      // 提取 document_id
      const docIdMatch = url.match(/docx\/([a-zA-Z0-9_-]+)/i)
      if (docIdMatch) {
        allMatches.add(docIdMatch[1])
      }
    }
  }
  
  urlsFound.push(...allMatches)
  
  // 提取每个文档的内容
  for (const docId of allMatches) {
    try {
      const result = await getDocumentForAI(docId, userToken)
      if (result.code === 200 && result.data) {
        // result.data.content 是纯文本
        const textContent = result.data.content || ''
        contents[docId] = textContent || '(文档内容为空)'
      } else {
        contents[docId] = `(提取失败: ${result.msg || '未知错误'})`
      }
    } catch (err: any) {
      contents[docId] = `(提取失败: ${err.message})`
    }
  }
  
  return { urlsFound, contents }
}

// ============ AI Function Calling Tools ============

/**
 * AI 可调用的工具列表（用于 Function Calling）
 */
export const documentTools = [
  {
    type: "function" as const,
    function: {
      name: "extractContentFromUrls",
      description: "自动识别文本中的飞书文档链接并提取内容。当用户的输入中包含飞书文档链接时，必须调用此工具提取文档内容进行分析。支持同时提取多个文档。",
      parameters: {
        type: "object",
        properties: {
          text: {
            type: "string",
            description: "用户输入的完整文本，其中可能包含一个或多个飞书文档链接。链接格式如：https://feishu.cn/docx/xxx 或 https://feishu.cn/wiki/xxx"
          },
          user_token: {
            type: "string",
            description: "飞书用户访问令牌（可选）"
          }
        },
        required: ["text"]
      }
    }
  },
  {
    type: "function" as const,
    function: {
      name: "getDocumentContent",
      description: "根据文档ID获取飞书文档的纯文本内容，用于阅读和分析文档。注意：document_id格式如doxbcmEtbxxx，从飞书文档URL中获取。",
      parameters: {
        type: "object",
        properties: {
          user_token: {
            type: "string",
            description: "飞书用户访问令牌，可从上下文获取。如果为空则自动使用tenant token。"
          },
          document_id: {
            type: "string",
            description: "飞书文档ID，格式如 doxbcmEtbxxx，从文档URL中获取，例如 https://feishu.cn/docx/doxbcmEtbxxx 中的 doxbcmEtbxxx"
          }
        },
        required: ["document_id"]
      }
    }
  },
  {
    type: "function" as const,
    function: {
      name: "saveOpenApiSpec",
      description: "保存生成的Swagger/OpenAPI规范JSON，返回对应的Swagger UI页面访问链接。当你生成了API接口文档后，必须调用此工具来保存并获取页面链接。",
      parameters: {
        type: "object",
        properties: {
          spec: {
            type: "object",
            description: "完整的Swagger/OpenAPI 3.0规范JSON对象"
          }
        },
        required: ["spec"]
      }
    }
  },
  {
    type: "function" as const,
    function: {
      name: "createFeishuDocument",
      description: "创建飞书文档（docx）。当需要为用户生成文档时使用此工具。",
      parameters: {
        type: "object",
        properties: {
          user_token: { 
            type: "string", 
            description: "飞书用户访问令牌" 
          },
          folder_token: { 
            type: "string", 
            description: "文件夹token，不填则在根目录创建" 
          },
          title: { 
            type: "string", 
            description: "文档标题" 
          }
        },
        required: ["user_token", "title"]
      }
    }
  },
  {
    type: "function" as const,
    function: {
      name: "createFeishuDocumentBlocks",
      description: "在飞书文档中批量创建块（段落、标题、列表、代码块等）。创建文档后使用此工具写入内容。",
      parameters: {
        type: "object",
        properties: {
          user_token: { 
            type: "string", 
            description: "飞书用户访问令牌" 
          },
          document_id: { 
            type: "string", 
            description: "文档ID，创建文档后返回的 document_id" 
          },
          blocks: {
            type: "array",
            description: "块数组，每项包含 block_type 和内容。block_type: 2=文本段落, 3=标题1, 4=标题2, 5=标题3, 6=标题4, 7=标题5, 8=标题6, 11=无序列表, 12=有序列表, 14=代码块, 17=分割线",
            items: {
              type: "object",
              properties: {
                block_type: { type: "number", description: "块类型" },
                content: { type: "string", description: "文本内容" }
              }
            }
          }
        },
        required: ["user_token", "document_id", "blocks"]
      }
    }
  },{
    type:"function" as const,
    function:{
      name:"createSwapperApiJson",
      description:`生成符合OpenAPI 3.0规范的 JSON 文档，用于Swagger UI展示，
      每个接口要包含：
   - summary
   - description
   - parameters（如有）
   - requestBody（如有）
   - responses（必须包含200响应）
      只返回 JSON，不要 markdown，不要解释
      `,
      "parameters": {
    "type": "object",
    "properties": {
      "apiInfo": {
        "type": "object",
        "description": "API基础信息",
        "properties": {
          "title": { "type": "string" },
          "version": { "type": "string" },
          "description": { "type": "string" }
        },
        "required": ["title", "version"]
      },
      "servers": {
        "type": "array",
        "description": "服务器地址列表",
        "items": {
          "type": "object",
          "properties": {
            "url": { "type": "string" },
            "description": { "type": "string" }
          },
          "required": ["url"]
        }
      },
      "endpoints": {
        "type": "array",
        "description": "接口列表",
        "items": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string",
              "description": "接口路径，如 /user/login"
            },
            "method": {
              "type": "string",
              "enum": ["get", "post", "put", "delete", "patch"]
            },
            "summary": { "type": "string" },
            "description": { "type": "string" },
            "tags": {
              "type": "array",
              "items": { "type": "string" }
            },
            "queryParams": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" },
                  "type": { "type": "string" },
                  "required": { "type": "boolean" },
                  "description": { "type": "string" },
                  "example": {}
                },
                "required": ["name", "type"]
              }
            },
            "pathParams": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" },
                  "type": { "type": "string" },
                  "required": { "type": "boolean" }
                },
                "required": ["name", "type"]
              }
            },
            "headers": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" },
                  "type": { "type": "string" },
                  "required": { "type": "boolean" }
                },
                "required": ["name", "type"]
              }
            },
            "requestBody": {
              "type": "object",
              "description": "请求体定义（JSON）",
              "properties": {
                "contentType": {
                  "type": "string",
                  "default": "application/json"
                },
                "schema": {
                  "type": "object",
                  "description": "字段结构（key-value形式）"
                },
                "example": {}
              }
            },
            "responses": {
              "type": "object",
              "description": "响应定义",
              "additionalProperties": {
                "type": "object",
                "properties": {
                  "description": { "type": "string" },
                  "schema": { "type": "object" },
                  "example": {}
                }
              }
            },
            "auth": {
              "type": "string",
              "enum": ["none", "bearer", "apiKey"]
            }
          },
          "required": ["path", "method", "summary"]
        }
      }
    },
    "required": ["apiInfo", "endpoints"]
  }
    },
    
  },
  {
    type: "function" as const,
    function: {
      name: "testApi",
      description: "自动批量测试HTTP接口，支持GET/POST/DELETE/PUT等方法，自动断言状态码和响应字段，返回测试报告。当用户需要测试API接口、验证接口可用性或进行接口回归测试时调用此工具。",
      parameters: {
        type: "object",
        properties: {
          base_url: {
            type: "string",
            description: "接口基础域名，例如 https://api.example.com 或 http://localhost:8080"
          },
          test_cases: {
            type: "array",
            description: "批量接口测试用例列表",
            items: {
              type: "object",
              properties: {
                name: { type: "string", description: "用例名称" },
                url: { type: "string", description: "接口路径（拼接base_url）" },
                method: { type: "string", description: "请求方法 GET/POST/PUT/DELETE" },
                headers: { type: "object", description: "请求头，可选" },
                params: { type: "object", description: "GET参数，可选" },
                json: { type: "object", description: "POST JSON参数，可选" },
                assertions: {
                  type: "array",
                  description: "断言规则",
                  items: {
                    type: "object",
                    properties: {
                      type: { type: "string", description: "断言类型 status_code/json_key" },
                      key: { type: "string", description: "json字段名，status_code无需传" },
                      expect: { type: "any", description: "预期值" }
                    }
                  }
                }
              },
              required: ["name", "url", "method", "assertions"]
            }
          }
        },
        required: ["base_url", "test_cases"]
      }
    }
  }
]

/**
 * 工具函数处理器映射表
 */
export const documentToolHandlers = {
  extractContentFromUrls: async (args: { text: string; user_token?: string }) => {
    console.log('[AI Tool] extractContentFromUrls called')
    try {
      const { urlsFound, contents } = await extractContentFromUrls(args.text, args.user_token)
      
      if (urlsFound.length === 0) {
        return JSON.stringify({
          success: true,
          message: '未在文本中找到飞书文档链接',
          urlsFound: [],
          contents: {}
        })
      }
      
      // 构建可读的返回结果
      const extractedContents = Object.entries(contents).map(([docId, content]) => ({
        document_id: docId,
        url: `https://feishu.cn/docx/${docId}`,
        content: content
      }))
      
      return JSON.stringify({
        success: true,
        message: `成功提取 ${urlsFound.length} 个文档的内容`,
        urlsFound,
        extractedContents
      })
    } catch (err: any) {
      console.error('[AI Tool] extractContentFromUrls error:', err)
      return JSON.stringify({ success: false, error: err.message })
    }
  },
  getDocumentContent: async (args: { user_token?: string; document_id: string }) => {
    const result = await getDocumentForAI(args.document_id, args.user_token)
    if (result.code === 200) {
      return JSON.stringify(result.data)
    }
    return JSON.stringify({ error: result.msg, code: result.code })
  },
  importOpenApiToApifox: async (args: { project_id: string; openapi_spec: Record<string, unknown>; options?: any }) => {
    console.log('[AI Tool] importOpenApiToApifox called with:', { 
      project_id: args.project_id, 
      spec_keys: Object.keys(args.openapi_spec || {}),
      spec_sample: JSON.stringify(args.openapi_spec).substring(0, 500)
    })
    try {
      // 动态导入避免编译时依赖缺失
      const { importOpenApiFromSpec } = await import('../../lib/apifox.js')
      const result = await importOpenApiFromSpec(args.project_id, args.openapi_spec, args.options || {})
      console.log('[AI Tool] importOpenApiFromSpec result:', result)
      if (result.success) {
        const counters = result.data?.counters
        return JSON.stringify({
          success: true,
          message: `成功导入 Apifox 项目 ${args.project_id}！`,
          details: {
            endpointCreated: counters?.endpointCreated || 0,
            endpointUpdated: counters?.endpointUpdated || 0,
            schemaCreated: counters?.schemaCreated || 0,
            schemaUpdated: counters?.schemaUpdated || 0,
          }
        })
      } else {
        return JSON.stringify({ success: false, error: result.error })
      }
    } catch (err: any) {
      console.error('[AI Tool] importOpenApiToApifox error:', err)
      return JSON.stringify({ success: false, error: err.message })
    }
  },
  saveOpenApiSpec: async (args: { spec: Record<string, unknown> }) => {
    try {
      console.log('[saveOpenApiSpec] Calling with spec keys:', Object.keys(args.spec || {}))
      const response = await axios.post('http://localhost:3001/api/openapi', {
        spec: args.spec
      })
      console.log('[saveOpenApiSpec] Response:', response.data)
      if (response.data?.success) {
        return JSON.stringify({
          success: true,
          specId: response.data.data.specId,
          swaggerUrl: response.data.data.swaggerUrl
        })
      }
      return JSON.stringify({ success: false, error: response.data?.error || '保存OpenAPI规范失败' })
    } catch (err: any) {
      console.error('[saveOpenApiSpec] Error:', err.response?.data || err.message)
      return JSON.stringify({ success: false, error: err.response?.data?.error || err.message })
    }
  },
  createFeishuDocument: async (args: { user_token: string; folder_token?: string; title: string }) => {
    try {
      const response = await axios.post(
        'https://open.feishu.cn/open-apis/docx/v1/documents',
        { 
          folder_token: args.folder_token || undefined, 
          title: args.title 
        },
        { headers: { Authorization: `Bearer ${args.user_token}` } }
      )
      if (response.data?.code === 0) {
        return JSON.stringify({
          success: true,
          document_id: response.data.data?.document?.document_id,
          title: args.title,
          url: `https://feishu.cn/docx/${response.data.data?.document?.document_id}`
        })
      }
      return JSON.stringify({ success: false, error: response.data?.msg || '创建文档失败', code: response.data?.code })
    } catch (err: any) {
      console.error('[createFeishuDocument] Error:', err.response?.data || err.message)
      return JSON.stringify({ success: false, error: err.response?.data?.msg || err.message })
    }
  },
  testApi: async (args: { base_url: string; test_cases: TestCase[] }) => {
    console.log('[AI Tool] testApi called with:', args.base_url, args.test_cases.length, 'cases')
    try {
      const result = await testApi(args.base_url, args.test_cases)
      return JSON.stringify(result)
    } catch (err: any) {
      console.error('[AI Tool] testApi error:', err)
      return JSON.stringify({ success: false, error: err.message })
    }
  },
  createFeishuDocumentBlocks: async (args: { user_token: string; document_id: string; blocks: Array<{block_type: number; content: string}> }) => {
    try {
      // 转换 blocks 为飞书 API 格式
      const children = args.blocks.map(block => {
        // 空内容处理
        const content = block.content || ' '
        
        if (block.block_type === 17) {
          // 分割线 - 跳过，避免错误
          return null
        }
        if (block.block_type === 14) {
          // 代码块
          return {
            block_type: 14,
            code: {
              elements: [{ 
                text_run: { 
                  content: content,
                  text_element_style: {}
                }
              }]
            }
          }
        }
        if (block.block_type >= 3 && block.block_type <= 8) {
          // 标题
          const headingMap: Record<number, string> = {
            3: 'heading1', 4: 'heading2', 5: 'heading3',
            6: 'heading4', 7: 'heading5', 8: 'heading6'
          }
          return {
            block_type: block.block_type,
            [headingMap[block.block_type]]: {
              elements: [{ 
                text_run: { 
                  content,
                  text_element_style: {}
                }
              }]
            }
          }
        }
        if (block.block_type === 11 || block.block_type === 12) {
          // 列表 - 改为普通文本块，飞书 API 不支持 bullet/ordered
          return {
            block_type: 2,
            text: {
              elements: [{ 
                text_run: { 
                  content: (block.block_type === 11 ? '• ' : '') + content,
                  text_element_style: {}
                }
              }]
            }
          }
        }
        // 默认文本段落
        return {
          block_type: 2,
          text: {
            elements: [{ 
              text_run: { 
                content,
                text_element_style: {}
              }
            }]
          }
        }
      }).filter(Boolean) // 过滤掉 null（分割线）

      // 分批写入，每次最多 50 个 blocks
      const batchSize = 50
      let successCount = 0
      
      // 等待文档创建完成
      await new Promise(resolve => setTimeout(resolve, 1000))
      
      for (let i = 0; i < children.length; i += batchSize) {
        const batch = children.slice(i, i + batchSize)
        
        console.log(`[createFeishuDocumentBlocks] Writing batch ${Math.floor(i/batchSize) + 1}, ${batch.length} blocks`)
        
        // 重试机制
        let retries = 3
        let lastError = null
        
        while (retries > 0) {
          try {
            const response = await axios.post(
              `https://open.feishu.cn/open-apis/docx/v1/documents/${args.document_id}/blocks/${args.document_id}/children`,
              { children: batch },
              { headers: { Authorization: `Bearer ${args.user_token}` } }
            )
            
            if (response.data?.code === 0) {
              successCount += batch.length
              break
            }
            
            lastError = response.data
            console.error('[createFeishuDocumentBlocks] Batch failed:', response.data)
            retries--
            
            if (retries > 0) {
              console.log(`[createFeishuDocumentBlocks] Retrying... (${retries} attempts left)`)
              await new Promise(resolve => setTimeout(resolve, 2000))
            }
          } catch (err: any) {
            lastError = err.response?.data || err.message
            retries--
            
            if (retries > 0) {
              console.log(`[createFeishuDocumentBlocks] Retrying... (${retries} attempts left)`)
              await new Promise(resolve => setTimeout(resolve, 2000))
            }
          }
        }
        
        if (retries === 0) {
          // 如果失败，尝试逐个写入
          console.log('[createFeishuDocumentBlocks] Batch failed, trying one by one...')
          for (const block of batch) {
            if (!block) continue
            try {
              const res = await axios.post(
                `https://open.feishu.cn/open-apis/docx/v1/documents/${args.document_id}/blocks/${args.document_id}/children`,
                { children: [block] },
                { headers: { Authorization: `Bearer ${args.user_token}` } }
              )
              if (res.data?.code === 0) {
                successCount++
              } else {
                console.warn('[createFeishuDocumentBlocks] Single block failed:', block.block_type, res.data)
              }
            } catch (err: any) {
              console.warn('[createFeishuDocumentBlocks] Single block error:', block.block_type, err.response?.data?.msg)
            }
            await new Promise(resolve => setTimeout(resolve, 500))
          }
        }
      }
      
      return JSON.stringify({ success: true, message: `内容写入成功，共 ${successCount} 个块` })
    } catch (err: any) {
      console.error('[createFeishuDocumentBlocks] Error:', err.response?.data || err.message)
      const errorData = err.response?.data
      const fieldViolations = errorData?.error?.field_violations || []
      return JSON.stringify({ 
        success: false, 
        error: errorData?.msg || err.message,
        field_violations: fieldViolations
      })
    }
  }
}

/**
 * SSE 流式 AI 对话（支持 CoT 和 Function Calling 可观测性）
 * @param userMessage 用户消息
 * @param userToken 用户飞书token
 * @param openId 用户openId
 * @param onEvent 事件回调函数，用于将 SSE 事件发送到客户端
 */
export async function runAIChatStream(
  userMessage: string,
  userToken?: string,
  openId?: string,
  onEvent?: (event: string, data: any) => void
): Promise<{
  sendToClient: (event: string, data: any) => void
  finish: () => void
  completed: Promise<void> // 新增：返回完成 Promise
}> {
  // 创建动态 OpenAI 客户端
  const apiKey = process.env.OPENAI_API_KEY
  const client = new OpenAI({ apiKey, baseURL: "https://api.deepseek.com" })
  
  // 事件回调
  let finished = false
  let completeResolve: (() => void) | null = null
  const completed = new Promise<void>((resolve) => {
    completeResolve = resolve
  })

  const sendToClient = (event: string, data: any) => {
    if (!finished && onEvent) {
      onEvent(event, data)
    }
  }

  const finish = () => {
    finished = true
    if (completeResolve) {
      completeResolve()
    }
  }

  console.log('[runAIChatStream] Function started, onEvent:', !!onEvent)
  // 在后台运行 AI 对话
  setImmediate(async () => {
    console.log('[runAIChatStream] setImmediate callback running')
    try {
      // 构建系统提示词
      const systemPrompt = `
你是专业后端API工程师，可以为用户生成API文档并写入飞书文档。

## 核心能力
1. 分析用户需求文档（**自动识别文本中的飞书文档链接并提取内容**）
2. 生成API接口设计文档
3. **创建飞书文档并写入内容（必须调用工具，禁止直接输出文本）**
4. **生成 Swagger UI 可视化接口文档（必须调用 saveOpenApiSpec）**
5. **生成测试用例并执行接口测试（必须调用 testApi）**

## 强制规则
**重要：你必须调用工具完成工作，禁止直接输出文本内容！**
- 不要直接输出 API 设计文档的文字
- 不要直接输出代码
- **必须**先调用 createFeishuDocument 创建文档
- **必须**再调用 createFeishuDocumentBlocks 写入内容
- **必须**调用 saveOpenApiSpec 保存 OpenAPI JSON 并获取 Swagger UI 链接
- **必须**调用 testApi 执行接口测试
- **严格禁止：禁止调用任何发送消息的工具！**

## 可用工具

### 0. extractContentFromUrls - 自动提取文档内容（**优先使用**）
当用户的输入中包含飞书文档链接时，**必须首先调用此工具**提取文档内容。
- text: 用户输入的完整文本（包含可能的文档链接）
- user_token: 飞书用户token（使用: ${userToken || '未提供'}）

### 1. createFeishuDocument - 创建飞书文档
当需要为用户生成文档时调用：user_token, title, folder_token(可选)

### 2. createFeishuDocumentBlocks - 写入文档内容
创建文档后调用此工具写入内容：user_token, document_id, blocks(内容块数组)

### 3. saveOpenApiSpec - 保存 OpenAPI JSON 并生成 Swagger UI
**必须调用此工具**来生成可交互的接口调试页面。

### 4. testApi - 执行接口测试
**生成API后必须调用此工具测试接口**，base_url 为 http://localhost:3001，test_cases 包含用例名称、接口路径、请求方法、参数和断言规则。

## 工作流程（接口对接模式）
1. 提取文档内容（如有链接）
2. 分析需求
3. 设计API
4. 创建文档并写入内容
5. 生成Swagger UI
6. 生成测试用例
7. 调用 testApi 执行测试
8. 分析测试结果并给出结论

## 重要提醒
- **createFeishuDocumentBlocks 只能调用一次！**调用后直接返回文档链接，不要再次调用
- **saveOpenApiSpec 必须调用！**用于生成 Swagger UI 页面链接
- 文档创建后，**直接返回文档链接和 Swagger UI 链接**
- **不要调用任何发送消息的工具**

## 输出格式
创建文档后，返回飞书文档链接和 Swagger UI 链接。
`

      const openai = new OpenAI({
        apiKey: process.env.OPENAI_API_KEY || '',
        baseURL: 'https://api.deepseek.com'
      })

      const messages: any[] = [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: userMessage }
      ]

      sendToClient('thinking', { content: '开始分析需求...' })

      const response = await client.chat.completions.create({
        model: 'deepseek-chat',
        messages,
        tools: documentTools as any,
        tool_choice: 'required'
      })

      const assistantMessage = response.choices[0]?.message

      if (assistantMessage?.tool_calls && assistantMessage.tool_calls.length > 0) {
        // 添加 assistant 消息
        messages.push(assistantMessage)

        for (const toolCall of assistantMessage.tool_calls) {
          const func = (toolCall as any).function
          const name = func.name

          // 发送工具调用开始事件
          sendToClient('tool_call', {
            name,
            arguments: func.arguments,
            status: 'calling'
          })

          try {
            // 调用工具
            const handler = documentToolHandlers[name as keyof typeof documentToolHandlers]
            if (!handler) {
              throw new Error(`Unknown tool: ${name}`)
            }
            console.log(`[AI Tool] Calling: ${name}`)
            const result = await handler(JSON.parse(func.arguments))
            console.log(`[AI Tool] ${name} result:`, JSON.parse(result))

            // 发送工具调用结果
            sendToClient('tool_result', {
              name,
              result: JSON.parse(result),
              status: 'success'
            })

            // 添加 tool 消息
            messages.push({
              role: 'tool',
              tool_call_id: toolCall.id,
              content: result
            })
          } catch (err: any) {
            // 发送工具调用错误
            sendToClient('tool_result', {
              name,
              error: err.message,
              status: 'error'
            })

            messages.push({
              role: 'tool',
              tool_call_id: toolCall.id,
              content: JSON.stringify({ error: err.message })
            })
          }
        }

        // 递归继续对话 - 需要持续调用直到没有工具调用
        let continueLoop = true
        while (continueLoop) {
          sendToClient('thinking', { content: '继续生成回复...' })

          const finalResponse = await client.chat.completions.create({
            model: 'deepseek-chat',
            messages,
            tools: documentTools as any,
            tool_choice: 'required'
          })

          const finalMessage = finalResponse.choices[0]?.message
          
          if (finalMessage?.tool_calls && finalMessage.tool_calls.length > 0) {
            // 有新的工具调用，继续处理
            messages.push(finalMessage)
            
            for (const toolCall of finalMessage.tool_calls) {
              const func = (toolCall as any).function
              const name = func.name
              
              sendToClient('tool_call', {
                name,
                arguments: func.arguments,
                status: 'calling'
              })
              
              try {
                const handler = documentToolHandlers[name as keyof typeof documentToolHandlers]
                if (!handler) {
                  throw new Error(`Unknown tool: ${name}`)
                }
                console.log(`[AI Tool] Calling: ${name}`)
                const result = await handler(JSON.parse(func.arguments))
                console.log(`[AI Tool] ${name} result:`, JSON.parse(result))

                sendToClient('tool_result', {
                  name,
                  result: JSON.parse(result),
                  status: 'success'
                })

                messages.push({
                  role: 'tool',
                  tool_call_id: toolCall.id,
                  content: result
                })
              } catch (err: any) {
                sendToClient('tool_result', {
                  name,
                  error: err.message,
                  status: 'error'
                })

                messages.push({
                  role: 'tool',
                  tool_call_id: toolCall.id,
                  content: JSON.stringify({ error: err.message })
                })
              }
            }
            // 继续循环处理更多工具调用
          } else {
            // 没有更多工具调用，结束
            continueLoop = false
            const finalContent = finalMessage?.content || ''
            sendToClient('text', { content: finalContent })
            sendToClient('done', { content: finalContent })
          }
        }
      } else {
        // 无工具调用
        sendToClient('text', { content: assistantMessage?.content || '' })
        sendToClient('done', { content: assistantMessage?.content || '' })
      }
    } catch (err: any) {
      console.error('[AI Stream] Error:', err)
      sendToClient('error', { message: err.message || 'AI 调用失败' })
      sendToClient('done', { content: '' })
    }
  })

  return { sendToClient, finish, completed }
}

const openai = new OpenAI({
    apiKey: process.env.OPENAI_API_KEY || '',
    baseURL:"https://api.deepseek.com"
})

export async function runAIChat(userMessage: string, messages: any[] = [], isFirstCall: boolean = true, userToken?: string, openId?: string, recursionDepth: number = 0) {
    const MAX_RECURSION = 5  // 最大递归深度
    
    if (recursionDepth >= MAX_RECURSION) {
        console.log(`[AI Tool] Stopping: max recursion depth (${MAX_RECURSION}) reached`)
        return '达到最大递归深度，停止处理'
    }
    
    const apiKey = process.env.OPENAI_API_KEY
    const client = new OpenAI({
        apiKey: apiKey,
        baseURL: "https://api.deepseek.com"
    })
    try {
        // 构建消息历史
        const allMessages: any[] = [
            {role:"system",content:`
你是专业后端API工程师，可以为用户生成API文档并写入飞书文档。

## 核心能力
1. 分析用户需求文档（**自动识别文本中的飞书文档链接并提取内容**）
2. 生成API接口设计文档
3. **创建飞书文档并写入内容（必须调用工具，禁止直接输出文本）**
4. **生成测试用例并执行接口测试（必须调用 testApi）**

## 强制规则
**重要：你必须调用工具完成工作，禁止直接输出文本内容！**
- 不要直接输出 API 设计文档的文字
- 不要直接输出代码
- **必须**先调用 createFeishuDocument 创建文档
- **必须**再调用 createFeishuDocumentBlocks 写入内容
- **必须**调用 testApi 执行接口测试
- **严格禁止：禁止调用任何发送消息的工具！**

## 可用工具

### 0. extractContentFromUrls - 自动提取文档内容（**优先使用**）
当用户的输入中包含飞书文档链接时，**必须首先调用此工具**提取文档内容。

### 1. createFeishuDocument - 创建飞书文档
当需要为用户生成文档时调用：user_token, title, folder_token(可选)

### 2. createFeishuDocumentBlocks - 写入文档内容
创建文档后调用此工具写入内容：user_token, document_id, blocks(内容块数组)

### 3. testApi - 执行接口测试
**生成API后必须调用此工具测试接口**，base_url 为 http://localhost:3001，test_cases 包含用例名称、接口路径、请求方法、参数和断言规则。

## 工作流程
1. 提取文档内容（如有链接）
2. 分析需求
3. 设计API
4. 创建文档并写入内容
5. 生成测试用例
6. 调用 testApi 执行测试
7. 分析测试结果并给出结论

## 重要提醒
- **createFeishuDocumentBlocks 只能调用一次！**调用后直接返回文档链接，不要再次调用
- 文档创建后，**直接返回文档链接**
- **不要调用任何发送消息的工具**

## 输出格式
创建文档后，返回文档链接和测试结果摘要即可。
`}
        ]
        
        // 如果是首次调用，添加之前的消息和当前用户消息
        if (isFirstCall) {
            allMessages.push(...messages, {role:"user",content:userMessage})
        } else {
            // 递归调用时，只添加之前的消息（已包含 user 消息）
            allMessages.push(...messages)
        }
        
        const response=await client.chat.completions.create({
            model:"deepseek-chat",
            messages: allMessages,
            tools: documentTools as any,
            tool_choice:"required"
        })

    const assistantMessage = response.choices[0]?.message

    if (assistantMessage?.tool_calls && assistantMessage.tool_calls.length > 0) {
        // 添加 assistant 的 tool_calls 消息
        messages.push(assistantMessage)
        let forceStop = false
        let consecutiveErrors = 0  // 连续错误计数
        
        for(const toolCall of assistantMessage.tool_calls){
            // 连续错误达到3次，停止调用
            if (consecutiveErrors >= 3) {
                console.log(`[AI Tool] Stopping: ${consecutiveErrors} consecutive errors reached`)
                const stopMsg = `工具连续失败${consecutiveErrors}次，停止调用`
                messages.push({role:"tool",tool_call_id:toolCall.id,content:JSON.stringify({error: stopMsg})})
                break
            }
            
            try {
                // 兼容处理 OpenAI SDK 的不同版本
                const func = (toolCall as any).function
                const name = func.name as keyof typeof documentToolHandlers
                
                // 如果是 createFeishuDocumentBlocks，调用后立即停止递归
                if (name === 'createFeishuDocumentBlocks') {
                    console.log('[AI] createFeishuDocumentBlocks called, will stop after this')
                    forceStop = true
                }
                
                // 解析 JSON，如果失败则尝试修复
                let parsedArgs
                try {
                    parsedArgs = JSON.parse(func.arguments)
                } catch (parseErr) {
                    console.warn(`[AI Tool] JSON parse failed, trying to fix...`)
                    console.warn(`[AI Tool] Original args (first 500 chars):`, func.arguments?.substring(0, 500))
                    
                    // 尝试修复常见的 JSON 错误
                    let fixedArgs = func.arguments || '{}'
                    
                    try {
                        // 1. 替换未转义的换行符（在字符串值中）
                        fixedArgs = fixedArgs.replace(/([^\\])\n/g, '$1\\n')
                        
                        // 2. 替换未转义的引号（在 content 字段中）
                        fixedArgs = fixedArgs.replace(/"content":\s*"([^"]*(?:"[^"]*)*)"/g, (match: string, content: string) => {
                            const escaped = content.replace(/"/g, '\\"')
                            return `"content": "${escaped}"`
                        })
                        
                        // 3. 移除末尾的逗号
                        fixedArgs = fixedArgs.replace(/,\s*}/g, '}')
                        fixedArgs = fixedArgs.replace(/,\s*]/g, ']')
                        
                        // 4. 替换所有未转义的控制字符
                        fixedArgs = fixedArgs.replace(/[\x00-\x1f]/g, (char: string) => {
                            return '\\u' + char.charCodeAt(0).toString(16).padStart(4, '0')
                        })
                        
                        parsedArgs = JSON.parse(fixedArgs)
                        console.log('[AI Tool] JSON fixed successfully')
                    } catch (fixErr) {
                        console.error('[AI Tool] JSON fix failed:', fixErr)
                        // 返回错误，不使用空对象
                        const errorMsg = `JSON解析失败: ${(parseErr as Error).message}`
                        messages.push({role:"tool",tool_call_id:toolCall.id,content:JSON.stringify({error: errorMsg})})
                        consecutiveErrors++
                        continue // 跳过这个工具调用
                    }
                }
                
                console.log(`[AI Tool] Calling ${name} with args:`, JSON.stringify(parsedArgs)?.substring(0, 200))
                const result = await documentToolHandlers[name](parsedArgs as any)
                console.log(`[AI Tool] ${name} result:`, result)
                // 添加 tool 角色的消息
                messages.push({role:"tool",tool_call_id:toolCall.id,content:result})
                
                // 检查结果是否包含错误
                try {
                    const resultObj = JSON.parse(result)
                    if (!resultObj.success) {
                        consecutiveErrors++
                    } else {
                        consecutiveErrors = 0
                    }
                } catch {
                    consecutiveErrors = 0
                }
            } catch (toolErr: any) {
                // 工具调用失败
                const errorMsg = `工具调用失败: ${toolErr.message}`
                console.error(`[AI Tool] Error:`, errorMsg)
                messages.push({role:"tool",tool_call_id:toolCall.id,content:JSON.stringify({error: errorMsg})})
                consecutiveErrors++
            }
        }

        // 连续错误达到3次，停止递归
        if (consecutiveErrors >= 3) {
            console.log('[AI Tool] Stopping recursion: too many consecutive errors')
            return '工具调用连续失败，停止处理'
        }

        // 如果 createFeishuDocumentBlocks 已调用，跳过递归，直接返回文档链接
        if (forceStop) {
            // 提取文档链接并返回
            let docUrl = ''
            for (const msg of messages) {
              if (msg.role === 'tool' && msg.content) {
                try {
                  const c = typeof msg.content === 'string' ? JSON.parse(msg.content) : msg.content
                  if (c?.url && c.url.includes('feishu.cn/docx/')) {
                    docUrl = c.url
                    break
                  }
                } catch {}
              }
            }
            return docUrl ? `文档已创建: ${docUrl}` : '文档已创建但未获取到链接'
        }

        // 递归调用时标记为非首次，增加递归深度
        return runAIChat(userMessage, messages, false, userToken, openId, recursionDepth + 1)
    }
    
    // 从消息历史中提取文档链接
    let documentUrl = ''
    for (const msg of messages) {
      if (msg.role === 'tool' && msg.content) {
        try {
          const content = typeof msg.content === 'string' ? JSON.parse(msg.content) : msg.content
          if (content?.url && content.url.includes('feishu.cn/docx/')) {
            documentUrl = content.url
            break
          }
        } catch {}
      }
    }
    
    // 如果找到文档链接，返回链接而非空内容
    if (documentUrl) {
      return `文档已创建: ${documentUrl}`
    }
    
    return assistantMessage?.content || ''
    } catch (err: any) {
        return { success: false, error: err.message || 'AI 调用失败' }
    }
}
