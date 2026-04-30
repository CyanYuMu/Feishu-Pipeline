/**
 * 飞书多维表格 API
 * 基于 http.ts 封装
 */

import { http } from '../../lib/http'

// ── 类型定义 ─────────────────────────────────────────────────────────────────

export interface BitableApp {
  app_token: string
  name: string
}

export interface BitableTable {
  table_id: string
  name: string
}

export interface BitableRecord {
  record_id: string
  fields: Record<string, any>
}

export interface BitableRecordsResponse {
  items: BitableRecord[]
  has_more: boolean
  page_token?: string
}

// ── API 函数 ─────────────────────────────────────────────────────────────────

/**
 * 创建多维表格
 * @see POST /api/bitable/apps
 */
export async function createBitableApp(name: string, folderToken?: string) {
  const response = await http.post('/api/bitable/apps', {
    name,
    folder_token: folderToken,
  })
  return (response as any).data
}

/**
 * 获取数据表列表
 * @see GET /api/bitable/apps/:appToken/tables
 */
export async function listBitableTables(appToken: string) {
  const response = await http.get(`/api/bitable/apps/${appToken}/tables`)
  return (response as any).data
}

/**
 * 创建数据表
 * @see POST /api/bitable/apps/:appToken/tables
 */
export async function createBitableTable(appToken: string, name: string) {
  const response = await http.post(`/api/bitable/apps/${appToken}/tables`, { name })
  return (response as any).data
}

/**
 * 获取记录列表
 * @see GET /api/bitable/apps/:appToken/tables/:tableId/records
 */
export async function listBitableRecords(
  appToken: string,
  tableId: string,
  options?: { pageSize?: number; pageToken?: string }
) {
  const response = await http.get(
    `/api/bitable/apps/${appToken}/tables/${tableId}/records`,
    {
      params: {
        page_size: options?.pageSize ?? 100,
        page_token: options?.pageToken,
      },
    }
  )
  return (response as any).data
}

/**
 * 创建或更新记录
 * @see POST /api/bitable/apps/:appToken/tables/:tableId/records
 */
export async function upsertBitableRecord(
  appToken: string,
  tableId: string,
  fields: Record<string, any>,
  recordId?: string
) {
  const response = await http.post(
    `/api/bitable/apps/${appToken}/tables/${tableId}/records`,
    { fields, record_id: recordId }
  )
  return (response as any).data
}

/**
 * 批量创建记录
 * @see POST /api/bitable/apps/:appToken/tables/:tableId/records/batch
 */
export async function batchUpsertBitableRecords(
  appToken: string,
  tableId: string,
  records: Array<{ fields: Record<string, any>; record_id?: string }>
) {
  const response = await http.post(
    `/api/bitable/apps/${appToken}/tables/${tableId}/records/batch`,
    { records }
  )
  return (response as any).data
}