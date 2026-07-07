import request from './request'

export function listReviewQueue(params) {
  // params: { status: 'review'|'published'|'archived', page, limit }
  return request.get('/admin/entries', { params })
}

export function approveEntry(id) {
  return request.post(`/admin/entries/${id}/approve`)
}

export function rejectEntry(id, reason) {
  return request.post(`/admin/entries/${id}/reject`, { reason })
}

export function takedownEntry(id, reason) {
  return request.post(`/admin/entries/${id}/takedown`, { reason })
}
