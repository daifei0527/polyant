import request from './request'

// 密码登录（Web admin 远程入口）。identifier 为邮箱或公钥。
export function login(identifier, password) {
  return request.post('/admin/session/login', { identifier, password })
}

// 当前会话自检（Bearer 由 request 拦截器自动注入）。供刷新后恢复用户信息。
export function getCurrentUser() {
  return request.get('/admin/session')
}
