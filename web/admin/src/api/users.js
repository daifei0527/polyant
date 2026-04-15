import request from './request'

export function listUsers(params) {
  return request.get('/admin/users', { params })
}

export function banUser(publicKey, reason, banType = 'full') {
  return request.post(`/admin/users/${publicKey}/ban`, { reason, ban_type: banType })
}

export function unbanUser(publicKey) {
  return request.post(`/admin/users/${publicKey}/unban`)
}

export function setUserLevel(publicKey, level, reason) {
  return request.put(`/admin/users/${publicKey}/level`, { level, reason })
}
