import request from './request'

export function createSession(publicKey) {
  return request.post('/admin/session/create', { public_key: publicKey })
}

export function getCurrentUser() {
  return request.get('/user/info')
}
