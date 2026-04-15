import request from './request'

export function getUserStats() {
  return request.get('/admin/stats/users')
}

export function getContributionStats(params) {
  return request.get('/admin/stats/contributions', { params })
}

export function getActivityTrend(days = 30) {
  return request.get('/admin/stats/activity', { params: { days } })
}

export function getRegistrationTrend(days = 30) {
  return request.get('/admin/stats/registrations', { params: { days } })
}
