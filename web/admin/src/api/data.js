import axios from 'axios'
import request from './request'

const token = () => sessionStorage.getItem('admin_token')

// 导出：二进制 ZIP，绕过 request.js 的 JSON envelope 拦截器（raw axios + blob）
export function exportData(include) {
  return axios.get('/api/v1/admin/export', {
    params: { include: include.join(',') },
    responseType: 'blob',
    headers: { Authorization: `Bearer ${token()}` }
  })
}

// 导入：multipart 上传，返回 JSON envelope（走 request 拦截器，自动解包 data.data）
export function importData(file, conflict) {
  const form = new FormData()
  form.append('file', file)
  form.append('conflict', conflict)
  return request.post('/admin/import', form)
}
