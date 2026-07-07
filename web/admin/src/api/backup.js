import request from './request'

export function createBackup() {
  return request.post('/admin/backup')
}

export function listBackups() {
  return request.get('/admin/backups')
}
