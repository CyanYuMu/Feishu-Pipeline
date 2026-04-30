/**
 * 飞书 SDK 客户端初始化
 * 文档: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/nodejs-sdk/using-the-sdk
 *
 * 注意: .env 由 tsx -r dotenv/config 在启动时加载，此处不再重复加载
 */

import lark from '@larksuiteoapi/node-sdk'
import axios from 'axios'

// 飞书应用配置（由 tsx -r dotenv/config 加载）
const FEISHU_APP_ID = process.env.FEISHU_APP_ID ?? ''
const FEISHU_APP_SECRET = process.env.FEISHU_APP_SECRET ?? ''

// 验证配置
if (!FEISHU_APP_ID || !FEISHU_APP_SECRET) {
  console.warn('[Feishu] 警告: 未配置 FEISHU_APP_ID 或 FEISHU_APP_SECRET')
}

// 创建飞书客户端实例
// disableTokenCache=false: SDK 自动管理租户 Token 的获取与刷新（推荐）
export const feishuClient = new lark.Client({
  appId: FEISHU_APP_ID,
  appSecret: FEISHU_APP_SECRET,
  disableTokenCache: false,
})

// 导出 lark 命名空间，方便使用 withTenantToken / withUserAccessToken 等工具函数
export { lark }

/**
 * 调用需要用户权限的 API（如获取用户信息）
 *
 * @param userToken - 从 Go 后端获取的 user_access_token
 * @param apiCall - 实际调用的飞书 API（传入 SDK 方法）
 *
 * @example
 * const userInfo = await withUserToken(userToken, (token) =>
 *   client.contact.v3.user.get({
 *     path: { user_id: 'user_id' },
 *   }, lark.withUserAccessToken(token))
 * )
 */
export async function withUserToken<T>(
  userToken: string,
  apiCall: (token: string) => Promise<T>
): Promise<T> {
  return apiCall(userToken)
}

/**
 * 获取用户信息（需要 user_access_token）
 * @see https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/user/get
 */
export async function getUserInfo(userToken: string, userId: string): Promise<unknown> {
  return feishuClient.contact.v3.user.get(
    {
      path: { user_id: userId },
      params: { user_id_type: 'open_id' },
    },
    lark.withUserAccessToken(userToken)
  )
}

/**
 * 示例: 调用文档 API（仅需 tenant token，SDK 自动处理）
 * @see https://open.feishu.cn/document/server-docs/docs/docs/docx-v1/document/raw-content
 */
export async function getDocumentRawContent(documentId: string): Promise<unknown> {
  return feishuClient.docx.v1.document.rawContent({
    path: { document_id: documentId },
    params: { lang: 0 },
  })
}

/**
 * 获取部门子部门列表
 * @see https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/department/children
 *
 * @param departmentId - 部门 ID（根部门为 "0"）
 * @param userToken - 用户访问令牌（可选，不传则使用 tenant token）
 * @param options - 可选参数
 *   - fetch_child: 是否递归获取子部门
 *   - page_size: 每页数量，最大 50
 *   - page_token: 分页标记
 *   - user_id_type: 用户 ID 类型
 *   - department_id_type: 部门 ID 类型
 */
export async function getDepartmentChildren(
  departmentId: string,
  userToken?: string,
  options: {
    fetch_child?: boolean
    page_size?: number
    page_token?: string
    user_id_type?: 'open_id' | 'union_id' | 'user_id'
    department_id_type?: 'open_department_id' | 'department_id'
  } = {}
): Promise<unknown> {
  const {
    fetch_child,
    page_size,
    page_token,
    user_id_type = 'open_id',
    department_id_type = 'open_department_id',
  } = options

  const requestOptions = userToken
    ? lark.withUserAccessToken(userToken)
    : undefined

  return feishuClient.contact.v3.department.children(
    {
      path: { department_id: departmentId },
      params: {
        fetch_child: fetch_child,
        page_size: page_size,
        page_token: page_token,
        user_id_type,
        department_id_type,
      },
    },
    requestOptions as any
  )
}

/**
 * 批量获取部门信息
 * @see https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/department/batch
 *
 * @param departmentIds - 部门 ID 数组（最多 50 个）
 * @param userToken - 用户访问令牌（可选，不传则使用 tenant token）
 * @param options - 可选参数
 *   - department_id_type: 部门 ID 类型
 *   - user_id_type: 用户 ID 类型
 */
export async function batchGetDepartments(
  departmentIds: string[],
  userToken?: string,
  options: {
    department_id_type?: 'open_department_id' | 'department_id'
    user_id_type?: 'open_id' | 'union_id' | 'user_id'
  } = {}
): Promise<unknown> {
  const {
    department_id_type = 'open_department_id',
    user_id_type = 'open_id',
  } = options

  const requestOptions = userToken
    ? lark.withUserAccessToken(userToken)
    : undefined

  return feishuClient.contact.v3.department.batch(
    {
      params: {
        department_ids: departmentIds,
        department_id_type,
        user_id_type,
      },
    },
    requestOptions as any
  )
}

/**
 * 批量获取用户姓名
 * @see https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/user/basic_batch
 *
 * @param userIds - 用户 ID 数组（最多 10 个）
 * @param userToken - 用户访问令牌（可选，不传则使用 tenant token）
 * @param options - 可选参数
 *   - user_id_type: 用户 ID 类型
 */
export async function batchGetUserNames(
  userIds: string[],
  userToken?: string,
  options: {
    user_id_type?: 'open_id' | 'union_id' | 'user_id'
  } = {}
): Promise<unknown> {
  const {
    user_id_type = 'open_id',
  } = options

  // SDK 没有 basicBatch 方法，直接使用 HTTP 调用
  
  // 获取 token
  let accessToken: string
  if (userToken) {
    accessToken = userToken
  } else {
    // 使用 tenant token
    const tokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
      { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
    )
    accessToken = tokenRes.data.app_access_token
  }

  const response = await axios.post(
    'https://open.feishu.cn/open-apis/contact/v3/users/basic_batch',
    { user_ids: userIds },
    {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json; charset=utf-8',
      },
      params: { user_id_type },
    }
  )

  return response.data
}

/**
 * 发送消息
 * @see https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
 *
 * @param receiveId - 接收者 ID
 * @param receiveIdType - 接收者 ID 类型：open_id / user_id / union_id / email / chat_id
 * @param msgType - 消息类型：text / post / image / interactive 等
 * @param content - 消息内容（JSON 序列化字符串）
 * @param userToken - 用户访问令牌（可选，不传则使用 tenant token）
 * @param uuid - 可选，用于请求去重
 */
export async function sendMessage(
  receiveId: string,
  receiveIdType: 'open_id' | 'user_id' | 'union_id' | 'email' | 'chat_id',
  msgType: string,
  content: string,
  userToken?: string,
  uuid?: string
): Promise<unknown> {
  // 获取 token
  let accessToken: string
  if (userToken) {
    accessToken = userToken
  } else {
    // 使用 tenant token
    const tokenRes = await axios.post(
      'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
      { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
    )
    accessToken = tokenRes.data.app_access_token
  }

  const response = await axios.post(
    'https://open.feishu.cn/open-apis/im/v1/messages',
    {
      receive_id: receiveId,
      msg_type: msgType,
      content,
      uuid,
    },
    {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json; charset=utf-8',
      },
      params: { receive_id_type: receiveIdType },
    }
  )

  return response.data
}

/**
 * 创建多维表格
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app/create
 *
 * @param name - 多维表格名称
 * @param folderToken - 父文件夹 Token（可选）
 */
export async function createBitableApp(
  name: string,
  folderToken?: string
): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  const response = await axios.post(
    'https://open.feishu.cn/open-apis/bitable/v1/apps',
    {
      name,
      folder_token: folderToken,
      time_zone: 'Asia/Shanghai',
    },
    {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json; charset=utf-8',
      },
    }
  )

  return response.data
}

/**
 * 获取数据表列表
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table/list
 *
 * @param appToken - 多维表格 App Token
 */
export async function listBitableTables(appToken: string): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  const response = await axios.get(
    `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables`,
    {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    }
  )

  return response.data
}

/**
 * 创建数据表
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table/create
 *
 * @param appToken - 多维表格 App Token
 * @param name - 数据表名称
 */
export async function createBitableTable(
  appToken: string,
  name: string
): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  const response = await axios.post(
    `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables`,
    { name },
    {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json; charset=utf-8',
      },
    }
  )

  return response.data
}

/**
 * 获取记录列表
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/list
 *
 * @param appToken - 多维表格 App Token
 * @param tableId - 数据表 ID
 */
export async function listBitableRecords(
  appToken: string,
  tableId: string,
  pageSize: number = 100,
  pageToken?: string
): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  const params: any = { page_size: pageSize }
  if (pageToken) params.page_token = pageToken

  const response = await axios.get(
    `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables/${tableId}/records`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
      params,
    }
  )

  return response.data
}

/**
 * 创建或更新记录
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/create
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/update
 *
 * @param appToken - 多维表格 App Token
 * @param tableId - 数据表 ID
 * @param fields - 记录字段
 * @param recordId - 记录 ID（可选，不传则创建新记录，传则更新）
 */
export async function upsertBitableRecord(
  appToken: string,
  tableId: string,
  fields: Record<string, any>,
  recordId?: string
): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  const url = recordId
    ? `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables/${tableId}/records/${recordId}`
    : `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables/${tableId}/records`

  const method = recordId ? 'PUT' : 'POST'
  const response = await axios({
    method,
    url,
    data: { fields },
    headers: {
      Authorization: `Bearer ${accessToken}`,
      'Content-Type': 'application/json; charset=utf-8',
    },
  })

  return response.data
}

/**
 * 批量创建或更新记录
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/batch_create
 * @see https://open.feishu.cn/document/server-docs/docs/bitable-v1/app-table-record/batch_update
 *
 * @param appToken - 多维表格 App Token
 * @param tableId - 数据表 ID
 * @param records - 记录数组（每条包含 fields 和可选的 record_id）
 */
export async function batchUpsertBitableRecords(
  appToken: string,
  tableId: string,
  records: Array<{ fields: Record<string, any>; record_id?: string }>
): Promise<unknown> {
  const tokenRes = await axios.post(
    'https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal',
    { app_id: FEISHU_APP_ID, app_secret: FEISHU_APP_SECRET }
  )
  const accessToken = tokenRes.data.app_access_token

  // 分批处理，每批最多 10 条
  const results: any[] = []
  const batchSize = 10

  for (let i = 0; i < records.length; i += batchSize) {
    const batch = records.slice(i, i + batchSize)
    const hasExistingRecords = batch.some(r => r.record_id)

    const url = `https://open.feishu.cn/open-apis/bitable/v1/apps/${appToken}/tables/${tableId}/records`

    const response = await axios({
      method: 'POST',
      url,
      data: { records: batch },
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json; charset=utf-8',
      },
    })

    results.push(response.data)
  }

  return results
}
