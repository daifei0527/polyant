import request from './request'

export function listElections(params) {
  return request.get('/admin/elections', { params })
}
export function getElection(id) {
  return request.get(`/admin/elections/${id}`)
}
export function createElection(data) {
  // data: { title, description, vote_threshold, duration_days, auto_elect }
  return request.post('/admin/elections', data)
}
export function closeElection(id) {
  return request.post(`/admin/elections/${id}/close`)
}
